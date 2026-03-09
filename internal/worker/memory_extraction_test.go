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

func TestMemoryExtractionWorker_Type(t *testing.T) {
	w := NewMemoryExtractionWorker(nil, nil, nil, nil, nil)
	if w.Type() != "memory_extraction" {
		t.Errorf("expected 'memory_extraction', got %q", w.Type())
	}
}

// mockDocGetDBTX supports document Get queries.
type mockDocGetDBTX struct {
	doc    *store.Document
	getErr error
}

func (m *mockDocGetDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockDocGetDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockDocGetDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if m.getErr != nil {
		return &mockRow{err: m.getErr}
	}
	return &mockDocRow{doc: m.doc}
}

func (m *mockDocGetDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

// mockDocRow scans a Document for Get queries.
type mockDocRow struct {
	doc *store.Document
}

func (r *mockDocRow) Scan(dest ...any) error {
	if len(dest) >= 12 {
		*dest[0].(*string) = r.doc.ID
		*dest[1].(*string) = r.doc.TopicID
		*dest[2].(*string) = r.doc.Slug
		*dest[3].(*string) = r.doc.Name
		*dest[4].(*string) = r.doc.DocType
		*dest[5].(*string) = r.doc.Status
		*dest[6].(*[]byte) = []byte("{}")
		*dest[7].(*time.Time) = r.doc.CreatedAt
		*dest[8].(*time.Time) = r.doc.UpdatedAt
		*dest[9].(**string) = nil
		*dest[10].(**string) = nil
		*dest[11].(**time.Time) = nil
	}
	return nil
}

// mockMemExTopicDBTX supports topic Get queries with memory_enabled.
type mockMemExTopicDBTX struct {
	topic  *store.Topic
	getErr error
}

func (m *mockMemExTopicDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockMemExTopicDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockMemExTopicDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if m.getErr != nil {
		return &mockRow{err: m.getErr}
	}
	return &mockMemExTopicRow{topic: m.topic}
}

func (m *mockMemExTopicDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

type mockMemExTopicRow struct {
	topic *store.Topic
}

func (r *mockMemExTopicRow) Scan(dest ...any) error {
	if len(dest) >= 12 {
		*dest[0].(*string) = r.topic.ID
		*dest[1].(*string) = r.topic.Slug
		*dest[2].(*string) = r.topic.Name
		*dest[3].(*string) = r.topic.Description
		*dest[4].(*string) = r.topic.Owner
		*dest[5].(*time.Time) = r.topic.CreatedAt
		*dest[6].(*time.Time) = r.topic.UpdatedAt
		*dest[7].(**string) = nil
		*dest[8].(**string) = nil
		*dest[9].(**string) = nil
		*dest[10].(*[]byte) = nil
		*dest[11].(*bool) = r.topic.MemoryEnabled
	}
	return nil
}

func TestMemoryExtractionWorker_GetDocumentError(t *testing.T) {
	docDB := &mockDocGetDBTX{getErr: errors.New("not found")}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		nil, nil, nil, nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "getting document") {
		t.Fatalf("expected 'getting document' error, got: %v", err)
	}
}

func TestMemoryExtractionWorker_GetTopicError(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topicDB := &mockMemExTopicDBTX{getErr: errors.New("not found")}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		nil,
		store.NewTopicStore(topicDB),
		nil, nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "getting topic") {
		t.Fatalf("expected 'getting topic' error, got: %v", err)
	}
}

func TestMemoryExtractionWorker_SkipWhenNotMemoryEnabled(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: false}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		nil,
		store.NewTopicStore(topicDB),
		nil, nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryExtractionWorker_ListChunksError(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: true}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	chunkDB := &mockChunkDBTX{queryErr: errors.New("list failed")}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewTopicStore(topicDB),
		nil, nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "listing chunks") {
		t.Fatalf("expected 'listing chunks' error, got: %v", err)
	}
}

func TestMemoryExtractionWorker_NoChunks(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: true}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	chunkDB := &mockChunkDBTX{chunks: nil}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewTopicStore(topicDB),
		nil, nil,
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryExtractionWorker_LLMError(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: true}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	chunk := &store.Chunk{ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "hello", Status: "active"}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewTopicStore(topicDB),
		nil,
		&failingLLMProvider{},
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "calling LLM for extraction") {
		t.Fatalf("expected LLM error, got: %v", err)
	}
}

func TestMemoryExtractionWorker_BadJSON(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: true}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	chunk := &store.Chunk{ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "hello", Status: "active"}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewTopicStore(topicDB),
		nil,
		llm.NewStubProvider("not json"),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "parsing extraction response") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestMemoryExtractionWorker_EmptyFacts(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: true}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	chunk := &store.Chunk{ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "hello", Status: "active"}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewTopicStore(topicDB),
		nil,
		llm.NewStubProvider(`{"facts": []}`),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryExtractionWorker_SkipsEmptyFacts(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: true}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	chunk := &store.Chunk{ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "hello", Status: "active"}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}
	// Response includes an empty fact that should be skipped.
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewTopicStore(topicDB),
		store.NewJobStore(&mockJobWithProgressDBTX{}),
		llm.NewStubProvider(`{"facts": ["", "user likes cats"]}`),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryExtractionWorker_CreateJobError(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: true}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	chunk := &store.Chunk{ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "hello", Status: "active"}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}
	failJobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("insert failed")}
		},
	}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewTopicStore(topicDB),
		store.NewJobStore(failJobDB),
		llm.NewStubProvider(`{"facts": ["user likes cats"]}`),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "creating memory_maintenance job") {
		t.Fatalf("expected job creation error, got: %v", err)
	}
}

func TestMemoryExtractionWorker_Success(t *testing.T) {
	doc := &store.Document{ID: "doc-1", TopicID: "topic-1", Slug: "test", Name: "Test", DocType: "note", Status: "ready"}
	docDB := &mockDocGetDBTX{doc: doc}
	topic := &store.Topic{ID: "topic-1", Slug: "test", Owner: "user1", MemoryEnabled: true}
	topicDB := &mockMemExTopicDBTX{topic: topic}
	chunk := &store.Chunk{ID: "c1", DocumentID: "doc-1", Sequence: 1, Content: "hello", Status: "active"}
	chunkDB := &mockChunkDBTX{chunks: []*store.Chunk{chunk}}
	w := NewMemoryExtractionWorker(
		store.NewDocumentStore(docDB),
		store.NewChunkStore(chunkDB),
		store.NewTopicStore(topicDB),
		store.NewJobStore(&mockJobWithProgressDBTX{}),
		llm.NewStubProvider(`{"facts": ["user likes cats", "user has a dog"]}`),
	)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// failingLLMProvider always returns an error.
type failingLLMProvider struct{}

func (p *failingLLMProvider) Complete(_ context.Context, _ []llm.Message) (*llm.Response, error) {
	return nil, errors.New("LLM error")
}

// mockJobWithProgressDBTX supports CreateWithProgress.
type mockJobWithProgressDBTX struct{}

func (m *mockJobWithProgressDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 1"), nil
}

func (m *mockJobWithProgressDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockJobWithProgressDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockJobRow{}
}

func (m *mockJobWithProgressDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}
