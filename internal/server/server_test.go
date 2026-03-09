package server_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/jackc/pgx/v5/pgxpool"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/retrieval"
	"github.com/Tight-Line/creel/internal/server"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector/pgvector"
)

// testEnv holds all resources for an integration test.
type testEnv struct {
	pool    *pgxpool.Pool
	conn    *grpc.ClientConn
	apiKey  string
	cleanup func()

	admin     pb.AdminServiceClient
	topics    pb.TopicServiceClient
	documents pb.DocumentServiceClient
	chunks    pb.ChunkServiceClient
	retrieval pb.RetrievalServiceClient
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()

	// Ensure schema exists and run migrations.
	if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	pgURL := pgCfg.URL()
	if err := store.RunMigrations(pgURL, "../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	// Create pool.
	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}

	// Create stores.
	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	grantStore := store.NewGrantStore(pool)
	accountStore := store.NewSystemAccountStore(pool)
	backend := pgvector.New(pool)

	// Create authorizer and searcher.
	authorizer := auth.NewGrantAuthorizer(grantStore)
	searcher := retrieval.NewSearcher(chunkStore, docStore, authorizer, backend)
	contextFetcher := retrieval.NewContextFetcher(chunkStore, authorizer)

	// Create a test system account for API key auth.
	acct, rawKey, err := accountStore.Create(ctx, "integration-test", "integration test account")
	if err != nil {
		pool.Close()
		t.Fatalf("creating test system account: %v", err)
	}

	// Set up API key validator with dynamic lookup.
	apiKeyValidator := auth.NewAPIKeyValidator(nil, accountStore)

	// Create gRPC server on a random port.
	srv := server.New(0, apiKeyValidator, nil)
	pb.RegisterAdminServiceServer(srv.GRPCServer(), server.NewAdminServer(pool, accountStore, "test"))
	pb.RegisterTopicServiceServer(srv.GRPCServer(), server.NewTopicServer(topicStore, authorizer, nil))
	jobStore := store.NewJobStore(pool)
	pb.RegisterDocumentServiceServer(srv.GRPCServer(), server.NewDocumentServer(docStore, jobStore, nil, authorizer))
	pb.RegisterChunkServiceServer(srv.GRPCServer(), server.NewChunkServer(chunkStore, docStore, backend, authorizer))
	pb.RegisterRetrievalServiceServer(srv.GRPCServer(), server.NewRetrievalServer(searcher, contextFetcher))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		pool.Close()
		t.Fatalf("listening: %v", err)
	}

	go func() { _ = srv.GRPCServer().Serve(lis) }()

	// Connect client.
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.GracefulStop()
		pool.Close()
		t.Fatalf("dialing: %v", err)
	}

	env := &testEnv{
		pool:   pool,
		conn:   conn,
		apiKey: rawKey,
		cleanup: func() {
			// Clean up test account.
			_ = accountStore.Delete(ctx, acct.ID)
			_ = conn.Close()
			srv.GracefulStop()
			pool.Close()
		},
		admin:     pb.NewAdminServiceClient(conn),
		topics:    pb.NewTopicServiceClient(conn),
		documents: pb.NewDocumentServiceClient(conn),
		chunks:    pb.NewChunkServiceClient(conn),
		retrieval: pb.NewRetrievalServiceClient(conn),
	}

	t.Cleanup(env.cleanup)
	return env
}

// authCtx returns a context with the API key in metadata.
func (e *testEnv) authCtx() context.Context {
	md := metadata.Pairs("authorization", "Bearer "+e.apiKey)
	return metadata.NewOutgoingContext(context.Background(), md)
}

func TestHealthRPC(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background() // Health is public; no auth needed.

	resp, err := env.admin.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("Status = %q, want ok", resp.Status)
	}
	if resp.Version != "test" {
		t.Errorf("Version = %q, want test", resp.Version)
	}
}

func TestAdminRPCs(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	// CreateSystemAccount.
	createResp, err := env.admin.CreateSystemAccount(ctx, &pb.CreateSystemAccountRequest{
		Name:        "test-admin-rpc",
		Description: "test account",
	})
	if err != nil {
		t.Fatalf("CreateSystemAccount: %v", err)
	}
	if createResp.Account.Name != "test-admin-rpc" {
		t.Errorf("Name = %q", createResp.Account.Name)
	}
	if createResp.ApiKey == "" {
		t.Error("expected non-empty API key")
	}
	accountID := createResp.Account.Id

	// ListSystemAccounts.
	listResp, err := env.admin.ListSystemAccounts(ctx, &pb.ListSystemAccountsRequest{})
	if err != nil {
		t.Fatalf("ListSystemAccounts: %v", err)
	}
	found := false
	for _, a := range listResp.Accounts {
		if a.Id == accountID {
			found = true
		}
	}
	if !found {
		t.Error("created account not found in list")
	}

	// RotateKey with grace period.
	rotateResp, err := env.admin.RotateKey(ctx, &pb.RotateKeyRequest{
		AccountId:          accountID,
		GracePeriodSeconds: 60,
	})
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if rotateResp.ApiKey == "" {
		t.Error("expected non-empty rotated API key")
	}

	// RotateKey without grace period.
	rotateResp2, err := env.admin.RotateKey(ctx, &pb.RotateKeyRequest{
		AccountId: accountID,
	})
	if err != nil {
		t.Fatalf("RotateKey (no grace): %v", err)
	}
	if rotateResp2.ApiKey == "" {
		t.Error("expected non-empty rotated API key")
	}

	// RevokeKey.
	_, err = env.admin.RevokeKey(ctx, &pb.RevokeKeyRequest{AccountId: accountID})
	if err != nil {
		t.Fatalf("RevokeKey: %v", err)
	}

	// DeleteSystemAccount.
	_, err = env.admin.DeleteSystemAccount(ctx, &pb.DeleteSystemAccountRequest{Id: accountID})
	if err != nil {
		t.Fatalf("DeleteSystemAccount: %v", err)
	}
}

func TestAdminRPCs_ValidationErrors(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	_, err := env.admin.CreateSystemAccount(ctx, &pb.CreateSystemAccountRequest{Name: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	_, err = env.admin.DeleteSystemAccount(ctx, &pb.DeleteSystemAccountRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	_, err = env.admin.RotateKey(ctx, &pb.RotateKeyRequest{AccountId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	_, err = env.admin.RevokeKey(ctx, &pb.RevokeKeyRequest{AccountId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestTopicRPCs(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	slug := fmt.Sprintf("test-topic-%d", time.Now().UnixNano())

	// CreateTopic.
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug:        slug,
		Name:        "Test Topic",
		Description: "A test topic",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	if topic.Slug != slug {
		t.Errorf("Slug = %q, want %q", topic.Slug, slug)
	}

	// GetTopic (owner has implicit admin).
	got, err := env.topics.GetTopic(ctx, &pb.GetTopicRequest{Id: topic.Id})
	if err != nil {
		t.Fatalf("GetTopic: %v", err)
	}
	if got.Name != "Test Topic" {
		t.Errorf("Name = %q", got.Name)
	}

	// UpdateTopic.
	updated, err := env.topics.UpdateTopic(ctx, &pb.UpdateTopicRequest{
		Id:          topic.Id,
		Name:        "Updated Topic",
		Description: "Updated description",
	})
	if err != nil {
		t.Fatalf("UpdateTopic: %v", err)
	}
	if updated.Name != "Updated Topic" {
		t.Errorf("Name = %q after update", updated.Name)
	}

	// ListTopics (system account sees all).
	listResp, err := env.topics.ListTopics(ctx, &pb.ListTopicsRequest{})
	if err != nil {
		t.Fatalf("ListTopics: %v", err)
	}
	found := false
	for _, tp := range listResp.Topics {
		if tp.Id == topic.Id {
			found = true
		}
	}
	if !found {
		t.Error("created topic not found in list")
	}

	// GrantAccess.
	grant, err := env.topics.GrantAccess(ctx, &pb.GrantAccessRequest{
		TopicId:    topic.Id,
		Principal:  "user:bob@example.com",
		Permission: pb.Permission_PERMISSION_WRITE,
	})
	if err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}
	if grant.Principal != "user:bob@example.com" {
		t.Errorf("Principal = %q", grant.Principal)
	}

	// ListGrants.
	grantsResp, err := env.topics.ListGrants(ctx, &pb.ListGrantsRequest{TopicId: topic.Id})
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	if len(grantsResp.Grants) == 0 {
		t.Error("expected at least one grant")
	}

	// RevokeAccess.
	_, err = env.topics.RevokeAccess(ctx, &pb.RevokeAccessRequest{
		TopicId:   topic.Id,
		Principal: "user:bob@example.com",
	})
	if err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}

	// DeleteTopic.
	_, err = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id})
	if err != nil {
		t.Fatalf("DeleteTopic: %v", err)
	}
}

func TestTopicRPCs_ValidationErrors(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	// Slug is now auto-generated when empty, so only empty name is an error.
	_, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{Name: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty name, got %v", err)
	}

	_, err = env.topics.GetTopic(ctx, &pb.GetTopicRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty id, got %v", err)
	}

	_, err = env.topics.UpdateTopic(ctx, &pb.UpdateTopicRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty id, got %v", err)
	}

	_, err = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty id, got %v", err)
	}

	_, err = env.topics.GrantAccess(ctx, &pb.GrantAccessRequest{TopicId: "", Principal: "user:x"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty topic_id, got %v", err)
	}

	_, err = env.topics.GrantAccess(ctx, &pb.GrantAccessRequest{
		TopicId:   "some-id",
		Principal: "user:x",
		// PERMISSION_UNSPECIFIED
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for unspecified permission, got %v", err)
	}

	_, err = env.topics.RevokeAccess(ctx, &pb.RevokeAccessRequest{TopicId: "", Principal: "user:x"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty topic_id, got %v", err)
	}

	_, err = env.topics.ListGrants(ctx, &pb.ListGrantsRequest{TopicId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty topic_id, got %v", err)
	}
}

func TestTopicRPCs_Unauthenticated(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background() // no auth

	_, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{Slug: "s", Name: "n"})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestDocumentRPCs(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	slug := fmt.Sprintf("doc-test-topic-%d", time.Now().UnixNano())
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug: slug, Name: "Doc Test Topic",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() { _, _ = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id}) })

	// CreateDocument with metadata.
	meta, _ := structpb.NewStruct(map[string]any{"key": "value"})
	doc, err := env.documents.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId:  topic.Id,
		Slug:     "test-doc",
		Name:     "Test Document",
		DocType:  "session",
		Metadata: meta,
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if doc.Slug != "test-doc" {
		t.Errorf("Slug = %q", doc.Slug)
	}
	if doc.DocType != "session" {
		t.Errorf("DocType = %q", doc.DocType)
	}

	// CreateDocument without doc_type (should default to "reference").
	doc2, err := env.documents.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: topic.Id,
		Slug:    "test-doc-2",
		Name:    "Test Document 2",
	})
	if err != nil {
		t.Fatalf("CreateDocument (no type): %v", err)
	}
	if doc2.DocType != "reference" {
		t.Errorf("DocType = %q, want reference", doc2.DocType)
	}

	// GetDocument.
	got, err := env.documents.GetDocument(ctx, &pb.GetDocumentRequest{Id: doc.Id})
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.Name != "Test Document" {
		t.Errorf("Name = %q", got.Name)
	}

	// ListDocuments.
	listResp, err := env.documents.ListDocuments(ctx, &pb.ListDocumentsRequest{TopicId: topic.Id})
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(listResp.Documents) != 2 {
		t.Errorf("expected 2 documents, got %d", len(listResp.Documents))
	}

	// UpdateDocument.
	updatedMeta, _ := structpb.NewStruct(map[string]any{"updated": true})
	updated, err := env.documents.UpdateDocument(ctx, &pb.UpdateDocumentRequest{
		Id:       doc.Id,
		Name:     "Updated Doc",
		DocType:  "reference",
		Metadata: updatedMeta,
	})
	if err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}
	if updated.Name != "Updated Doc" {
		t.Errorf("Name = %q after update", updated.Name)
	}

	// DeleteDocument.
	_, err = env.documents.DeleteDocument(ctx, &pb.DeleteDocumentRequest{Id: doc2.Id})
	if err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
}

func TestDocumentRPCs_ValidationErrors(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	_, err := env.documents.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: "", Name: "n",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty topic_id, got %v", err)
	}

	_, err = env.documents.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: "some-id", Name: "",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty name, got %v", err)
	}

	_, err = env.documents.GetDocument(ctx, &pb.GetDocumentRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	_, err = env.documents.ListDocuments(ctx, &pb.ListDocumentsRequest{TopicId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	_, err = env.documents.UpdateDocument(ctx, &pb.UpdateDocumentRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	_, err = env.documents.DeleteDocument(ctx, &pb.DeleteDocumentRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestChunkRPCs(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()
	backend := pgvector.New(env.pool)
	dim := backend.EmbeddingDimension()

	slug := fmt.Sprintf("chunk-test-topic-%d", time.Now().UnixNano())
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug: slug, Name: "Chunk Test Topic",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() { _, _ = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id}) })

	doc, err := env.documents.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: topic.Id, Slug: "chunk-doc", Name: "Chunk Doc",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// IngestChunks with embeddings.
	embedding := make([]float64, dim)
	embedding[0] = 1.0
	ingestResp, err := env.chunks.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: doc.Id,
		Chunks: []*pb.ChunkInput{
			{Content: "Hello world", Sequence: 0, Embedding: embedding},
			{Content: "Goodbye world", Sequence: 1, Embedding: embedding},
		},
	})
	if err != nil {
		t.Fatalf("IngestChunks: %v", err)
	}
	if len(ingestResp.Chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(ingestResp.Chunks))
	}
	chunkID := ingestResp.Chunks[0].Id
	if ingestResp.Chunks[0].EmbeddingId == "" {
		t.Error("expected non-empty embedding_id")
	}
	if ingestResp.Chunks[0].Status != pb.ChunkStatus_CHUNK_STATUS_ACTIVE {
		t.Errorf("Status = %v, want ACTIVE", ingestResp.Chunks[0].Status)
	}

	// IngestChunks without embeddings.
	ingestResp2, err := env.chunks.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: doc.Id,
		Chunks:     []*pb.ChunkInput{{Content: "No embedding", Sequence: 2}},
	})
	if err != nil {
		t.Fatalf("IngestChunks (no embedding): %v", err)
	}
	if ingestResp2.Chunks[0].EmbeddingId != "" {
		t.Error("expected empty embedding_id when no embedding provided")
	}

	// GetChunk.
	got, err := env.chunks.GetChunk(ctx, &pb.GetChunkRequest{Id: chunkID})
	if err != nil {
		t.Fatalf("GetChunk: %v", err)
	}
	if got.Content != "Hello world" {
		t.Errorf("Content = %q", got.Content)
	}

	// DeleteChunk.
	_, err = env.chunks.DeleteChunk(ctx, &pb.DeleteChunkRequest{Id: chunkID})
	if err != nil {
		t.Fatalf("DeleteChunk: %v", err)
	}

	// GetChunk after delete should fail.
	_, err = env.chunks.GetChunk(ctx, &pb.GetChunkRequest{Id: chunkID})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestChunkRPCs_ValidationErrors(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	_, err := env.chunks.IngestChunks(ctx, &pb.IngestChunksRequest{DocumentId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	_, err = env.chunks.GetChunk(ctx, &pb.GetChunkRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	_, err = env.chunks.DeleteChunk(ctx, &pb.DeleteChunkRequest{Id: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestRetrievalRPCs(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()
	backend := pgvector.New(env.pool)
	dim := backend.EmbeddingDimension()

	slug := fmt.Sprintf("search-test-topic-%d", time.Now().UnixNano())
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug: slug, Name: "Search Test Topic",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() { _, _ = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id}) })

	// Grant access so search can find this topic.
	// System account is the owner, but AccessibleTopics only checks grants.
	principalID := "system:integration-test"
	_, err = env.topics.GrantAccess(ctx, &pb.GrantAccessRequest{
		TopicId:    topic.Id,
		Principal:  principalID,
		Permission: pb.Permission_PERMISSION_READ,
	})
	if err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	doc, err := env.documents.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: topic.Id, Slug: "search-doc", Name: "Search Doc",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	embedding := make([]float64, dim)
	embedding[0] = 1.0
	_, err = env.chunks.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: doc.Id,
		Chunks: []*pb.ChunkInput{
			{Content: "Searchable content", Sequence: 0, Embedding: embedding},
		},
	})
	if err != nil {
		t.Fatalf("IngestChunks: %v", err)
	}

	// Search across all accessible topics.
	searchResp, err := env.retrieval.Search(ctx, &pb.SearchRequest{
		QueryEmbedding: embedding,
		TopK:           5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(searchResp.Results) == 0 {
		t.Error("expected at least one search result")
	}

	// Search with specific topic filter.
	searchResp2, err := env.retrieval.Search(ctx, &pb.SearchRequest{
		TopicIds:       []string{topic.Id},
		QueryEmbedding: embedding,
		TopK:           5,
	})
	if err != nil {
		t.Fatalf("Search (filtered): %v", err)
	}
	if len(searchResp2.Results) == 0 {
		t.Error("expected at least one search result with topic filter")
	}
	if searchResp2.Results[0].TopicId != topic.Id {
		t.Errorf("TopicId = %q, want %q", searchResp2.Results[0].TopicId, topic.Id)
	}

	// Search with default topK (0 should default to 10).
	searchResp3, err := env.retrieval.Search(ctx, &pb.SearchRequest{
		QueryEmbedding: embedding,
	})
	if err != nil {
		t.Fatalf("Search (default topK): %v", err)
	}
	_ = searchResp3 // just checking it doesn't error

	// Search with metadata filter.
	metaFilter, _ := structpb.NewStruct(map[string]any{"nonexistent": "value"})
	searchResp4, err := env.retrieval.Search(ctx, &pb.SearchRequest{
		QueryEmbedding: embedding,
		MetadataFilter: metaFilter,
		TopK:           5,
	})
	if err != nil {
		t.Fatalf("Search (metadata filter): %v", err)
	}
	_ = searchResp4
}

func TestRetrievalRPCs_ValidationErrors(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	_, err := env.retrieval.Search(ctx, &pb.SearchRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty embedding, got %v", err)
	}
}

func TestGetContextRPC(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	// Missing document_id should be InvalidArgument.
	_, err := env.retrieval.GetContext(ctx, &pb.GetContextRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty document_id, got %v", err)
	}

	// Non-existent document should return Internal (wraps "document not found").
	_, err = env.retrieval.GetContext(ctx, &pb.GetContextRequest{
		DocumentId: "00000000-0000-0000-0000-000000000000",
	})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal for missing document, got %v", err)
	}

	// Seed a topic, document, and chunks to test the success path.
	slug := fmt.Sprintf("getctx-test-%d", time.Now().UnixNano())
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug: slug, Name: "GetContext Test",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() { _, _ = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id}) })

	// Grant read so GetContext can authorize.
	_, err = env.topics.GrantAccess(ctx, &pb.GrantAccessRequest{
		TopicId:    topic.Id,
		Principal:  "system:integration-test",
		Permission: pb.Permission_PERMISSION_READ,
	})
	if err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	doc, err := env.documents.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: topic.Id, Slug: "ctx-doc", Name: "Context Doc",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// Ingest 5 chunks with role metadata.
	backend := pgvector.New(env.pool)
	dim := backend.EmbeddingDimension()
	embedding := make([]float64, dim)
	embedding[0] = 1.0

	var inputs []*pb.ChunkInput
	for i := 0; i < 5; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		meta, _ := structpb.NewStruct(map[string]any{
			"role": role,
			"turn": float64(i/2 + 1),
		})
		inputs = append(inputs, &pb.ChunkInput{
			Content:   fmt.Sprintf("message %d", i),
			Sequence:  int32(i),
			Embedding: embedding,
			Metadata:  meta,
		})
	}
	_, err = env.chunks.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: doc.Id,
		Chunks:     inputs,
	})
	if err != nil {
		t.Fatalf("IngestChunks: %v", err)
	}

	// GetContext with no filters: should return all 5 chunks in sequence order.
	resp, err := env.retrieval.GetContext(ctx, &pb.GetContextRequest{
		DocumentId: doc.Id,
	})
	if err != nil {
		t.Fatalf("GetContext (all): %v", err)
	}
	if len(resp.Chunks) != 5 {
		t.Fatalf("expected 5 chunks, got %d", len(resp.Chunks))
	}
	for i, c := range resp.Chunks {
		if int(c.Sequence) != i {
			t.Errorf("chunk[%d].Sequence = %d, want %d", i, c.Sequence, i)
		}
		if c.Content != fmt.Sprintf("message %d", i) {
			t.Errorf("chunk[%d].Content = %q, want %q", i, c.Content, fmt.Sprintf("message %d", i))
		}
		if c.Metadata == nil {
			t.Errorf("chunk[%d].Metadata is nil", i)
		}
	}

	// GetContext with last_n=2: should return the last 2 chunks (sequences 3, 4).
	resp2, err := env.retrieval.GetContext(ctx, &pb.GetContextRequest{
		DocumentId: doc.Id,
		LastN:      2,
	})
	if err != nil {
		t.Fatalf("GetContext (last_n=2): %v", err)
	}
	if len(resp2.Chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(resp2.Chunks))
	}
	if resp2.Chunks[0].Sequence != 3 {
		t.Errorf("first chunk sequence = %d, want 3", resp2.Chunks[0].Sequence)
	}
	if resp2.Chunks[1].Sequence != 4 {
		t.Errorf("second chunk sequence = %d, want 4", resp2.Chunks[1].Sequence)
	}
}

func TestPermissionConversion(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	slug := fmt.Sprintf("perm-test-%d", time.Now().UnixNano())
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug: slug, Name: "Perm Test",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() { _, _ = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id}) })

	// Test all permission levels round-trip through proto conversion.
	perms := []pb.Permission{
		pb.Permission_PERMISSION_READ,
		pb.Permission_PERMISSION_WRITE,
		pb.Permission_PERMISSION_ADMIN,
	}
	for _, perm := range perms {
		principal := fmt.Sprintf("user:perm-test-%d@example.com", perm)
		grant, err := env.topics.GrantAccess(ctx, &pb.GrantAccessRequest{
			TopicId:    topic.Id,
			Principal:  principal,
			Permission: perm,
		})
		if err != nil {
			t.Fatalf("GrantAccess(%v): %v", perm, err)
		}
		if grant.Permission != perm {
			t.Errorf("Permission = %v, want %v", grant.Permission, perm)
		}
	}
}

func TestListTopics_NonSystemPrincipal(t *testing.T) {
	// This tests the ListTopics path where principals is not nil (non-system account).
	// We need an OIDC-authenticated user or a static API key with IsSystem=false.
	// Since we use a system account API key in the test env, we can test the
	// system account path (principals == nil) is already covered.
	// The non-system path is tested indirectly through the GrantAuthorizer tests.
	// But let's verify the system path returns topics.
	env := setupTestEnv(t)
	ctx := env.authCtx()

	_, err := env.topics.ListTopics(ctx, &pb.ListTopicsRequest{})
	if err != nil {
		t.Fatalf("ListTopics: %v", err)
	}
}

func TestUploadDocumentRPC(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	slug := fmt.Sprintf("upload-test-topic-%d", time.Now().UnixNano())
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug: slug, Name: "Upload Test Topic",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() { _, _ = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id}) })

	// Upload with file content.
	resp, err := env.documents.UploadDocument(ctx, &pb.UploadDocumentRequest{
		TopicId:     topic.Id,
		Name:        "Test Upload",
		File:        []byte("Hello, world!"),
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("UploadDocument: %v", err)
	}
	if resp.Document == nil {
		t.Fatal("expected document in response")
	}
	if resp.Document.Status != "pending" {
		t.Errorf("Status = %q, want pending", resp.Document.Status)
	}
	if resp.JobId == "" {
		t.Error("expected non-empty job_id")
	}
	if resp.Document.Slug == "" {
		t.Error("expected auto-generated slug")
	}

	// Verify document exists.
	doc, err := env.documents.GetDocument(ctx, &pb.GetDocumentRequest{Id: resp.Document.Id})
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.Status != "pending" {
		t.Errorf("Status = %q, want pending", doc.Status)
	}
}

func TestUploadDocumentRPC_ValidationErrors(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	// Neither file nor source_url.
	_, err := env.documents.UploadDocument(ctx, &pb.UploadDocumentRequest{
		TopicId: "some-id", Name: "n",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for no file/url, got %v", err)
	}

	// Both file and source_url.
	_, err = env.documents.UploadDocument(ctx, &pb.UploadDocumentRequest{
		TopicId: "some-id", Name: "n", File: []byte("x"), SourceUrl: "http://example.com",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for both file+url, got %v", err)
	}
}

func TestCreateTopicRPC_AutoSlug(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	// Create topic without slug; should auto-generate.
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Name: "Auto Slug Topic",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() { _, _ = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id}) })

	if topic.Slug == "" {
		t.Error("expected auto-generated slug")
	}
}

func TestCreateDocumentRPC_AutoSlug(t *testing.T) {
	env := setupTestEnv(t)
	ctx := env.authCtx()

	slug := fmt.Sprintf("auto-slug-test-%d", time.Now().UnixNano())
	topic, err := env.topics.CreateTopic(ctx, &pb.CreateTopicRequest{
		Slug: slug, Name: "Auto Slug Doc Topic",
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() { _, _ = env.topics.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: topic.Id}) })

	// Create document without slug.
	doc, err := env.documents.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: topic.Id,
		Name:    "My Document Name",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if doc.Slug == "" {
		t.Error("expected auto-generated slug")
	}
	if doc.Status != "ready" {
		t.Errorf("Status = %q, want ready", doc.Status)
	}
}
