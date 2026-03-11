package pgvector

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
	"github.com/Tight-Line/creel/internal/vector/vectortest"
)

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

var errMock = errors.New("mock db error")

type mockDB struct {
	execFn  func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *mockDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

type mockRows struct {
	closed   bool
	nextOnce bool
	called   bool
	scanErr  error
	iterErr  error
}

func (r *mockRows) Close()                                       { r.closed = true }
func (r *mockRows) Err() error                                   { return r.iterErr }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) RawValues() [][]byte                          { return nil }
func (r *mockRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockRows) Conn() *pgx.Conn                              { return nil }

func (r *mockRows) Next() bool {
	if r.nextOnce && !r.called {
		r.called = true
		return true
	}
	return false
}

func (r *mockRows) Scan(_ ...any) error { return r.scanErr }

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

func TestNewWithDimension(t *testing.T) {
	b := NewWithDimension(&mockDB{}, 768)
	if b.EmbeddingDimension() != 768 {
		t.Errorf("dim = %d, want 768", b.EmbeddingDimension())
	}
}

func TestStore_BadMetadata(t *testing.T) {
	b := New(&mockDB{})
	err := b.Store(context.Background(), "id", []float64{1.0}, map[string]any{"bad": math.Inf(1)})
	if err == nil || !strings.Contains(err.Error(), "marshaling metadata") {
		t.Fatalf("expected marshaling metadata error, got %v", err)
	}
}

func TestStore_ExecError(t *testing.T) {
	db := &mockDB{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	b := New(db)
	err := b.Store(context.Background(), "id", []float64{1.0}, nil)
	if err == nil || !strings.Contains(err.Error(), "storing embedding") {
		t.Fatalf("expected storing embedding error, got %v", err)
	}
}

func TestDelete_ExecError(t *testing.T) {
	db := &mockDB{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	b := New(db)
	err := b.Delete(context.Background(), "id")
	if err == nil || !strings.Contains(err.Error(), "deleting embedding") {
		t.Fatalf("expected deleting embedding error, got %v", err)
	}
}

func TestSearch_BadMetadataFilter(t *testing.T) {
	b := New(&mockDB{})
	_, err := b.Search(context.Background(), []float64{1.0}, vector.Filter{
		Metadata: map[string]any{"bad": math.Inf(1)},
	}, 10)
	if err == nil || !strings.Contains(err.Error(), "marshaling metadata filter") {
		t.Fatalf("expected marshaling metadata filter error, got %v", err)
	}
}

func TestSearch_QueryError(t *testing.T) {
	db := &mockDB{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	b := New(db)
	_, err := b.Search(context.Background(), []float64{1.0}, vector.Filter{}, 10)
	if err == nil || !strings.Contains(err.Error(), "searching embeddings") {
		t.Fatalf("expected searching embeddings error, got %v", err)
	}
}

func TestSearch_ScanError(t *testing.T) {
	db := &mockDB{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	b := New(db)
	_, err := b.Search(context.Background(), []float64{1.0}, vector.Filter{}, 10)
	if err == nil || !strings.Contains(err.Error(), "scanning result") {
		t.Fatalf("expected scanning result error, got %v", err)
	}
}

func TestStoreBatch_PropagatesError(t *testing.T) {
	db := &mockDB{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	b := New(db)
	err := b.StoreBatch(context.Background(), []vector.StoreItem{
		{ID: "id1", Embedding: []float64{1.0}},
	})
	if err == nil || !strings.Contains(err.Error(), "storing embedding") {
		t.Fatalf("expected storing embedding error, got %v", err)
	}
}

func TestDeleteBatch_ExecError(t *testing.T) {
	db := &mockDB{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	b := New(db)
	err := b.DeleteBatch(context.Background(), []string{"id1"})
	if err == nil || !strings.Contains(err.Error(), "batch deleting embeddings") {
		t.Fatalf("expected batch deleting error, got %v", err)
	}
}

func TestPing_ExecError(t *testing.T) {
	db := &mockDB{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	b := New(db)
	err := b.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error from Ping, got nil")
	}
}

// ---------------------------------------------------------------------------
// Conformance (integration)
// ---------------------------------------------------------------------------

func TestPgvectorConformance(t *testing.T) {
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()
	if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}

	pgURL := pgCfg.URL()
	if err := store.RunMigrations(pgURL, "../../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	defer pool.Close()

	ids := vectortest.TestIDs
	allIDs := []string{ids.Chunk1, ids.Chunk2, ids.Chunk3, ids.Batch1, ids.Batch2}

	// Seed parent rows required by the chunk_embeddings FK constraint.
	// Clean up stale data first (cascade handles embeddings).
	_, _ = pool.Exec(ctx, `DELETE FROM topics WHERE slug = 'conformance-test'`)

	// Clean up ALL embeddings to prevent interference with the conformance
	// suite's unfiltered search queries. Other test packages may leave
	// embeddings that affect cosine similarity ordering.
	_, _ = pool.Exec(ctx, `TRUNCATE chunk_embeddings`)

	topicStore := store.NewTopicStore(pool)
	topic, err := topicStore.Create(ctx, "conformance-test", "Conformance", "test", "system:test", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	docStore := store.NewDocumentStore(pool)
	doc, err := docStore.Create(ctx, topic.ID, "conformance-doc", "Conformance Doc", "text", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	for i, id := range allIDs {
		// Insert chunk rows with the exact IDs the conformance suite expects.
		_, err := pool.Exec(ctx,
			`INSERT INTO chunks (id, document_id, sequence, content)
			 VALUES ($1, $2, $3, 'conformance test')
			 ON CONFLICT (id) DO NOTHING`,
			id, doc.ID, i,
		)
		if err != nil {
			t.Fatalf("seeding chunk %s: %v", id, err)
		}
	}

	backend := New(pool)
	vectortest.RunConformanceTests(t, backend)
}
