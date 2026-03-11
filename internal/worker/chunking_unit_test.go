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
)

// mockTopicDBTX returns topic data for TopicStore.Get queries.
type mockTopicDBTX struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockTopicDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockTopicDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockTopicDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{err: errors.New("not configured")}
}

func (m *mockTopicDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

// mockTopicRow returns a valid Topic scan result.
type mockTopicRow struct {
	chunkingStrategy []byte
}

func (r *mockTopicRow) Scan(dest ...any) error {
	// id, slug, name, description, owner, created_at, updated_at,
	// llm_config_id, embedding_config_id, extraction_prompt_config_id, chunking_strategy, vector_backend_config_id
	if len(dest) >= 12 {
		*dest[0].(*string) = "topic-1"
		*dest[1].(*string) = "test-topic"
		*dest[2].(*string) = "Test Topic"
		*dest[3].(*string) = ""
		*dest[4].(*string) = "system:test"
		*dest[5].(*time.Time) = time.Now()
		*dest[6].(*time.Time) = time.Now()
		*dest[7].(**string) = nil
		*dest[8].(**string) = nil
		*dest[9].(**string) = nil
		*dest[10].(*[]byte) = r.chunkingStrategy
		*dest[11].(**string) = nil // vector_backend_config_id
	}
	return nil
}

// mockDocForChunking provides content queries and exec for chunking tests.
type mockDocForChunking struct {
	execCount   int
	execErr     error
	contentText string
	contentErr  error
	topicIDErr  error
}

func (m *mockDocForChunking) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	m.execCount++
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockDocForChunking) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockDocForChunking) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if strings.Contains(sql, "document_content") {
		if m.contentErr != nil {
			return &mockRow{err: m.contentErr}
		}
		return &mockDocContentRow{extractedText: m.contentText}
	}
	if strings.Contains(sql, "topic_id FROM documents") {
		if m.topicIDErr != nil {
			return &mockRow{err: m.topicIDErr}
		}
		return &mockTopicIDRow{topicID: "topic-1"}
	}
	return &mockRow{err: errors.New("unexpected query")}
}

func (m *mockDocForChunking) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

// mockTopicIDRow returns topic_id from a document.
type mockTopicIDRow struct {
	topicID string
}

func (r *mockTopicIDRow) Scan(dest ...any) error {
	if len(dest) >= 1 {
		*dest[0].(*string) = r.topicID
	}
	return nil
}

// mockDocContentRow returns extracted text for document_content queries.
type mockDocContentRow struct {
	extractedText string
}

func (r *mockDocContentRow) Scan(dest ...any) error {
	// Scans: document_id, raw_content, content_type, extracted_text, created_at, updated_at
	if len(dest) >= 6 {
		*dest[0].(*string) = "doc-1"
		*dest[1].(*[]byte) = nil
		*dest[2].(*string) = "text/plain"
		*dest[3].(*string) = r.extractedText
		// dest[4] and dest[5] are time.Time, zero values are fine
	}
	return nil
}

// mockChunkCreateDBTX supports both chunk creation (QueryRow) and other operations.
type mockChunkCreateDBTX struct {
	createErr   error
	createCount int
}

func (m *mockChunkCreateDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockChunkCreateDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockChunkCreateDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	m.createCount++
	if m.createErr != nil {
		return &mockRow{err: m.createErr}
	}
	return &mockChunkCreateRow{seq: m.createCount}
}

func (m *mockChunkCreateDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

type mockChunkCreateRow struct {
	seq int
}

func (r *mockChunkCreateRow) Scan(dest ...any) error {
	// id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
	if len(dest) >= 9 {
		*dest[0].(*string) = "chunk-1"
		*dest[1].(*string) = "doc-1"
		*dest[2].(*int) = r.seq
		*dest[3].(*string) = "content"
		*dest[4].(**string) = nil
		*dest[5].(*string) = "active"
		*dest[6].(**string) = nil
		*dest[7].(*[]byte) = []byte("{}")
		*dest[8].(*time.Time) = time.Now()
	}
	return nil
}

func TestChunkingWorker_Process_GetContentError(t *testing.T) {
	docDB := &mockDocForChunking{contentErr: pgx.ErrNoRows}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		nil, nil, nil, nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "getting document content") {
		t.Errorf("error should mention getting content: %v", err)
	}
}

func TestChunkingWorker_Process_EmptyExtractedText(t *testing.T) {
	docDB := &mockDocForChunking{contentText: ""}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		nil, nil, nil, nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for empty text")
	}
	if !strings.Contains(err.Error(), "extracted text is empty") {
		t.Errorf("error should mention empty text: %v", err)
	}
}

func TestChunkingWorker_Process_TopicIDLookupError(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text", topicIDErr: errors.New("not found")}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		nil, nil, nil, nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "looking up document topic") {
		t.Errorf("error should mention topic lookup: %v", err)
	}
}

func TestChunkingWorker_Process_TopicGetError(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: pgx.ErrNoRows}
		},
	}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		nil, nil,
		store.NewTopicStore(topicDB),
		nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "getting topic") {
		t.Errorf("error should mention getting topic: %v", err)
	}
}

func TestChunkingWorker_Process_ChunkCreateError(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{}
		},
	}
	chunkDB := &mockChunkCreateDBTX{createErr: errors.New("insert failed")}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		nil,
		store.NewTopicStore(topicDB),
		nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "creating chunk") {
		t.Errorf("error should mention creating chunk: %v", err)
	}
}

func TestChunkingWorker_Process_CreateEmbeddingJobError(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{}
		},
	}
	chunkDB := &mockChunkCreateDBTX{} // succeeds
	failingJobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("insert failed")}
		},
	}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewJobStore(failingJobDB),
		store.NewTopicStore(topicDB),
		nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "creating embedding job") {
		t.Errorf("error should mention creating embedding job: %v", err)
	}
}

func TestChunkingWorker_Process_WithChunkingStrategy(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text that is long enough to split into chunks for testing purposes yes"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{chunkingStrategy: []byte(`{"chunk_size":10,"chunk_overlap":2}`)}
		},
	}
	chunkDB := &mockChunkCreateDBTX{}
	jobDB := &mockJobDBTX{}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewJobStore(jobDB),
		store.NewTopicStore(topicDB),
		nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With chunk_size=10, the text should be split into multiple chunks.
	if chunkDB.createCount < 2 {
		t.Errorf("expected multiple chunk creations, got %d", chunkDB.createCount)
	}
}

func TestChunkingWorker_Process_Success(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "short"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{}
		},
	}
	chunkDB := &mockChunkCreateDBTX{}
	jobDB := &mockJobDBTX{}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewJobStore(jobDB),
		store.NewTopicStore(topicDB),
		nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunkDB.createCount != 1 {
		t.Errorf("expected 1 chunk creation, got %d", chunkDB.createCount)
	}
}

func TestChunkingWorker_Process_SemanticNoLLMProvider(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{chunkingStrategy: []byte(`{"type":"semantic"}`)}
		},
	}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		nil, nil,
		store.NewTopicStore(topicDB),
		nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for missing LLM provider")
	}
	if !strings.Contains(err.Error(), "LLM provider") {
		t.Errorf("error should mention LLM provider: %v", err)
	}
}

func TestChunkingWorker_Process_SemanticLLMError(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{chunkingStrategy: []byte(`{"type":"semantic"}`)}
		},
	}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		nil, nil,
		store.NewTopicStore(topicDB),
		&failingLLMProvider{},
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "semantic chunking") {
		t.Errorf("error should mention semantic chunking: %v", err)
	}
}

func TestChunkingWorker_Process_SemanticBadJSON(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{chunkingStrategy: []byte(`{"type":"semantic"}`)}
		},
	}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		nil, nil,
		store.NewTopicStore(topicDB),
		llm.NewStubProvider("not json"),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
	if !strings.Contains(err.Error(), "semantic chunking") {
		t.Errorf("error should mention semantic chunking: %v", err)
	}
}

func TestChunkingWorker_Process_SemanticEmptyChunks(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{chunkingStrategy: []byte(`{"type":"semantic"}`)}
		},
	}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		nil, nil,
		store.NewTopicStore(topicDB),
		llm.NewStubProvider(`[]`),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for empty chunks")
	}
	if !strings.Contains(err.Error(), "no chunks") {
		t.Errorf("error should mention no chunks: %v", err)
	}
}

func TestChunkingWorker_Process_SemanticSuccess(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{chunkingStrategy: []byte(`{"type":"semantic"}`)}
		},
	}
	chunkDB := &mockChunkCreateDBTX{}
	jobDB := &mockJobDBTX{}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewJobStore(jobDB),
		store.NewTopicStore(topicDB),
		llm.NewStubProvider(`["first section", "second section"]`),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunkDB.createCount != 2 {
		t.Errorf("expected 2 chunk creations, got %d", chunkDB.createCount)
	}
}

func TestChunkingWorker_Process_SemanticStripsCodeFences(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{chunkingStrategy: []byte(`{"type":"semantic"}`)}
		},
	}
	chunkDB := &mockChunkCreateDBTX{}
	jobDB := &mockJobDBTX{}
	// LLM wraps response in markdown code fences.
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewJobStore(jobDB),
		store.NewTopicStore(topicDB),
		llm.NewStubProvider("```json\n[\"chunk one\"]\n```"),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunkDB.createCount != 1 {
		t.Errorf("expected 1 chunk creation, got %d", chunkDB.createCount)
	}
}

func TestChunkingWorker_Process_SemanticSkipsEmptyChunks(t *testing.T) {
	docDB := &mockDocForChunking{contentText: "some text"}
	topicDB := &mockTopicDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockTopicRow{chunkingStrategy: []byte(`{"type":"semantic"}`)}
		},
	}
	chunkDB := &mockChunkCreateDBTX{}
	jobDB := &mockJobDBTX{}
	w := NewChunkingWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewJobStore(jobDB),
		store.NewTopicStore(topicDB),
		llm.NewStubProvider(`["real chunk", "", "  "]`),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunkDB.createCount != 1 {
		t.Errorf("expected 1 chunk creation (empty ones filtered), got %d", chunkDB.createCount)
	}
}
