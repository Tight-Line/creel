package worker

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector/pgvector"
)

func TestChunkingWorker_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	jobStore := store.NewJobStore(pool)

	topic, err := topicStore.Create(ctx, fmt.Sprintf("chunking-test-%d", time.Now().UnixNano()),
		"Chunking Test", "", "system:test", nil, nil, nil, false, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "chunk-doc", "Chunk Doc", "reference", "processing", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	// Save extracted text.
	longText := strings.Repeat("Hello world. ", 300) // ~3900 chars
	if err := docStore.SaveContent(ctx, doc.ID, nil, "text/plain"); err != nil {
		t.Fatalf("saving content: %v", err)
	}
	if err := docStore.SaveExtractedText(ctx, doc.ID, longText); err != nil {
		t.Fatalf("saving extracted text: %v", err)
	}

	// Create chunking job.
	job, err := jobStore.Create(ctx, doc.ID, "chunking")
	if err != nil {
		t.Fatalf("creating job: %v", err)
	}

	w := NewChunkingWorker(docStore, chunkStore, jobStore, topicStore, nil)
	if err := w.Process(ctx, job); err != nil {
		t.Fatalf("processing: %v", err)
	}

	// Verify chunks were created.
	var zeroTime time.Time
	chunks, err := chunkStore.ListByDocument(ctx, doc.ID, 0, zeroTime)
	if err != nil {
		t.Fatalf("listing chunks: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify sequence numbers are sequential.
	for i, c := range chunks {
		if c.Sequence != i+1 {
			t.Errorf("chunk[%d].Sequence = %d, want %d", i, c.Sequence, i+1)
		}
	}

	// Verify an embedding job was created.
	jobs, err := jobStore.List(ctx, store.ListJobsOptions{DocumentID: doc.ID, Status: "queued"})
	if err != nil {
		t.Fatalf("listing jobs: %v", err)
	}
	found := false
	for _, j := range jobs {
		if j.JobType == "embedding" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a queued embedding job to be created")
	}
}

func TestChunkingWorker_ShortText_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	jobStore := store.NewJobStore(pool)

	topic, err := topicStore.Create(ctx, fmt.Sprintf("chunking-short-%d", time.Now().UnixNano()),
		"Short Text Test", "", "system:test", nil, nil, nil, false, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "short-doc", "Short Doc", "reference", "processing", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	shortText := "This is a short document."
	if err := docStore.SaveContent(ctx, doc.ID, nil, "text/plain"); err != nil {
		t.Fatalf("saving content: %v", err)
	}
	if err := docStore.SaveExtractedText(ctx, doc.ID, shortText); err != nil {
		t.Fatalf("saving extracted text: %v", err)
	}

	job, err := jobStore.Create(ctx, doc.ID, "chunking")
	if err != nil {
		t.Fatalf("creating job: %v", err)
	}

	w := NewChunkingWorker(docStore, chunkStore, jobStore, topicStore, nil)
	if err := w.Process(ctx, job); err != nil {
		t.Fatalf("processing: %v", err)
	}

	var zeroTime time.Time
	chunks, err := chunkStore.ListByDocument(ctx, doc.ID, 0, zeroTime)
	if err != nil {
		t.Fatalf("listing chunks: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content != shortText {
		t.Errorf("chunk content = %q, want %q", chunks[0].Content, shortText)
	}
}

func TestChunkingWorker_EmptyText_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	jobStore := store.NewJobStore(pool)

	topic, err := topicStore.Create(ctx, fmt.Sprintf("chunking-empty-%d", time.Now().UnixNano()),
		"Empty Text Test", "", "system:test", nil, nil, nil, false, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "empty-doc", "Empty Doc", "reference", "processing", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	if err := docStore.SaveContent(ctx, doc.ID, nil, "text/plain"); err != nil {
		t.Fatalf("saving content: %v", err)
	}
	// Don't save extracted text (leave empty default).

	job, err := jobStore.Create(ctx, doc.ID, "chunking")
	if err != nil {
		t.Fatalf("creating job: %v", err)
	}

	w := NewChunkingWorker(docStore, chunkStore, jobStore, topicStore, nil)
	err = w.Process(ctx, job)
	if err == nil {
		t.Fatal("expected error for empty extracted text")
	}

	// Document should be marked as failed.
	updatedDoc, err := docStore.Get(ctx, doc.ID)
	if err != nil {
		t.Fatalf("getting document: %v", err)
	}
	if updatedDoc.Status != "failed" {
		t.Errorf("Status = %q, want failed", updatedDoc.Status)
	}
}

func TestEmbeddingWorker_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	jobStore := store.NewJobStore(pool)
	vectorBackend := pgvector.New(pool)

	topic, err := topicStore.Create(ctx, fmt.Sprintf("embedding-test-%d", time.Now().UnixNano()),
		"Embedding Test", "", "system:test", nil, nil, nil, false, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "embed-doc", "Embed Doc", "reference", "processing", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	// Create chunks.
	chunk1, err := chunkStore.Create(ctx, doc.ID, "First chunk content", 1, nil)
	if err != nil {
		t.Fatalf("creating chunk 1: %v", err)
	}
	chunk2, err := chunkStore.Create(ctx, doc.ID, "Second chunk content", 2, nil)
	if err != nil {
		t.Fatalf("creating chunk 2: %v", err)
	}

	// Create embedding job.
	job, err := jobStore.Create(ctx, doc.ID, "embedding")
	if err != nil {
		t.Fatalf("creating job: %v", err)
	}

	provider := NewStubEmbeddingProvider(vectorBackend.EmbeddingDimension())
	w := NewEmbeddingWorker(docStore, chunkStore, topicStore, jobStore, vectorBackend, provider)
	if err := w.Process(ctx, job); err != nil {
		t.Fatalf("processing: %v", err)
	}

	// Verify document is ready.
	updatedDoc, err := docStore.Get(ctx, doc.ID)
	if err != nil {
		t.Fatalf("getting document: %v", err)
	}
	if updatedDoc.Status != "ready" {
		t.Errorf("Status = %q, want ready", updatedDoc.Status)
	}

	// Verify chunks have embedding IDs.
	c1, err := chunkStore.Get(ctx, chunk1.ID)
	if err != nil {
		t.Fatalf("getting chunk 1: %v", err)
	}
	if c1.EmbeddingID == nil {
		t.Error("chunk 1 should have embedding_id set")
	}

	c2, err := chunkStore.Get(ctx, chunk2.ID)
	if err != nil {
		t.Fatalf("getting chunk 2: %v", err)
	}
	if c2.EmbeddingID == nil {
		t.Error("chunk 2 should have embedding_id set")
	}
}

func TestEmbeddingWorker_NoChunksNeedEmbedding_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	jobStore := store.NewJobStore(pool)
	vectorBackend := pgvector.New(pool)

	topic, err := topicStore.Create(ctx, fmt.Sprintf("embedding-noop-%d", time.Now().UnixNano()),
		"No Embedding Test", "", "system:test", nil, nil, nil, false, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "noop-doc", "NoOp Doc", "reference", "processing", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	// Create a chunk that already has an embedding.
	chunk, err := chunkStore.Create(ctx, doc.ID, "Already embedded", 1, nil)
	if err != nil {
		t.Fatalf("creating chunk: %v", err)
	}
	if err := chunkStore.SetEmbeddingID(ctx, chunk.ID, chunk.ID); err != nil {
		t.Fatalf("setting embedding ID: %v", err)
	}

	job, err := jobStore.Create(ctx, doc.ID, "embedding")
	if err != nil {
		t.Fatalf("creating job: %v", err)
	}

	provider := NewStubEmbeddingProvider(vectorBackend.EmbeddingDimension())
	w := NewEmbeddingWorker(docStore, chunkStore, topicStore, jobStore, vectorBackend, provider)
	if err := w.Process(ctx, job); err != nil {
		t.Fatalf("processing: %v", err)
	}

	// Document should be ready.
	updatedDoc, err := docStore.Get(ctx, doc.ID)
	if err != nil {
		t.Fatalf("getting document: %v", err)
	}
	if updatedDoc.Status != "ready" {
		t.Errorf("Status = %q, want ready", updatedDoc.Status)
	}
}

func TestPipeline_EndToEnd_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	jobStore := store.NewJobStore(pool)
	vectorBackend := pgvector.New(pool)

	topic, err := topicStore.Create(ctx, fmt.Sprintf("pipeline-e2e-%d", time.Now().UnixNano()),
		"Pipeline E2E Test", "", "system:test", nil, nil, nil, false, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	// Create a pending document with content.
	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "e2e-doc", "E2E Doc", "reference", "pending", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	plainText := "This is a test document for the full pipeline. It has enough text to be meaningful."
	if err := docStore.SaveContent(ctx, doc.ID, []byte(plainText), "text/plain"); err != nil {
		t.Fatalf("saving content: %v", err)
	}

	// Step 1: Extraction.
	extractionJob, err := jobStore.Create(ctx, doc.ID, "extraction")
	if err != nil {
		t.Fatalf("creating extraction job: %v", err)
	}

	extractionWorker := NewExtractionWorker(docStore, jobStore)
	if err := extractionWorker.Process(ctx, extractionJob); err != nil {
		t.Fatalf("extraction: %v", err)
	}

	// Verify chunking job was created.
	jobs, err := jobStore.List(ctx, store.ListJobsOptions{DocumentID: doc.ID, Status: "queued"})
	if err != nil {
		t.Fatalf("listing jobs: %v", err)
	}
	var chunkingJob *store.ProcessingJob
	for _, j := range jobs {
		if j.JobType == "chunking" {
			chunkingJob = j
			break
		}
	}
	if chunkingJob == nil {
		t.Fatal("expected a chunking job to be created")
	}

	// Step 2: Chunking.
	chunkingWorker := NewChunkingWorker(docStore, chunkStore, jobStore, topicStore, nil)
	if err := chunkingWorker.Process(ctx, chunkingJob); err != nil {
		t.Fatalf("chunking: %v", err)
	}

	// Verify chunks created.
	var zeroTime time.Time
	chunks, err := chunkStore.ListByDocument(ctx, doc.ID, 0, zeroTime)
	if err != nil {
		t.Fatalf("listing chunks: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	// Verify embedding job was created.
	jobs, err = jobStore.List(ctx, store.ListJobsOptions{DocumentID: doc.ID, Status: "queued"})
	if err != nil {
		t.Fatalf("listing jobs: %v", err)
	}
	var embeddingJob *store.ProcessingJob
	for _, j := range jobs {
		if j.JobType == "embedding" {
			embeddingJob = j
			break
		}
	}
	if embeddingJob == nil {
		t.Fatal("expected an embedding job to be created")
	}

	// Step 3: Embedding.
	provider := NewStubEmbeddingProvider(vectorBackend.EmbeddingDimension())
	embeddingWorker := NewEmbeddingWorker(docStore, chunkStore, topicStore, jobStore, vectorBackend, provider)
	if err := embeddingWorker.Process(ctx, embeddingJob); err != nil {
		t.Fatalf("embedding: %v", err)
	}

	// Verify document is ready.
	finalDoc, err := docStore.Get(ctx, doc.ID)
	if err != nil {
		t.Fatalf("getting document: %v", err)
	}
	if finalDoc.Status != "ready" {
		t.Errorf("Status = %q, want ready", finalDoc.Status)
	}

	// Verify all chunks have embeddings.
	chunks, err = chunkStore.ListByDocument(ctx, doc.ID, 0, zeroTime)
	if err != nil {
		t.Fatalf("listing chunks: %v", err)
	}
	for _, c := range chunks {
		if c.EmbeddingID == nil {
			t.Errorf("chunk %s (seq %d) should have embedding_id set", c.ID, c.Sequence)
		}
	}
}
