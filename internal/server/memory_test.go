package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// mockEmbedder implements EmbeddingProvider for tests.
type mockEmbedder struct {
	embedding []float64
	err       error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float64, error) {
	return m.embedding, m.err
}

func authedCtx(principalID string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), &auth.Principal{ID: principalID})
}

// ---------------------------------------------------------------------------
// GetMemory tests
// ---------------------------------------------------------------------------

func TestMemoryServer_GetMemory_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.GetMemory(context.Background(), &pb.GetMemoryRequest{Scope: "s"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_GetMemory_MissingScope(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.GetMemory(authedCtx("user:alice"), &pb.GetMemoryRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_GetMemory_StoreError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.GetMemory(authedCtx("user:alice"), &pb.GetMemoryRequest{Scope: "s"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// SearchMemories tests
// ---------------------------------------------------------------------------

func TestMemoryServer_SearchMemories_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.SearchMemories(context.Background(), &pb.SearchMemoriesRequest{Scope: "s"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_SearchMemories_MissingScope(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_SearchMemories_QueryTextNoEmbedder(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:     "s",
		QueryText: "hello",
	})
	assertCode(t, err, codes.FailedPrecondition)
}

func TestMemoryServer_SearchMemories_EmbedError(t *testing.T) {
	embedder := &mockEmbedder{err: errors.New("embed error")}
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, embedder)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:     "s",
		QueryText: "hello",
	})
	assertCode(t, err, codes.Internal)
}

func TestMemoryServer_SearchMemories_EmbeddingIDsError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:          "s",
		QueryEmbedding: []float64{1.0, 2.0},
	})
	assertCode(t, err, codes.Internal)
}

func TestMemoryServer_SearchMemories_NoEmbeddings(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &emptyRows{}, nil
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	resp, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:          "s",
		QueryEmbedding: []float64{1.0, 2.0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetResults()) != 0 {
		t.Fatalf("expected 0 results, got %d", len(resp.GetResults()))
	}
}

func TestMemoryServer_SearchMemories_BackendSearchError(t *testing.T) {
	// Return one embedding ID row so the code proceeds to backend search.
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &stringRows{values: []string{"emb_1"}}, nil
	}}
	backend := &mockBackend{searchErr: errors.New("search error")}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, nil)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:          "s",
		QueryEmbedding: []float64{1.0, 2.0},
	})
	assertCode(t, err, codes.Internal)
}

func TestMemoryServer_SearchMemories_NoSearchResults(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &stringRows{values: []string{"emb_1"}}, nil
	}}
	backend := &mockBackend{searchResults: nil}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, nil)
	resp, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:          "s",
		QueryEmbedding: []float64{1.0, 2.0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetResults()) != 0 {
		t.Fatalf("expected 0 results, got %d", len(resp.GetResults()))
	}
}

func TestMemoryServer_SearchMemories_FetchMemoriesError(t *testing.T) {
	callCount := 0
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		callCount++
		if callCount == 1 {
			// EmbeddingIDsByPrincipalScope
			return &stringRows{values: []string{"emb_1"}}, nil
		}
		// GetByEmbeddingIDs
		return nil, errors.New("db error")
	}}
	backend := &mockBackend{searchResults: []vector.SearchResult{{ChunkID: "emb_1", Score: 0.9}}}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, nil)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:          "s",
		QueryEmbedding: []float64{1.0, 2.0},
	})
	assertCode(t, err, codes.Internal)
}

func TestMemoryServer_SearchMemories_FallbackNoEmbedding(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &emptyRows{}, nil
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	resp, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope: "s",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetResults()) != 0 {
		t.Fatalf("expected 0 results, got %d", len(resp.GetResults()))
	}
}

func TestMemoryServer_SearchMemories_FallbackError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope: "s",
	})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// AddMemory tests
// ---------------------------------------------------------------------------

func TestMemoryServer_AddMemory_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.AddMemory(context.Background(), &pb.AddMemoryRequest{Content: "c"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_AddMemory_MissingContent(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_AddMemory_StoreError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errors.New("db error")}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c"})
	assertCode(t, err, codes.Internal)
}

func TestMemoryServer_AddMemory_DefaultScope(t *testing.T) {
	// Verify default scope is set when not provided.
	var capturedScope string
	db := &mockDBTX{queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
		capturedScope = args[1].(string)
		return &mockRow{err: errors.New("stop")}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, _ = srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c"})
	if capturedScope != "default" {
		t.Fatalf("expected default scope, got %q", capturedScope)
	}
}

func TestMemoryServer_AddMemory_WithTriple(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errors.New("stop")}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{
		Content:   "c",
		Subject:   "user",
		Predicate: "likes",
		Object:    "fishing",
	})
	// Error from mock is expected
	assertCode(t, err, codes.Internal)
}

func TestMemoryServer_AddMemory_WithMetadata(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errors.New("stop")}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	meta, _ := structpb.NewStruct(map[string]any{"key": "value"})
	_, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{
		Content:  "c",
		Metadata: meta,
	})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// UpdateMemory tests
// ---------------------------------------------------------------------------

func TestMemoryServer_UpdateMemory_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.UpdateMemory(context.Background(), &pb.UpdateMemoryRequest{Id: "id"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_UpdateMemory_MissingID(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_UpdateMemory_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
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
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
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
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{Id: "id", Content: "new"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// DeleteMemory tests
// ---------------------------------------------------------------------------

func TestMemoryServer_DeleteMemory_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.DeleteMemory(context.Background(), &pb.DeleteMemoryRequest{Id: "id"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_DeleteMemory_MissingID(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.DeleteMemory(authedCtx("user:alice"), &pb.DeleteMemoryRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_DeleteMemory_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
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
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
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
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.DeleteMemory(authedCtx("user:alice"), &pb.DeleteMemoryRequest{Id: "id"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// ListMemories tests
// ---------------------------------------------------------------------------

func TestMemoryServer_ListMemories_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.ListMemories(context.Background(), &pb.ListMemoriesRequest{Scope: "s"})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_ListMemories_MissingScope(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.ListMemories(authedCtx("user:alice"), &pb.ListMemoriesRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestMemoryServer_ListMemories_StoreError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.ListMemories(authedCtx("user:alice"), &pb.ListMemoriesRequest{Scope: "s"})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// ListScopes tests
// ---------------------------------------------------------------------------

func TestMemoryServer_ListScopes_Unauthenticated(t *testing.T) {
	srv := NewMemoryServer(store.NewMemoryStore(&mockDBTX{}), &mockBackend{}, nil)
	_, err := srv.ListScopes(context.Background(), &pb.ListScopesRequest{})
	assertCode(t, err, codes.Unauthenticated)
}

func TestMemoryServer_ListScopes_StoreError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errors.New("db error")
	}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	_, err := srv.ListScopes(authedCtx("user:alice"), &pb.ListScopesRequest{})
	assertCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// AddMemory embedding tests
// ---------------------------------------------------------------------------

func TestMemoryServer_AddMemory_WithEmbedding(t *testing.T) {
	// Need a mock store that succeeds on Create, then SetEmbeddingID.
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			callCount++
			return &memoryRow{mem: store.Memory{
				ID:        "mem-id",
				Principal: "user:alice",
				Scope:     "s",
				Content:   "c",
				Status:    "active",
			}}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	embedder := &mockEmbedder{embedding: []float64{1.0, 2.0, 3.0}}
	backend := &mockBackend{}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, embedder)
	resp, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetId() != "mem-id" {
		t.Fatalf("expected ID 'mem-id', got %q", resp.GetId())
	}
}

func TestMemoryServer_AddMemory_EmbedError_StillSucceeds(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &memoryRow{mem: store.Memory{
				ID:        "mem-id",
				Principal: "user:alice",
				Scope:     "s",
				Content:   "c",
				Status:    "active",
			}}
		},
	}
	embedder := &mockEmbedder{err: errors.New("embed error")}
	backend := &mockBackend{}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, embedder)
	resp, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetId() != "mem-id" {
		t.Fatalf("expected ID 'mem-id', got %q", resp.GetId())
	}
}

func TestMemoryServer_AddMemory_BackendStoreError_StillSucceeds(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &memoryRow{mem: store.Memory{
				ID:        "mem-id",
				Principal: "user:alice",
				Scope:     "s",
				Content:   "c",
				Status:    "active",
			}}
		},
	}
	embedder := &mockEmbedder{embedding: []float64{1.0, 2.0}}
	backend := &mockBackend{storeErr: errors.New("store error")}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, embedder)
	resp, err := srv.AddMemory(authedCtx("user:alice"), &pb.AddMemoryRequest{Content: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetId() != "mem-id" {
		t.Fatalf("expected ID 'mem-id', got %q", resp.GetId())
	}
}

// ---------------------------------------------------------------------------
// UpdateMemory embedding and content preservation tests
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
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, nil)
	// Empty content should preserve existing content
	resp, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{Id: "id"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetContent() != "original" {
		t.Fatalf("expected content 'original', got %q", resp.GetContent())
	}
}

func TestMemoryServer_UpdateMemory_WithEmbedding(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				return &memoryRow{mem: store.Memory{
					ID:        "id",
					Principal: "user:alice",
					Scope:     "s",
					Content:   "old content",
					Status:    "active",
				}}
			}
			return &memoryRow{mem: store.Memory{
				ID:        "id",
				Principal: "user:alice",
				Scope:     "s",
				Content:   "new content",
				Status:    "active",
			}}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	embedder := &mockEmbedder{embedding: []float64{1.0, 2.0}}
	backend := &mockBackend{}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, embedder)
	resp, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{
		Id:      "id",
		Content: "new content",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetContent() != "new content" {
		t.Fatalf("expected content 'new content', got %q", resp.GetContent())
	}
}

func TestMemoryServer_UpdateMemory_EmbedError_StillSucceeds(t *testing.T) {
	callCount := 0
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
		callCount++
		if callCount == 1 {
			return &memoryRow{mem: store.Memory{
				ID:        "id",
				Principal: "user:alice",
				Scope:     "s",
				Content:   "old content",
				Status:    "active",
			}}
		}
		return &memoryRow{mem: store.Memory{
			ID:        "id",
			Principal: "user:alice",
			Scope:     "s",
			Content:   "new content",
			Status:    "active",
		}}
	}}
	embedder := &mockEmbedder{err: errors.New("embed error")}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, embedder)
	resp, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{
		Id:      "id",
		Content: "new content",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetContent() != "new content" {
		t.Fatalf("expected content 'new content', got %q", resp.GetContent())
	}
}

func TestMemoryServer_UpdateMemory_BackendStoreError_StillSucceeds(t *testing.T) {
	callCount := 0
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
		callCount++
		if callCount == 1 {
			return &memoryRow{mem: store.Memory{
				ID:        "id",
				Principal: "user:alice",
				Scope:     "s",
				Content:   "old content",
				Status:    "active",
			}}
		}
		return &memoryRow{mem: store.Memory{
			ID:        "id",
			Principal: "user:alice",
			Scope:     "s",
			Content:   "new content",
			Status:    "active",
		}}
	}}
	embedder := &mockEmbedder{embedding: []float64{1.0}}
	backend := &mockBackend{storeErr: errors.New("store error")}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, embedder)
	resp, err := srv.UpdateMemory(authedCtx("user:alice"), &pb.UpdateMemoryRequest{
		Id:      "id",
		Content: "new content",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetContent() != "new content" {
		t.Fatalf("expected content 'new content', got %q", resp.GetContent())
	}
}

// ---------------------------------------------------------------------------
// SearchMemories successful path tests
// ---------------------------------------------------------------------------

func TestMemoryServer_SearchMemories_SuccessfulSearch(t *testing.T) {
	callCount := 0
	embID := "emb_1"
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		callCount++
		if callCount == 1 {
			// EmbeddingIDsByPrincipalScope
			return &stringRows{values: []string{"emb_1"}}, nil
		}
		// GetByEmbeddingIDs
		return &memoryRows{memories: []store.Memory{{
			ID:          "mem-1",
			Principal:   "user:alice",
			Scope:       "s",
			Content:     "test memory",
			EmbeddingID: &embID,
			Status:      "active",
		}}}, nil
	}}
	backend := &mockBackend{searchResults: []vector.SearchResult{{ChunkID: "emb_1", Score: 0.95}}}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, nil)
	resp, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:          "s",
		QueryEmbedding: []float64{1.0, 2.0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetResults()) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.GetResults()))
	}
	if resp.GetResults()[0].GetScore() != 0.95 {
		t.Fatalf("expected score 0.95, got %f", resp.GetResults()[0].GetScore())
	}
}

func TestMemoryServer_SearchMemories_MissingMemoryForResult(t *testing.T) {
	callCount := 0
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		callCount++
		if callCount == 1 {
			return &stringRows{values: []string{"emb_1"}}, nil
		}
		// Return empty memoryRows so no memory matches "emb_1"
		return &memoryRows{}, nil
	}}
	backend := &mockBackend{searchResults: []vector.SearchResult{{ChunkID: "emb_1", Score: 0.9}}}
	srv := NewMemoryServer(store.NewMemoryStore(db), backend, nil)
	resp, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:          "s",
		QueryEmbedding: []float64{1.0, 2.0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetResults()) != 0 {
		t.Fatalf("expected 0 results, got %d", len(resp.GetResults()))
	}
}

func TestMemoryServer_SearchMemories_WithQueryText(t *testing.T) {
	callCount := 0
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		callCount++
		// EmbeddingIDsByPrincipalScope returns empty to trigger empty response
		return &emptyRows{}, nil
	}}
	embedder := &mockEmbedder{embedding: []float64{1.0, 2.0}}
	srv := NewMemoryServer(store.NewMemoryStore(db), &mockBackend{}, embedder)
	resp, err := srv.SearchMemories(authedCtx("user:alice"), &pb.SearchMemoriesRequest{
		Scope:     "s",
		QueryText: "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetResults()) != 0 {
		t.Fatalf("expected 0 results, got %d", len(resp.GetResults()))
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

// stringRows implements pgx.Rows returning string values.
type stringRows struct {
	values []string
	idx    int
}

func (r *stringRows) Close()                                       {}
func (r *stringRows) Err() error                                   { return nil }
func (r *stringRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *stringRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *stringRows) RawValues() [][]byte                          { return nil }
func (r *stringRows) Conn() *pgx.Conn                              { return nil }
func (r *stringRows) Values() ([]any, error)                       { return nil, nil }

func (r *stringRows) Next() bool {
	if r.idx < len(r.values) {
		r.idx++
		return true
	}
	return false
}

func (r *stringRows) Scan(dest ...any) error {
	*dest[0].(*string) = r.values[r.idx-1]
	return nil
}

// memoryRows implements pgx.Rows returning scannable memory data.
type memoryRows struct {
	memories []store.Memory
	idx      int
}

func (r *memoryRows) Close()                                       {}
func (r *memoryRows) Err() error                                   { return nil }
func (r *memoryRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *memoryRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *memoryRows) RawValues() [][]byte                          { return nil }
func (r *memoryRows) Conn() *pgx.Conn                              { return nil }
func (r *memoryRows) Values() ([]any, error)                       { return nil, nil }

func (r *memoryRows) Next() bool {
	if r.idx < len(r.memories) {
		r.idx++
		return true
	}
	return false
}

func (r *memoryRows) Scan(dest ...any) error {
	m := r.memories[r.idx-1]
	*dest[0].(*string) = m.ID
	*dest[1].(*string) = m.Principal
	*dest[2].(*string) = m.Scope
	*dest[3].(*string) = m.Content
	*dest[4].(**string) = m.EmbeddingID
	*dest[5].(**string) = m.Subject
	*dest[6].(**string) = m.Predicate
	*dest[7].(**string) = m.Object
	*dest[8].(**string) = m.SourceChunkID
	*dest[9].(*string) = m.Status
	*dest[10].(**time.Time) = m.InvalidatedAt
	*dest[11].(*[]byte) = []byte("{}")
	*dest[12].(*time.Time) = m.CreatedAt
	*dest[13].(*time.Time) = m.UpdatedAt
	return nil
}
