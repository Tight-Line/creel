package worker

import (
	"context"
	"fmt"

	"github.com/Tight-Line/creel/internal/store"
)

const (
	// DefaultChunkSize is the default chunk size in characters.
	DefaultChunkSize = 2048
	// DefaultChunkOverlap is the default overlap in characters.
	DefaultChunkOverlap = 200
)

// ChunkingWorker splits extracted text into chunks and creates the next pipeline job.
type ChunkingWorker struct {
	docStore   *store.DocumentStore
	chunkStore *store.ChunkStore
	jobStore   *store.JobStore
	topicStore *store.TopicStore
}

// NewChunkingWorker creates a new chunking worker.
func NewChunkingWorker(docStore *store.DocumentStore, chunkStore *store.ChunkStore, jobStore *store.JobStore, topicStore *store.TopicStore) *ChunkingWorker {
	return &ChunkingWorker{
		docStore:   docStore,
		chunkStore: chunkStore,
		jobStore:   jobStore,
		topicStore: topicStore,
	}
}

// Type returns the job type this worker handles.
func (w *ChunkingWorker) Type() string {
	return "chunking"
}

// Process splits the document's extracted text into chunks and enqueues an embedding job.
func (w *ChunkingWorker) Process(ctx context.Context, job *store.ProcessingJob) error {
	// Get extracted text.
	content, err := w.docStore.GetContent(ctx, job.DocumentID)
	if err != nil {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after content error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("getting document content: %w", err)
	}

	if content.ExtractedText == "" {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed: %w", setErr)
		}
		return fmt.Errorf("extracted text is empty for document %s", job.DocumentID)
	}

	// Get topic to read chunking strategy.
	topicID, err := w.docStore.TopicIDForDocument(ctx, job.DocumentID)
	if err != nil {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after topic lookup error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("looking up document topic: %w", err)
	}

	topic, err := w.topicStore.Get(ctx, topicID)
	if err != nil {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after topic get error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("getting topic: %w", err)
	}

	chunkSize := DefaultChunkSize
	chunkOverlap := DefaultChunkOverlap
	if topic.ChunkingStrategy != nil {
		if topic.ChunkingStrategy.ChunkSize > 0 {
			chunkSize = topic.ChunkingStrategy.ChunkSize
		}
		if topic.ChunkingStrategy.ChunkOverlap > 0 {
			chunkOverlap = topic.ChunkingStrategy.ChunkOverlap
		}
	}

	// Split text into chunks.
	chunks := SplitText(content.ExtractedText, chunkSize, chunkOverlap)

	// Create chunks in the store.
	for i, text := range chunks {
		if _, err := w.chunkStore.Create(ctx, job.DocumentID, text, i+1, nil); err != nil {
			// coverage:ignore - requires DB failure after successful claim
			if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
				return fmt.Errorf("setting document status to failed after chunk creation error: %w (original: %v)", setErr, err)
			}
			return fmt.Errorf("creating chunk %d: %w", i+1, err)
		}
	}

	// Create the next pipeline job: embedding.
	if _, err := w.jobStore.Create(ctx, job.DocumentID, "embedding"); err != nil {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after embedding job creation error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("creating embedding job: %w", err)
	}

	return nil
}

// SplitText splits text into chunks with overlap. If the text is shorter than
// or equal to chunkSize, it returns a single chunk. The overlap ensures context
// continuity between adjacent chunks.
func SplitText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}

	if len(text) <= chunkSize {
		if text == "" {
			return nil
		}
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(text) {
		end := start + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[start:end])
		if end == len(text) {
			break
		}
		start = end - overlap
	}
	return chunks
}
