package server

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/retrieval"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// ---------------------------------------------------------------------------
// Mock infrastructure
// ---------------------------------------------------------------------------

// mockDBTX implements store.DBTX for injecting errors.
type mockDBTX struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	beginFn    func(ctx context.Context) (pgx.Tx, error)
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, errors.New("mockDBTX.Exec not configured")
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return nil, errors.New("mockDBTX.Query not configured")
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{err: errors.New("mockDBTX.QueryRow not configured")}
}

func (m *mockDBTX) Begin(ctx context.Context) (pgx.Tx, error) {
	if m.beginFn != nil {
		return m.beginFn(ctx)
	}
	return nil, errors.New("mockDBTX.Begin not configured")
}

// mockRow implements pgx.Row.
type mockRow struct{ err error }

func (r *mockRow) Scan(_ ...any) error { return r.err }

// mockAuthorizer implements auth.Authorizer.
type mockAuthorizer struct {
	checkErr         error
	accessibleTopics []string
	accessibleErr    error
}

func (m *mockAuthorizer) Check(_ context.Context, _ *auth.Principal, _ string, _ auth.Action) error {
	return m.checkErr
}

func (m *mockAuthorizer) AccessibleTopics(_ context.Context, _ *auth.Principal, _ auth.Action) ([]string, error) {
	return m.accessibleTopics, m.accessibleErr
}

// mockBackend implements vector.Backend.
type mockBackend struct {
	storeErr      error
	deleteErr     error
	searchResults []vector.SearchResult
	searchErr     error
	dim           int
}

func (b *mockBackend) EmbeddingDimension() int { return b.dim }

func (b *mockBackend) Store(_ context.Context, _ string, _ []float64, _ map[string]any) error {
	return b.storeErr
}

func (b *mockBackend) Delete(_ context.Context, _ string) error {
	return b.deleteErr
}

func (b *mockBackend) Search(_ context.Context, _ []float64, _ vector.Filter, _ int) ([]vector.SearchResult, error) {
	return b.searchResults, b.searchErr
}

func (b *mockBackend) StoreBatch(_ context.Context, _ []vector.StoreItem) error {
	return b.storeErr
}

func (b *mockBackend) DeleteBatch(_ context.Context, _ []string) error {
	return b.deleteErr
}

func (b *mockBackend) Ping(_ context.Context) error { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func systemCtx() context.Context {
	return auth.ContextWithPrincipal(context.Background(), &auth.Principal{
		ID:       "system:test",
		IsSystem: true,
	})
}

func userCtx() context.Context {
	return auth.ContextWithPrincipal(context.Background(), &auth.Principal{
		ID:     "user:test@example.com",
		Groups: []string{"group:eng"},
	})
}

// failDBTX returns a mockDBTX where every method returns an error.
func failDBTX() *mockDBTX {
	return &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("db error")}
		},
		beginFn: func(_ context.Context) (pgx.Tx, error) {
			return nil, errors.New("db error")
		},
	}
}

func requireCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Errorf("got code %v, want %v; err = %v", got, want, err)
	}
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestStringToChunkStatus(t *testing.T) {
	tests := []struct {
		in   string
		want pb.ChunkStatus
	}{
		{"active", pb.ChunkStatus_CHUNK_STATUS_ACTIVE},
		{"compacted", pb.ChunkStatus_CHUNK_STATUS_COMPACTED},
		{"unknown", pb.ChunkStatus_CHUNK_STATUS_UNSPECIFIED},
		{"", pb.ChunkStatus_CHUNK_STATUS_UNSPECIFIED},
	}
	for _, tt := range tests {
		if got := stringToChunkStatus(tt.in); got != tt.want {
			t.Errorf("stringToChunkStatus(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestProtoPermissionToString(t *testing.T) {
	tests := []struct {
		in   pb.Permission
		want string
	}{
		{pb.Permission_PERMISSION_READ, "read"},
		{pb.Permission_PERMISSION_WRITE, "write"},
		{pb.Permission_PERMISSION_ADMIN, "admin"},
		{pb.Permission_PERMISSION_UNSPECIFIED, "read"}, // default
	}
	for _, tt := range tests {
		if got := protoPermissionToString(tt.in); got != tt.want {
			t.Errorf("protoPermissionToString(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStringToProtoPermission(t *testing.T) {
	tests := []struct {
		in   string
		want pb.Permission
	}{
		{"read", pb.Permission_PERMISSION_READ},
		{"write", pb.Permission_PERMISSION_WRITE},
		{"admin", pb.Permission_PERMISSION_ADMIN},
		{"bogus", pb.Permission_PERMISSION_UNSPECIFIED},
		{"", pb.Permission_PERMISSION_UNSPECIFIED},
	}
	for _, tt := range tests {
		if got := stringToProtoPermission(tt.in); got != tt.want {
			t.Errorf("stringToProtoPermission(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestMapToStruct_UnsupportedValue(t *testing.T) {
	// A map with a channel value cannot be converted to a structpb.Struct.
	m := map[string]any{"bad": make(chan int)}
	got := mapToStruct(m)
	if got != nil {
		t.Errorf("expected nil for unsupported value type, got %v", got)
	}
}

func TestMapToStruct_NilMap(t *testing.T) {
	got := mapToStruct(nil)
	if got != nil {
		t.Errorf("expected nil for nil map, got %v", got)
	}
}

func TestStructToMap_NilStruct(t *testing.T) {
	got := structToMap(nil)
	if got != nil {
		t.Errorf("expected nil for nil struct, got %v", got)
	}
}

func TestStoreChunkToProto_CompactedBy(t *testing.T) {
	compactedBy := "chunk-999"
	embID := "emb-123"
	c := &store.Chunk{
		ID:          "chunk-1",
		DocumentID:  "doc-1",
		Sequence:    0,
		Content:     "hello",
		EmbeddingID: &embID,
		Status:      "compacted",
		CompactedBy: &compactedBy,
	}
	proto := storeChunkToProto(c)
	if proto.CompactedBy != "chunk-999" {
		t.Errorf("CompactedBy = %q, want %q", proto.CompactedBy, "chunk-999")
	}
	if proto.EmbeddingId != "emb-123" {
		t.Errorf("EmbeddingId = %q, want %q", proto.EmbeddingId, "emb-123")
	}
	if proto.Status != pb.ChunkStatus_CHUNK_STATUS_COMPACTED {
		t.Errorf("Status = %v, want COMPACTED", proto.Status)
	}
}

// ---------------------------------------------------------------------------
// Server.Run and GracefulStop
// ---------------------------------------------------------------------------

func TestServerRun_ListenError(t *testing.T) {
	srv := New(-1, auth.NewAPIKeyValidator(nil, nil), nil)
	err := srv.Run()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestServerRun_Success(t *testing.T) {
	srv := New(0, auth.NewAPIKeyValidator(nil, nil), nil)
	done := make(chan error, 1)
	go func() { done <- srv.Run() }()
	// Give the server a moment to start, then stop it.
	// GracefulStop causes Serve to return, which covers the Run success path
	// (listener creation and fmt.Printf).
	for i := 0; i < 100; i++ {
		// busy-wait for the server to be ready
	}
	srv.GracefulStop()
	<-done // error from Serve after stop is expected
}

func TestServerGracefulStop(t *testing.T) {
	srv := New(0, auth.NewAPIKeyValidator(nil, nil), nil)
	// GracefulStop on a server that has not started should not panic.
	srv.GracefulStop()
}

// ---------------------------------------------------------------------------
// AdminServer error paths
// ---------------------------------------------------------------------------

func TestAdminServer_CreateSystemAccount_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewAdminServer(nil, store.NewSystemAccountStore(db), "test")
	ctx := systemCtx()

	_, err := s.CreateSystemAccount(ctx, &pb.CreateSystemAccountRequest{Name: "x"})
	requireCode(t, err, codes.Internal)
}

func TestAdminServer_ListSystemAccounts_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewAdminServer(nil, store.NewSystemAccountStore(db), "test")
	ctx := systemCtx()

	_, err := s.ListSystemAccounts(ctx, &pb.ListSystemAccountsRequest{})
	requireCode(t, err, codes.Internal)
}

func TestAdminServer_DeleteSystemAccount_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewAdminServer(nil, store.NewSystemAccountStore(db), "test")
	ctx := systemCtx()

	_, err := s.DeleteSystemAccount(ctx, &pb.DeleteSystemAccountRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestAdminServer_RotateKey_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewAdminServer(nil, store.NewSystemAccountStore(db), "test")
	ctx := systemCtx()

	_, err := s.RotateKey(ctx, &pb.RotateKeyRequest{AccountId: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestAdminServer_RevokeKey_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewAdminServer(nil, store.NewSystemAccountStore(db), "test")
	ctx := systemCtx()

	_, err := s.RevokeKey(ctx, &pb.RevokeKeyRequest{AccountId: "some-id"})
	requireCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// TopicServer error paths
// ---------------------------------------------------------------------------

func TestTopicServer_CreateTopic_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.CreateTopic(ctx, &pb.CreateTopicRequest{Slug: "s", Name: "n"})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_GetTopic_AuthorizerError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.GetTopic(ctx, &pb.GetTopicRequest{Id: "topic-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestTopicServer_GetTopic_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.GetTopic(ctx, &pb.GetTopicRequest{Id: "topic-1"})
	requireCode(t, err, codes.NotFound)
}

func TestTopicServer_ListTopics_NonSystemPrincipal(t *testing.T) {
	// Non-system principals trigger the branch that appends ID+groups to principals.
	// The query will fail because our mock DB errors, producing codes.Internal.
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := userCtx()

	_, err := s.ListTopics(ctx, &pb.ListTopicsRequest{})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_ListTopics_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.ListTopics(ctx, &pb.ListTopicsRequest{})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_UpdateTopic_AuthorizerError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{Id: "topic-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestTopicServer_UpdateTopic_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.UpdateTopic(ctx, &pb.UpdateTopicRequest{Id: "topic-1"})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_DeleteTopic_AuthorizerError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: "topic-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestTopicServer_DeleteTopic_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: "topic-1"})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_GrantAccess_AuthorizerError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.GrantAccess(ctx, &pb.GrantAccessRequest{
		TopicId:    "topic-1",
		Principal:  "user:x",
		Permission: pb.Permission_PERMISSION_READ,
	})
	requireCode(t, err, codes.PermissionDenied)
}

func TestTopicServer_GrantAccess_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.GrantAccess(ctx, &pb.GrantAccessRequest{
		TopicId:    "topic-1",
		Principal:  "user:x",
		Permission: pb.Permission_PERMISSION_READ,
	})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_RevokeAccess_AuthorizerError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.RevokeAccess(ctx, &pb.RevokeAccessRequest{
		TopicId:   "topic-1",
		Principal: "user:x",
	})
	requireCode(t, err, codes.PermissionDenied)
}

func TestTopicServer_RevokeAccess_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.RevokeAccess(ctx, &pb.RevokeAccessRequest{
		TopicId:   "topic-1",
		Principal: "user:x",
	})
	requireCode(t, err, codes.Internal)
}

func TestTopicServer_ListGrants_AuthorizerError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.ListGrants(ctx, &pb.ListGrantsRequest{TopicId: "topic-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestTopicServer_ListGrants_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewTopicServer(store.NewTopicStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.ListGrants(ctx, &pb.ListGrantsRequest{TopicId: "topic-1"})
	requireCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// DocumentServer error paths
// ---------------------------------------------------------------------------

func TestDocumentServer_CreateDocument_AuthorizerError(t *testing.T) {
	db := failDBTX()
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: "topic-1", Slug: "s", Name: "n",
	})
	requireCode(t, err, codes.PermissionDenied)
}

func TestDocumentServer_CreateDocument_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.CreateDocument(ctx, &pb.CreateDocumentRequest{
		TopicId: "topic-1", Slug: "s", Name: "n",
	})
	requireCode(t, err, codes.Internal)
}

func TestDocumentServer_GetDocument_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.GetDocument(ctx, &pb.GetDocumentRequest{Id: "doc-1"})
	requireCode(t, err, codes.NotFound)
}

func TestDocumentServer_GetDocument_AuthorizerError(t *testing.T) {
	// Store succeeds (returns a doc), then authorizer denies.
	// We need QueryRow to succeed for the Get, then Check to fail.
	// The Get call scans 8 fields. We need a row that succeeds on Scan.
	// Simplest: use a special mockDBTX that returns a working row for Get.
	// Actually, store.Get scans into 8 fields from QueryRow. We can't easily
	// make the scan succeed with mockRow. Instead, note that GetDocument calls
	// store.Get first; if it fails, we get NotFound. To hit the authorizer
	// error, we need Get to succeed. That requires a real DB row.
	//
	// Alternative approach: the store.DocumentStore.Get does QueryRow().Scan().
	// Our mockRow just returns err from Scan. If err is nil, Scan returns nil
	// but does not populate fields; the Document will have zero values.
	// The handler then uses d.TopicID (empty string) for the authorizer check.
	// That is fine for our purposes.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil} // Scan succeeds with zero values
		},
	}
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.GetDocument(ctx, &pb.GetDocumentRequest{Id: "doc-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestDocumentServer_ListDocuments_AuthorizerError(t *testing.T) {
	db := failDBTX()
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.ListDocuments(ctx, &pb.ListDocumentsRequest{TopicId: "topic-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestDocumentServer_ListDocuments_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.ListDocuments(ctx, &pb.ListDocumentsRequest{TopicId: "topic-1"})
	requireCode(t, err, codes.Internal)
}

func TestDocumentServer_UpdateDocument_TopicIDError(t *testing.T) {
	db := failDBTX()
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.UpdateDocument(ctx, &pb.UpdateDocumentRequest{Id: "doc-1"})
	requireCode(t, err, codes.NotFound)
}

func TestDocumentServer_UpdateDocument_AuthorizerError(t *testing.T) {
	// TopicIDForDocument must succeed, then authorizer denies.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.UpdateDocument(ctx, &pb.UpdateDocumentRequest{Id: "doc-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestDocumentServer_UpdateDocument_StoreError(t *testing.T) {
	// TopicIDForDocument succeeds, authorizer passes, Update fails.
	// TopicIDForDocument uses QueryRow, then Update also uses QueryRow.
	// We need the first QueryRow (TopicIDForDocument) to succeed and
	// the second (Update) to fail.
	calls := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			calls++
			if calls == 1 {
				return &mockRow{err: nil} // TopicIDForDocument succeeds
			}
			return &mockRow{err: errors.New("db error")} // Update fails
		},
	}
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.UpdateDocument(ctx, &pb.UpdateDocumentRequest{Id: "doc-1"})
	requireCode(t, err, codes.Internal)
}

func TestDocumentServer_DeleteDocument_TopicIDError(t *testing.T) {
	db := failDBTX()
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.DeleteDocument(ctx, &pb.DeleteDocumentRequest{Id: "doc-1"})
	requireCode(t, err, codes.NotFound)
}

func TestDocumentServer_DeleteDocument_AuthorizerError(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.DeleteDocument(ctx, &pb.DeleteDocumentRequest{Id: "doc-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestDocumentServer_DeleteDocument_StoreError(t *testing.T) {
	// TopicIDForDocument uses QueryRow, Delete uses Exec.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := NewDocumentServer(store.NewDocumentStore(db), &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.DeleteDocument(ctx, &pb.DeleteDocumentRequest{Id: "doc-1"})
	requireCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// ChunkServer error paths
// ---------------------------------------------------------------------------

func TestChunkServer_IngestChunks_DocumentTopicIDError(t *testing.T) {
	db := failDBTX()
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: "doc-1",
		Chunks:     []*pb.ChunkInput{{Content: "c", Sequence: 0}},
	})
	requireCode(t, err, codes.NotFound)
}

func TestChunkServer_IngestChunks_AuthorizerError(t *testing.T) {
	// DocumentTopicID (on chunkStore) uses QueryRow. Must succeed for auth check.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: "doc-1",
		Chunks:     []*pb.ChunkInput{{Content: "c", Sequence: 0}},
	})
	requireCode(t, err, codes.PermissionDenied)
}

func TestChunkServer_IngestChunks_CreateError(t *testing.T) {
	// DocumentTopicID succeeds (first QueryRow), Create fails (second QueryRow).
	calls := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			calls++
			if calls == 1 {
				return &mockRow{err: nil} // DocumentTopicID
			}
			return &mockRow{err: errors.New("db error")} // Create
		},
	}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: "doc-1",
		Chunks:     []*pb.ChunkInput{{Content: "c", Sequence: 0}},
	})
	requireCode(t, err, codes.Internal)
}

func TestChunkServer_IngestChunks_BackendStoreError(t *testing.T) {
	// DocumentTopicID succeeds, Create succeeds, backend.Store fails.
	calls := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			calls++
			if calls <= 2 {
				return &mockRow{err: nil} // DocumentTopicID + Create
			}
			return &mockRow{err: errors.New("db error")}
		},
	}
	backend := &mockBackend{storeErr: errors.New("vector error")}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), backend, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: "doc-1",
		Chunks:     []*pb.ChunkInput{{Content: "c", Sequence: 0, Embedding: []float64{1.0}}},
	})
	requireCode(t, err, codes.Internal)
}

func TestChunkServer_IngestChunks_SetEmbeddingIDError(t *testing.T) {
	// DocumentTopicID succeeds, Create succeeds, backend.Store succeeds,
	// SetEmbeddingID fails (uses Exec).
	calls := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			calls++
			return &mockRow{err: nil}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.IngestChunks(ctx, &pb.IngestChunksRequest{
		DocumentId: "doc-1",
		Chunks:     []*pb.ChunkInput{{Content: "c", Sequence: 0, Embedding: []float64{1.0}}},
	})
	requireCode(t, err, codes.Internal)
}

func TestChunkServer_GetChunk_StoreGetError(t *testing.T) {
	db := failDBTX()
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.GetChunk(ctx, &pb.GetChunkRequest{Id: "chunk-1"})
	requireCode(t, err, codes.NotFound)
}

func TestChunkServer_GetChunk_DocumentTopicIDError(t *testing.T) {
	// Get succeeds (first QueryRow), DocumentTopicID fails (second QueryRow).
	calls := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			calls++
			if calls == 1 {
				return &mockRow{err: nil} // Get
			}
			return &mockRow{err: errors.New("db error")} // DocumentTopicID
		},
	}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.GetChunk(ctx, &pb.GetChunkRequest{Id: "chunk-1"})
	requireCode(t, err, codes.Internal)
}

func TestChunkServer_GetChunk_AuthorizerError(t *testing.T) {
	// Get and DocumentTopicID both succeed, authorizer denies.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.GetChunk(ctx, &pb.GetChunkRequest{Id: "chunk-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestChunkServer_DeleteChunk_StoreGetError(t *testing.T) {
	db := failDBTX()
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.DeleteChunk(ctx, &pb.DeleteChunkRequest{Id: "chunk-1"})
	requireCode(t, err, codes.NotFound)
}

func TestChunkServer_DeleteChunk_DocumentTopicIDError(t *testing.T) {
	calls := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			calls++
			if calls == 1 {
				return &mockRow{err: nil}
			}
			return &mockRow{err: errors.New("db error")}
		},
	}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.DeleteChunk(ctx, &pb.DeleteChunkRequest{Id: "chunk-1"})
	requireCode(t, err, codes.Internal)
}

func TestChunkServer_DeleteChunk_AuthorizerError(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{checkErr: errors.New("denied")})
	ctx := systemCtx()

	_, err := s.DeleteChunk(ctx, &pb.DeleteChunkRequest{Id: "chunk-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestChunkServer_DeleteChunk_BackendDeleteError(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	backend := &mockBackend{deleteErr: errors.New("vector error")}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), backend, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.DeleteChunk(ctx, &pb.DeleteChunkRequest{Id: "chunk-1"})
	requireCode(t, err, codes.Internal)
}

func TestChunkServer_DeleteChunk_StoreDeleteError(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	s := NewChunkServer(store.NewChunkStore(db), store.NewDocumentStore(db), &mockBackend{}, &mockAuthorizer{})
	ctx := systemCtx()

	_, err := s.DeleteChunk(ctx, &pb.DeleteChunkRequest{Id: "chunk-1"})
	requireCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// RetrievalServer error paths
// ---------------------------------------------------------------------------

func TestRetrievalServer_Search_SearcherError(t *testing.T) {
	// Create a Searcher with mock stores/authorizer/backend that will error.
	// The Searcher.Search calls AccessibleTopics first. If that errors,
	// it returns an error which the handler wraps as codes.Internal.
	db := failDBTX()
	chunkStore := store.NewChunkStore(db)
	authz := &mockAuthorizer{accessibleErr: errors.New("auth error")}
	backend := &mockBackend{}
	searcher := retrieval.NewSearcher(chunkStore, authz, backend)
	s := NewRetrievalServer(searcher)
	ctx := systemCtx()

	_, err := s.Search(ctx, &pb.SearchRequest{
		QueryEmbedding: []float64{1.0},
		TopK:           5,
	})
	requireCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// Nil principal tests (no auth context)
// ---------------------------------------------------------------------------

func TestNilPrincipal_TopicServer(t *testing.T) {
	db := &mockDBTX{}
	ts := store.NewTopicStore(db)
	authz := &mockAuthorizer{}
	s := NewTopicServer(ts, authz)
	ctx := context.Background() // no principal

	_, err := s.CreateTopic(ctx, &pb.CreateTopicRequest{Slug: "s", Name: "n"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.GetTopic(ctx, &pb.GetTopicRequest{Id: "id"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.ListTopics(ctx, &pb.ListTopicsRequest{})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.UpdateTopic(ctx, &pb.UpdateTopicRequest{Id: "id"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.DeleteTopic(ctx, &pb.DeleteTopicRequest{Id: "id"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.GrantAccess(ctx, &pb.GrantAccessRequest{TopicId: "t", Principal: "p", Permission: pb.Permission_PERMISSION_READ})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.RevokeAccess(ctx, &pb.RevokeAccessRequest{TopicId: "t", Principal: "p"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.ListGrants(ctx, &pb.ListGrantsRequest{TopicId: "t"})
	requireCode(t, err, codes.Unauthenticated)
}

func TestNilPrincipal_DocumentServer(t *testing.T) {
	db := &mockDBTX{}
	ds := store.NewDocumentStore(db)
	authz := &mockAuthorizer{}
	s := NewDocumentServer(ds, authz)
	ctx := context.Background()

	_, err := s.CreateDocument(ctx, &pb.CreateDocumentRequest{TopicId: "t", Slug: "s", Name: "n"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.GetDocument(ctx, &pb.GetDocumentRequest{Id: "id"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.ListDocuments(ctx, &pb.ListDocumentsRequest{TopicId: "t"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.UpdateDocument(ctx, &pb.UpdateDocumentRequest{Id: "id"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.DeleteDocument(ctx, &pb.DeleteDocumentRequest{Id: "id"})
	requireCode(t, err, codes.Unauthenticated)
}

func TestNilPrincipal_ChunkServer(t *testing.T) {
	db := &mockDBTX{}
	cs := store.NewChunkStore(db)
	ds := store.NewDocumentStore(db)
	authz := &mockAuthorizer{}
	backend := &mockBackend{}
	s := NewChunkServer(cs, ds, backend, authz)
	ctx := context.Background()

	_, err := s.IngestChunks(ctx, &pb.IngestChunksRequest{DocumentId: "d", Chunks: []*pb.ChunkInput{{Content: "c"}}})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.GetChunk(ctx, &pb.GetChunkRequest{Id: "id"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.DeleteChunk(ctx, &pb.DeleteChunkRequest{Id: "id"})
	requireCode(t, err, codes.Unauthenticated)
}

func TestNilPrincipal_RetrievalServer(t *testing.T) {
	db := &mockDBTX{}
	cs := store.NewChunkStore(db)
	authz := &mockAuthorizer{}
	backend := &mockBackend{}
	searcher := retrieval.NewSearcher(cs, authz, backend)
	s := NewRetrievalServer(searcher)
	ctx := context.Background()

	_, err := s.Search(ctx, &pb.SearchRequest{QueryEmbedding: []float64{1.0}})
	requireCode(t, err, codes.Unauthenticated)
}
