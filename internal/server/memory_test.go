package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

func authedCtx(principalID string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), &auth.Principal{ID: principalID})
}

// ---------------------------------------------------------------------------
// GetMemory tests
// ---------------------------------------------------------------------------

func TestMemoryServer_GetMemory_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.GetMemory(context.Background(), &pb.GetMemoryRequest{Scope: "s"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_GetMemory_MissingScope(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.GetMemory(authedCtx("user:alice"), &pb.GetMemoryRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_GetMemory_StoreError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.GetMemory(authedCtx("user:alice"), &pb.GetMemoryRequest{Scope: "s"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// SearchMemories tests
// ---------------------------------------------------------------------------

func TestMemoryServer_SearchMemories_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.SearchMemories(context.Background(), &pb.SearchMemoriesRequest{Scope: "s"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_SearchMemories_MissingScope(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_SearchMemories_Success(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &emptyRows{}, nil
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	resp, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{Scope: "s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetResults()) != 0 {
		t.Fatalf("expected 0 results, got %d", len(resp.GetResults()))
	}
}

func TestMemoryServer_SearchMemories_StoreError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{Scope: "s"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// AddMemory tests
// ---------------------------------------------------------------------------

func TestMemoryServer_AddMemory_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), store.NewJobStore(&mockDBTX{}))
	_, err := srv.AddMemory(context.Background(), &pb.AddMemoryRequest{Content: "c"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_AddMemory_MissingContent(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), store.NewJobStore(&mockDBTX{}))
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_AddMemory_NoJobStore(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c"})
	assertCode(t, err, codes.FailedPrecondition)
}

func TestMemoryServer_AddMemory_JobCreationError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errors.New("db error")}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), store.NewJobStore(db))
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c"})
	assertCode(t, err, codes.Internal)
}

func TestMemoryServer_AddMemory_Success(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
		return &jobRow{id: "job-1"}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), store.NewJobStore(db))
	resp, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c", Scope: "fishing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetJobId() != "job-1" {
		t.Fatalf("expected job ID 'job-1', got %q", resp.GetJobId())
	}
}

func TestMemoryServer_AddMemory_DefaultScope(t *testing.T) {
	var capturedProgress []byte
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
		// CreateDocless: args are jobType, progressJSON
		if len(args) >= 2 {
			if b, ok := args[1].([]byte); ok {
				capturedProgress = b
			}
		}
		return &jobRow{id: "job-1"}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), store.NewJobStore(db))
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedProgress != nil {
		var progress map[string]any
		_ = json.Unmarshal(capturedProgress, &progress)
		if progress["scope"] != "default" {
			t.Fatalf("expected default scope in progress, got %q", progress["scope"])
		}
	}
}

func TestMemoryServer_AddMemory_WithTriple(t *testing.T) {
	var capturedProgress []byte
	db := &mockDBTX{queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
		for _, arg := range args {
			if b, ok := arg.([]byte); ok {
				capturedProgress = b
			}
		}
		return &jobRow{id: "job-1"}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), store.NewJobStore(db))
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{
		Content:   "User likes fly fishing",
		Scope:     "prefs",
		Subject:   "user",
		Predicate: "likes",
		Object:    "fly fishing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedProgress == nil {
		t.Fatal("expected progress to be captured")
	}
	var progress map[string]any
	_ = json.Unmarshal(capturedProgress, &progress)
	if progress["subject"] != "user" {
		t.Fatalf("expected subject 'user', got %q", progress["subject"])
	}
	if progress["predicate"] != "likes" {
		t.Fatalf("expected predicate 'likes', got %q", progress["predicate"])
	}
	if progress["object"] != "fly fishing" {
		t.Fatalf("expected object 'fly fishing', got %q", progress["object"])
	}
}

// ---------------------------------------------------------------------------
// UpdateMemory tests
// ---------------------------------------------------------------------------

func TestMemoryServer_UpdateMemory_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.UpdateMemory(context.Background(), &pb.UpdateMemoryRequest{Id: "id"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_UpdateMemory_MissingID(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_UpdateMemory_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{Id: "id"})
	assertCode(t, err, codes.NotFound)
}

func TestMemoryServer_UpdateMemory_NotOwner(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
		return &memoryRow{mem: store.Memory{
			ID:        "id",
			Principal: "user:bob",
			Scope:     "s",
			Content:   "c",
			Status:    "active",
		}}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{Id: "id", Content: "new"})
	assertCode(t, err, codes.PermissionDenied)
}

func TestMemoryServer_UpdateMemory_StoreError(t *testing.T) {
	callCount := 0
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
		callCount++
		if callCount == 1 {
			return &memoryRow{mem: store.Memory{
				ID:        "id",
				Principal: "user:alice",
				Scope:     "s",
				Content:   "c",
				Status:    "active",
			}}
		}
		return &mockRow{err: errors.New("db error")}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{Id: "id", Content: "new"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// DeleteMemory tests
// ---------------------------------------------------------------------------

func TestMemoryServer_DeleteMemory_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.DeleteMemory(context.Background(), &pb.DeleteMemoryRequest{Id: "id"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_DeleteMemory_MissingID(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.DeleteMemory(authedCtx("user:alice"), &pb.DeleteMemoryRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_DeleteMemory_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.DeleteMemory(authedCtx("user:alice"), &pb.DeleteMemoryRequest{Id: "id"})
	assertCode(t, err, codes.NotFound)
}

func TestMemoryServer_DeleteMemory_NotOwner(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &memoryRow{mem: store.Memory{
			ID:        "id",
			Principal: "user:bob",
			Scope:     "s",
			Content:   "c",
			Status:    "active",
		}}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.DeleteMemory(authedCtx("user:alice"), &pb.DeleteMemoryRequest{Id: "id"})
	assertCode(t, err, codes.PermissionDenied)
}

func TestMemoryServer_DeleteMemory_InvalidateError(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			return &memoryRow{mem: store.Memory{
				ID:        "id",
				Principal: "user:alice",
				Scope:     "s",
				Content:   "c",
				Status:    "active",
			}}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errors.New("db error")
		},
	}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.DeleteMemory(authedCtx("user:alice"), &pb.DeleteMemoryRequest{Id: "id"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// ListMemories tests
// ---------------------------------------------------------------------------

func TestMemoryServer_ListMemories_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.ListMemories(context.Background(), &pb.ListMemoriesRequest{Scope: "s"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_ListMemories_MissingScope(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.ListMemories(authedCtx("user:alice"), &pb.ListMemoriesRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_ListMemories_StoreError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.ListMemories(authedCtx("user:alice"), &pb.ListMemoriesRequest{Scope: "s"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// ListScopes tests
// ---------------------------------------------------------------------------

func TestMemoryServer_ListScopes_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), nil)
	_, err := srv.ListScopes(context.Background(), &pb.ListScopesRequest{})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_ListScopes_StoreError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	_, err := srv.ListScopes(authedCtx("user:alice"), &pb.ListScopesRequest{})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// jobRow mock for AddMemory tests (scans a processing job from CreateDocless)
// ---------------------------------------------------------------------------

// jobRow implements pgx.Row returning a minimal ProcessingJob for scanJob.
type jobRow struct {
	id string
}

func (r *jobRow) Scan(dest ...any) error {
	// scanJob expects: id, docID(*string), jobType, status, progressBytes, error, startedAt, completedAt, createdAt
	*dest[0].(*string) = r.id
	*dest[1].(**string) = nil // document_id is NULL
	*dest[2].(*string) = "memory_maintenance"
	*dest[3].(*string) = "queued"
	*dest[4].(*[]byte) = []byte("{}")
	*dest[5].(**string) = nil
	*dest[6].(**time.Time) = nil
	*dest[7].(**time.Time) = nil
	*dest[8].(*time.Time) = time.Now()
	return nil
}

// ---------------------------------------------------------------------------
// UpdateMemory content preservation test
// ---------------------------------------------------------------------------

func TestMemoryServer_UpdateMemory_PreservesContentWhenEmpty(t *testing.T) {
	callCount := 0
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
		callCount++
		return &memoryRow{mem: store.Memory{
			ID:        "id",
			Principal: "user:alice",
			Scope:     "s",
			Content:   "original",
			Status:    "active",
			Metadata:  map[string]any{"key": "val"},
		}}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), nil)
	// Empty content should preserve existing content
	resp, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{Id: "id"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetContent() != "original" {
		t.Fatalf("expected content 'original', got %q", resp.GetContent())
	}
}

// ---------------------------------------------------------------------------
// storeMemoryToProto coverage for SourceChunkID and InvalidatedAt
// ---------------------------------------------------------------------------

func TestStoreMemoryToProto_AllFields(t *testing.T) {
	subj := "user"
	pred := "prefers"
	obj := "concise"
	srcChunk := "chunk-123"
	now := time.Now()
	m := &store.Memory{
		ID:            "id",
		Principal:     "user:alice",
		Scope:         "s",
		Content:       "c",
		Subject:       &subj,
		Predicate:     &pred,
		Object:        &obj,
		SourceChunkID: &srcChunk,
		Status:        "invalidated",
		InvalidatedAt: &now,
		Metadata:      map[string]any{"key": "val"},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	proto := storeMemoryToProto(m)
	if proto.GetSourceChunkId() != "chunk-123" {
		t.Fatalf("expected source_chunk_id 'chunk-123', got %q", proto.GetSourceChunkId())
	}
	if proto.GetInvalidatedAt() == nil {
		t.Fatal("expected invalidated_at to be set")
	}
}

// ---------------------------------------------------------------------------
// Helper types
// ---------------------------------------------------------------------------

func assertCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %v, got nil", code)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != code {
		t.Fatalf("expected code %v, got %v: %s", code, st.Code(), st.Message())
	}
}

// memoryRow implements pgx.Row returning a store.Memory.
type memoryRow struct {
	mem store.Memory
}

func (r *memoryRow) Scan(dest ...any) error {
	*dest[0].(*string) = r.mem.ID
	*dest[1].(*string) = r.mem.Principal
	*dest[2].(*string) = r.mem.Scope
	*dest[3].(*string) = r.mem.Content
	*dest[4].(**string) = r.mem.EmbeddingID
	*dest[5].(**string) = r.mem.Subject
	*dest[6].(**string) = r.mem.Predicate
	*dest[7].(**string) = r.mem.Object
	*dest[8].(**string) = r.mem.SourceChunkID
	*dest[9].(*string) = r.mem.Status
	*dest[10].(**time.Time) = r.mem.InvalidatedAt
	*dest[11].(*[]byte) = []byte("{}")
	*dest[12].(*time.Time) = r.mem.CreatedAt
	*dest[13].(*time.Time) = r.mem.UpdatedAt
	return nil
}
