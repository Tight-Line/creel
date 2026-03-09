package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// mockChunkDBTX is a mock DBTX that returns chunks for ListByDocument queries.
type mockChunkDBTX struct {
	chunks    []*store.Chunk
	queryErr  error
	execErr   error
	execCount int
}

func (m *mockChunkDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	m.execCount++
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockChunkDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return &mockChunkRows{chunks: m.chunks}, nil
}

func (m *mockChunkDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{err: errors.New("not configured")}
}

func (m *mockChunkDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

// mockChunkRows implements pgx.Rows for chunk listing.
type mockChunkRows struct {
	chunks []*store.Chunk
	idx    int
}

func (r *mockChunkRows) Close()                                       {}
func (r *mockChunkRows) Err() error                                   { return nil }
func (r *mockChunkRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockChunkRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockChunkRows) RawValues() [][]byte                          { return nil }
func (r *mockChunkRows) Conn() *pgx.Conn                              { return nil }
func (r *mockChunkRows) Values() ([]any, error)                       { return nil, nil }

func (r *mockChunkRows) Next() bool {
	if r.idx < len(r.chunks) {
		r.idx++
		return true
	}
	return false
}

func (r *mockChunkRows) Scan(dest ...any) error {
	c := r.chunks[r.idx-1]
	// id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
	*dest[0].(*string) = c.ID
	*dest[1].(*string) = c.DocumentID
	*dest[2].(*int) = c.Sequence
	*dest[3].(*string) = c.Content
	*dest[4].(**string) = c.EmbeddingID
	*dest[5].(*string) = c.Status
	*dest[6].(**string) = c.CompactedBy
	*dest[7].(*[]byte) = []byte("{}")
	*dest[8].(*time.Time) = c.CreatedAt
	return nil
}

// mockVectorBackend is a mock vector.Backend for testing.
type mockVectorBackend struct {
	storeErr error
	dim      int
}

func (m *mockVectorBackend) EmbeddingDimension() int { return m.dim }
func (m *mockVectorBackend) Store(_ context.Context, _ string, _ []float64, _ map[string]any) error {
	return m.storeErr
}
func (m *mockVectorBackend) Delete(_ context.Context, _ string) error { return nil }
func (m *mockVectorBackend) Search(_ context.Context, _ []float64, _ vector.Filter, _ int) ([]vector.SearchResult, error) {
	return nil, nil
}
func (m *mockVectorBackend) StoreBatch(_ context.Context, _ []vector.StoreItem) error {
	return m.storeErr
}
func (m *mockVectorBackend) DeleteBatch(_ context.Context, _ []string) error { return nil }
func (m *mockVectorBackend) Ping(_ context.Context) error                    { return nil }

func TestEmbeddingWorker_Process_ListChunksError(t *testing.T) {
	docDB := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	chunkDB := &mockChunkDBTX{queryErr: errors.New("list failed")}

	w := NewEmbeddingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		nil, nil,
		&mockVectorBackend{dim: 4},
		NewStubEmbeddingProvider(4),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "listing chunks") {
		t.Errorf("error should mention listing chunks: %v", err)
	}
}

func TestEmbeddingWorker_Process_ProviderError(t *testing.T) {
	docDB := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	chunk := &store.Chunk{
		ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "test", Status: "active",
	}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}

	w := NewEmbeddingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		nil, nil,
		&mockVectorBackend{dim: 4},
		&failingEmbeddingProvider{dim: 4},
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "computing embeddings") {
		t.Errorf("error should mention computing embeddings: %v", err)
	}
}

func TestEmbeddingWorker_Process_CountMismatch(t *testing.T) {
	docDB := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	chunks := []*store.Chunk{
		{ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "test1", Status: "active"},
		{ID: "c2", DocumentID: "doc-1", Sequence: 2, Content: "test2", Status: "active"},
	}
	chunkDB := &mockChunkDBTX{chunks: chunks}

	w := NewEmbeddingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		nil, nil,
		&mockVectorBackend{dim: 4},
		&mismatchEmbeddingProvider{dim: 4},
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "embedding count mismatch") {
		t.Errorf("error should mention count mismatch: %v", err)
	}
}

func TestEmbeddingWorker_Process_VectorStoreError(t *testing.T) {
	docDB := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	chunk := &store.Chunk{
		ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "test", Status: "active",
	}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}

	w := NewEmbeddingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		nil, nil,
		&mockVectorBackend{dim: 4, storeErr: errors.New("vector store failed")},
		NewStubEmbeddingProvider(4),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "storing embeddings") {
		t.Errorf("error should mention storing embeddings: %v", err)
	}
}

func TestEmbeddingWorker_Process_NoChunksNeedEmbedding(t *testing.T) {
	docDB := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	embID := "emb-1"
	chunk := &store.Chunk{
		ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "test", Status: "active",
		EmbeddingID: &embID, // Already has embedding.
	}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}

	w := NewEmbeddingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		nil, nil,
		&mockVectorBackend{dim: 4},
		NewStubEmbeddingProvider(4),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmbeddingWorker_Process_Success(t *testing.T) {
	docDB := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	chunk := &store.Chunk{
		ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "test", Status: "active",
	}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}

	w := NewEmbeddingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		nil, nil,
		&mockVectorBackend{dim: 4},
		NewStubEmbeddingProvider(4),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmbeddingWorker_Process_SetEmbeddingIDError(t *testing.T) {
	docDB := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	chunk := &store.Chunk{
		ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "test", Status: "active",
	}
	chunkDB := &mockChunkDBTX{
		chunks:  []*store.Chunk{chunk},
		execErr: errors.New("set embedding id failed"),
	}

	w := NewEmbeddingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		nil, nil,
		&mockVectorBackend{dim: 4},
		NewStubEmbeddingProvider(4),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "setting embedding ID") {
		t.Errorf("error should mention setting embedding ID: %v", err)
	}
}
