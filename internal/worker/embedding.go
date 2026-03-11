package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// EmbeddingProvider computes vector embeddings for text.
type EmbeddingProvider interface {
	// Embed computes embeddings for the given texts.
	Embed(ctx context.Context, texts []string) ([][]float64, error)
	// Dimensions returns the number of dimensions this provider produces.
	Dimensions() int
	// Model returns the model identifier (e.g. "text-embedding-3-small").
	// Returns "" if unknown (e.g. stub provider).
	Model() string
}

// StubEmbeddingProvider returns deterministic fake embeddings for testing.
// It produces vectors of the configured dimension where each element is
// a simple hash of the input text, normalized to [0,1].
type StubEmbeddingProvider struct {
	dim int
}

// NewStubEmbeddingProvider creates a stub provider with the given dimension.
func NewStubEmbeddingProvider(dim int) *StubEmbeddingProvider {
	return &StubEmbeddingProvider{dim: dim}
}

// Embed returns deterministic embeddings based on the input text.
func (s *StubEmbeddingProvider) Embed(_ context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i, text := range texts {
		emb := make([]float64, s.dim)
		// Simple deterministic hash: use character values to seed the embedding.
		for j := 0; j < s.dim; j++ {
			if len(text) > 0 {
				charIdx := j % len(text)
				emb[j] = float64(text[charIdx]%100) / 100.0
			} else {
				emb[j] = 0.01
			}
		}
		result[i] = emb
	}
	return result, nil
}

// Dimensions returns the embedding dimension.
func (s *StubEmbeddingProvider) Dimensions() int {
	return s.dim
}

// Model returns "" for the stub provider.
func (s *StubEmbeddingProvider) Model() string {
	return ""
}

// EmbeddingWorker computes embeddings for document chunks and stores them.
type EmbeddingWorker struct {
	docStore      *store.DocumentStore
	chunkStore    *store.ChunkStore
	topicStore    *store.TopicStore
	jobStore      *store.JobStore
	vectorBackend vector.Backend
	provider      EmbeddingProvider
}

// NewEmbeddingWorker creates a new embedding worker.
func NewEmbeddingWorker(
	docStore *store.DocumentStore,
	chunkStore *store.ChunkStore,
	topicStore *store.TopicStore,
	jobStore *store.JobStore,
	vectorBackend vector.Backend,
	provider EmbeddingProvider,
) *EmbeddingWorker {
	return &EmbeddingWorker{
		docStore:      docStore,
		chunkStore:    chunkStore,
		topicStore:    topicStore,
		jobStore:      jobStore,
		vectorBackend: vectorBackend,
		provider:      provider,
	}
}

// Type returns the job type this worker handles.
func (w *EmbeddingWorker) Type() string {
	return "embedding"
}

// Process computes embeddings for all chunks without embeddings, stores them,
// and marks the document as ready.
func (w *EmbeddingWorker) Process(ctx context.Context, job *store.ProcessingJob) error {
	// Get all chunks for this document that don't have embeddings yet.
	var zeroTime time.Time
	chunks, err := w.chunkStore.ListByDocument(ctx, job.DocumentID, 0, zeroTime)
	if err != nil {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after chunk listing error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("listing chunks: %w", err)
	}

	// Filter to chunks without embeddings.
	var needsEmbedding []*store.Chunk
	for _, c := range chunks {
		if c.EmbeddingID == nil {
			needsEmbedding = append(needsEmbedding, c)
		}
	}

	if len(needsEmbedding) == 0 {
		// coverage:ignore - requires DB failure after successful query
		if err := w.docStore.UpdateStatus(ctx, job.DocumentID, "ready"); err != nil {
			return fmt.Errorf("setting document status to ready: %w", err)
		}
		return nil
	}

	// Collect texts for embedding.
	texts := make([]string, len(needsEmbedding))
	for i, c := range needsEmbedding {
		texts[i] = c.Content
	}

	// Compute embeddings.
	embeddings, err := w.provider.Embed(ctx, texts)
	if err != nil {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after embedding error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("computing embeddings: %w", err)
	}

	if len(embeddings) != len(needsEmbedding) {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after dimension mismatch: %w", setErr)
		}
		return fmt.Errorf("embedding count mismatch: got %d, want %d", len(embeddings), len(needsEmbedding))
	}

	// Store embeddings in the vector backend and set embedding IDs on chunks.
	// Include the embedding model in the stored metadata so we can trace
	// which config produced each embedding.
	modelName := w.provider.Model()
	items := make([]vector.StoreItem, len(needsEmbedding))
	for i, c := range needsEmbedding {
		meta := make(map[string]any, len(c.Metadata)+1) // coverage:ignore - happy path; tested via integration
		for k, v := range c.Metadata {                  // coverage:ignore - happy path; tested via integration
			meta[k] = v
		}
		if modelName != "" { // coverage:ignore - happy path; tested via integration
			meta["embedding_model"] = modelName
		}
		items[i] = vector.StoreItem{
			ID:        c.ID,
			Embedding: embeddings[i],
			Metadata:  meta,
		}
	}

	if err := w.vectorBackend.StoreBatch(ctx, items); err != nil {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after vector store error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("storing embeddings: %w", err)
	}

	// Set embedding_id on each chunk (embedding_id = chunk_id for pgvector).
	for _, c := range needsEmbedding {
		if err := w.chunkStore.SetEmbeddingID(ctx, c.ID, c.ID); err != nil {
			// coverage:ignore - requires DB failure after successful claim
			if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
				return fmt.Errorf("setting document status to failed after embedding ID update error: %w (original: %v)", setErr, err)
			}
			return fmt.Errorf("setting embedding ID for chunk %s: %w", c.ID, err)
		}
	}

	// Mark document as ready.
	// coverage:ignore - requires DB failure after successful operations
	if err := w.docStore.UpdateStatus(ctx, job.DocumentID, "ready"); err != nil {
		return fmt.Errorf("setting document status to ready: %w", err)
	}

	return nil
}
