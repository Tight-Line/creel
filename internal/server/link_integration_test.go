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

	"github.com/jackc/pgx/v5/pgxpool"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/server"
	"github.com/Tight-Line/creel/internal/store"
)

type linkTestEnv struct {
	pool       *pgxpool.Pool
	conn       *grpc.ClientConn
	apiKey     string
	cleanup    func()
	link       pb.LinkServiceClient
	topicStore *store.TopicStore
	docStore   *store.DocumentStore
	chunkStore *store.ChunkStore
}

func setupLinkTestEnv(t *testing.T) *linkTestEnv {
	t.Helper()

	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()

	if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	pgURL := pgCfg.URL()
	if err := store.RunMigrations(pgURL, "../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	linkStore := store.NewLinkStore(pool)
	grantStore := store.NewGrantStore(pool)
	accountStore := store.NewSystemAccountStore(pool)
	authorizer := auth.NewGrantAuthorizer(grantStore)

	acct, rawKey, err := accountStore.Create(ctx, "link-test", "link integration test")
	if err != nil {
		pool.Close()
		t.Fatalf("creating test system account: %v", err)
	}

	apiKeyValidator := auth.NewAPIKeyValidator(nil, accountStore)

	srv := server.New(0, apiKeyValidator, nil)
	linkServer := server.NewLinkServer(linkStore, chunkStore, authorizer)
	pb.RegisterLinkServiceServer(srv.GRPCServer(), linkServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		pool.Close()
		t.Fatalf("listening: %v", err)
	}

	go func() { _ = srv.GRPCServer().Serve(lis) }()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.GracefulStop()
		pool.Close()
		t.Fatalf("dialing: %v", err)
	}

	env := &linkTestEnv{
		pool:       pool,
		conn:       conn,
		apiKey:     rawKey,
		topicStore: topicStore,
		docStore:   docStore,
		chunkStore: chunkStore,
		cleanup: func() {
			_ = accountStore.Delete(ctx, acct.ID)
			_ = conn.Close()
			srv.GracefulStop()
			pool.Close()
		},
		link: pb.NewLinkServiceClient(conn),
	}

	t.Cleanup(env.cleanup)
	return env
}

func (e *linkTestEnv) authCtx() context.Context {
	md := metadata.Pairs("authorization", "Bearer "+e.apiKey)
	return metadata.NewOutgoingContext(context.Background(), md)
}

func TestLinkService_Integration_CRUD(t *testing.T) {
	env := setupLinkTestEnv(t)
	ctx := env.authCtx()

	// Create a topic, document, and two chunks.
	topic, err := env.topicStore.Create(context.Background(),
		fmt.Sprintf("link-test-%d", time.Now().UnixNano()),
		"Link Test", "", "system:link-test", nil, nil, nil, false)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = env.topicStore.Delete(context.Background(), topic.ID) })

	doc, err := env.docStore.CreateWithStatus(context.Background(),
		topic.ID, "link-doc", "Link Doc", "reference", "ready", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	chunk1, err := env.chunkStore.Create(context.Background(), doc.ID, "First chunk", 1, nil)
	if err != nil {
		t.Fatalf("creating chunk 1: %v", err)
	}
	chunk2, err := env.chunkStore.Create(context.Background(), doc.ID, "Second chunk", 2, nil)
	if err != nil {
		t.Fatalf("creating chunk 2: %v", err)
	}

	// Create a link.
	link, err := env.link.CreateLink(ctx, &pb.CreateLinkRequest{
		SourceChunkId: chunk1.ID,
		TargetChunkId: chunk2.ID,
		LinkType:      pb.LinkType_LINK_TYPE_MANUAL,
	})
	if err != nil {
		t.Fatalf("creating link: %v", err)
	}
	if link.GetId() == "" {
		t.Error("expected link ID")
	}
	if link.GetSourceChunkId() != chunk1.ID {
		t.Errorf("source = %q, want %q", link.GetSourceChunkId(), chunk1.ID)
	}
	if link.GetTargetChunkId() != chunk2.ID {
		t.Errorf("target = %q, want %q", link.GetTargetChunkId(), chunk2.ID)
	}
	if link.GetLinkType() != pb.LinkType_LINK_TYPE_MANUAL {
		t.Errorf("link_type = %v, want MANUAL", link.GetLinkType())
	}

	// List links for source chunk.
	listResp, err := env.link.ListLinks(ctx, &pb.ListLinksRequest{
		ChunkId: chunk1.ID,
	})
	if err != nil {
		t.Fatalf("listing links: %v", err)
	}
	if len(listResp.GetLinks()) != 1 {
		t.Fatalf("expected 1 link, got %d", len(listResp.GetLinks()))
	}

	// List links for target chunk without backlinks should return 0.
	listResp, err = env.link.ListLinks(ctx, &pb.ListLinksRequest{
		ChunkId: chunk2.ID,
	})
	if err != nil {
		t.Fatalf("listing links from target: %v", err)
	}
	if len(listResp.GetLinks()) != 0 {
		t.Errorf("expected 0 links from target without backlinks, got %d", len(listResp.GetLinks()))
	}

	// List with backlinks from target chunk should return 1.
	listResp, err = env.link.ListLinks(ctx, &pb.ListLinksRequest{
		ChunkId:          chunk2.ID,
		IncludeBacklinks: true,
	})
	if err != nil {
		t.Fatalf("listing links with backlinks: %v", err)
	}
	if len(listResp.GetLinks()) != 1 {
		t.Errorf("expected 1 link with backlinks, got %d", len(listResp.GetLinks()))
	}

	// Delete the link.
	_, err = env.link.DeleteLink(ctx, &pb.DeleteLinkRequest{Id: link.GetId()})
	if err != nil {
		t.Fatalf("deleting link: %v", err)
	}

	// Verify it's gone.
	listResp, err = env.link.ListLinks(ctx, &pb.ListLinksRequest{
		ChunkId:          chunk1.ID,
		IncludeBacklinks: true,
	})
	if err != nil {
		t.Fatalf("listing after delete: %v", err)
	}
	if len(listResp.GetLinks()) != 0 {
		t.Errorf("expected 0 links after delete, got %d", len(listResp.GetLinks()))
	}
}

func TestLinkService_Integration_NotFound(t *testing.T) {
	env := setupLinkTestEnv(t)
	ctx := env.authCtx()

	// Create link with non-existent source chunk.
	_, err := env.link.CreateLink(ctx, &pb.CreateLinkRequest{
		SourceChunkId: "00000000-0000-0000-0000-000000000000",
		TargetChunkId: "00000000-0000-0000-0000-000000000001",
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}

	// Delete non-existent link.
	_, err = env.link.DeleteLink(ctx, &pb.DeleteLinkRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound for delete, got %v", err)
	}

	// List links for non-existent chunk.
	_, err = env.link.ListLinks(ctx, &pb.ListLinksRequest{
		ChunkId: "00000000-0000-0000-0000-000000000000",
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound for list, got %v", err)
	}
}
