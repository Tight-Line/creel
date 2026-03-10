package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// compactionMockDB is a mock DBTX for compaction worker tests.
type compactionMockDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *compactionMockDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *compactionMockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &compactEmptyRows{}, nil
}

func (m *compactionMockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{err: errors.New("not configured")}
}

func (m *compactionMockDB) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

func TestCompactionWorker_Type(t *testing.T) {
	w := &CompactionWorker{}
	if w.Type() != "compaction" {
		t.Errorf("expected type compaction, got %s", w.Type())
	}
}

func TestCompactionWorker_TooFewChunks(t *testing.T) {
	// With only 1 active chunk, compaction should be a no-op.
	db := &compactionMockDB{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM chunks") {
				return &compactChunkRows{chunks: []compactChunkData{
					{id: "c1", content: "hello"},
				}}, nil
			}
			return &compactEmptyRows{}, nil
		},
	}
	w := NewCompactionWorker(
		store.NewChunkStore(db),
		store.NewLinkStore(db),
		store.NewCompactionStore(db),
		store.NewDocumentStore(db),
		store.NewJobStore(db),
		&compactMockVectorBackend{},
		NewStubEmbeddingProvider(3),
		llm.NewStubProvider(`{"facts": []}`),
	)
	err := w.Process(context.Background(), &store.ProcessingJob{
		ID: "j1", DocumentID: "d1",
	})
	if err != nil {
		t.Fatalf("expected no error for too few chunks, got %v", err)
	}
}

func TestCompactionWorker_LLMError(t *testing.T) {
	db := &compactionMockDB{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM chunks") {
				return &compactChunkRows{chunks: []compactChunkData{
					{id: "c1", content: "hello"},
					{id: "c2", content: "world"},
				}}, nil
			}
			return &compactEmptyRows{}, nil
		},
	}

	failingLLM := &compactFailingLLM{err: errors.New("LLM down")}
	w := NewCompactionWorker(
		store.NewChunkStore(db),
		store.NewLinkStore(db),
		store.NewCompactionStore(db),
		store.NewDocumentStore(db),
		store.NewJobStore(db),
		&compactMockVectorBackend{},
		NewStubEmbeddingProvider(3),
		failingLLM,
	)
	err := w.Process(context.Background(), &store.ProcessingJob{
		ID: "j1", DocumentID: "d1",
	})
	if err == nil || !strings.Contains(err.Error(), "LLM compaction") {
		t.Errorf("expected LLM error, got %v", err)
	}
}

func TestCompactionWorker_EmptyLLMResponse(t *testing.T) {
	db := &compactionMockDB{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM chunks") {
				return &compactChunkRows{chunks: []compactChunkData{
					{id: "c1", content: "hello"},
					{id: "c2", content: "world"},
				}}, nil
			}
			return &compactEmptyRows{}, nil
		},
	}

	w := NewCompactionWorker(
		store.NewChunkStore(db),
		store.NewLinkStore(db),
		store.NewCompactionStore(db),
		store.NewDocumentStore(db),
		store.NewJobStore(db),
		&compactMockVectorBackend{},
		NewStubEmbeddingProvider(3),
		llm.NewStubProvider("   "), // whitespace-only response
	)
	err := w.Process(context.Background(), &store.ProcessingJob{
		ID: "j1", DocumentID: "d1",
	})
	if err == nil || !strings.Contains(err.Error(), "empty compaction summary") {
		t.Errorf("expected empty summary error, got %v", err)
	}
}

func TestCompactionWorker_WithChunkIDs(t *testing.T) {
	// Test the specific chunk IDs path.
	queryRowCount := 0
	db := &compactionMockDB{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM chunks") {
				// GetMultiple returns rows
				return &compactChunkRows{chunks: []compactChunkData{
					{id: "c1", content: "hello"},
					{id: "c2", content: "world"},
				}}, nil
			}
			return &compactEmptyRows{}, nil
		},
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			queryRowCount++
			if strings.Contains(sql, "MAX(sequence)") {
				return &compactSeqRow{seq: 3}
			}
			if strings.Contains(sql, "INSERT INTO chunks") {
				return &compactNewChunkRow{id: "sc1"}
			}
			if strings.Contains(sql, "INSERT INTO compaction_records") {
				return &compactNewRecordRow{id: "r1"}
			}
			return &mockRow{err: errors.New("unexpected query")}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 2"), nil
		},
	}

	w := NewCompactionWorker(
		store.NewChunkStore(db),
		store.NewLinkStore(db),
		store.NewCompactionStore(db),
		store.NewDocumentStore(db),
		store.NewJobStore(db),
		&compactMockVectorBackend{},
		NewStubEmbeddingProvider(3),
		llm.NewStubProvider("This is a compacted summary."),
	)
	err := w.Process(context.Background(), &store.ProcessingJob{
		ID:         "j1",
		DocumentID: "d1",
		Progress: map[string]any{
			"chunk_ids":    []any{"c1", "c2"},
			"requested_by": "user:test",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Mock helpers ---

type compactMockVectorBackend struct {
	storeErr  error
	deleteErr error
}

func (b *compactMockVectorBackend) EmbeddingDimension() int { return 3 }
func (b *compactMockVectorBackend) Store(_ context.Context, _ string, _ []float64, _ map[string]any) error {
	return b.storeErr
}
func (b *compactMockVectorBackend) Delete(_ context.Context, _ string) error { return b.deleteErr }
func (b *compactMockVectorBackend) Search(_ context.Context, _ []float64, _ vector.Filter, _ int) ([]vector.SearchResult, error) {
	return nil, nil
}
func (b *compactMockVectorBackend) StoreBatch(_ context.Context, _ []vector.StoreItem) error {
	return b.storeErr
}
func (b *compactMockVectorBackend) DeleteBatch(_ context.Context, _ []string) error {
	return b.deleteErr
}
func (b *compactMockVectorBackend) Ping(_ context.Context) error { return nil }

type compactFailingLLM struct{ err error }

func (p *compactFailingLLM) Complete(_ context.Context, _ []llm.Message) (*llm.Response, error) {
	return nil, p.err
}

type compactEmptyRows struct{}

func (r *compactEmptyRows) Close()                                       {}
func (r *compactEmptyRows) Err() error                                   { return nil }
func (r *compactEmptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *compactEmptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *compactEmptyRows) Next() bool                                   { return false }
func (r *compactEmptyRows) Scan(_ ...any) error                          { return nil }
func (r *compactEmptyRows) Values() ([]any, error)                       { return nil, nil }
func (r *compactEmptyRows) RawValues() [][]byte                          { return nil }
func (r *compactEmptyRows) Conn() *pgx.Conn                              { return nil }

type compactChunkData struct {
	id, content string
}

type compactChunkRows struct {
	chunks []compactChunkData
	idx    int
}

func (r *compactChunkRows) Close()                                       {}
func (r *compactChunkRows) Err() error                                   { return nil }
func (r *compactChunkRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *compactChunkRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *compactChunkRows) RawValues() [][]byte                          { return nil }
func (r *compactChunkRows) Conn() *pgx.Conn                              { return nil }
func (r *compactChunkRows) Values() ([]any, error)                       { return nil, nil }

func (r *compactChunkRows) Next() bool {
	if r.idx < len(r.chunks) {
		r.idx++
		return true
	}
	return false
}

func (r *compactChunkRows) Scan(dest ...any) error {
	row := r.chunks[r.idx-1]
	// id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
	*dest[0].(*string) = row.id
	*dest[1].(*string) = "d1"
	*dest[2].(*int) = r.idx
	*dest[3].(*string) = row.content
	*dest[4].(**string) = nil
	*dest[5].(*string) = "active"
	*dest[6].(**string) = nil
	*dest[7].(*[]byte) = []byte("{}")
	*dest[8].(*time.Time) = time.Now()
	return nil
}

type compactSeqRow struct{ seq int }

func (r *compactSeqRow) Scan(dest ...any) error {
	*dest[0].(*int) = r.seq
	return nil
}

type compactNewChunkRow struct{ id string }

func (r *compactNewChunkRow) Scan(dest ...any) error {
	*dest[0].(*string) = r.id
	*dest[1].(*string) = "d1"
	*dest[2].(*int) = 10
	*dest[3].(*string) = "summary content"
	*dest[4].(**string) = nil
	*dest[5].(*string) = "active"
	*dest[6].(**string) = nil
	*dest[7].(*[]byte) = []byte("{}")
	*dest[8].(*time.Time) = time.Now()
	return nil
}

type compactNewRecordRow struct{ id string }

func (r *compactNewRecordRow) Scan(dest ...any) error {
	*dest[0].(*string) = r.id
	*dest[1].(*string) = "sc1"
	*dest[2].(*[]string) = []string{"c1", "c2"}
	*dest[3].(*string) = "d1"
	*dest[4].(*string) = "user:test"
	*dest[5].(*time.Time) = time.Now()
	return nil
}
