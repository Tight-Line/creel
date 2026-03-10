package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// CompactionWorker merges multiple chunks into a single summary chunk using an LLM.
type CompactionWorker struct {
	chunkStore      *store.ChunkStore
	linkStore       *store.LinkStore
	compactionStore *store.CompactionStore
	docStore        *store.DocumentStore
	jobStore        *store.JobStore
	vectorBackend   vector.Backend
	embedProvider   EmbeddingProvider
	llmProvider     llm.Provider
}

// NewCompactionWorker creates a new compaction worker.
func NewCompactionWorker(
	chunkStore *store.ChunkStore,
	linkStore *store.LinkStore,
	compactionStore *store.CompactionStore,
	docStore *store.DocumentStore,
	jobStore *store.JobStore,
	vectorBackend vector.Backend,
	embedProvider EmbeddingProvider,
	llmProvider llm.Provider,
) *CompactionWorker {
	return &CompactionWorker{
		chunkStore:      chunkStore,
		linkStore:       linkStore,
		compactionStore: compactionStore,
		docStore:        docStore,
		jobStore:        jobStore,
		vectorBackend:   vectorBackend,
		embedProvider:   embedProvider,
		llmProvider:     llmProvider,
	}
}

// Type returns the job type this worker handles.
func (w *CompactionWorker) Type() string {
	return "compaction"
}

// Process compacts chunks for a document using an LLM to synthesize a summary.
func (w *CompactionWorker) Process(ctx context.Context, job *store.ProcessingJob) error {
	// Determine which chunks to compact.
	var chunkIDs []string
	if ids, ok := job.Progress["chunk_ids"]; ok {
		if idSlice, ok := ids.([]any); ok {
			for _, id := range idSlice {
				if s, ok := id.(string); ok {
					chunkIDs = append(chunkIDs, s)
				}
			}
		}
	}

	var chunks []*store.Chunk
	var err error

	if len(chunkIDs) > 0 {
		// Fetch specific chunks.
		chunkMap, err := w.chunkStore.GetMultiple(ctx, chunkIDs)
		// coverage:ignore - requires DB failure after successful claim
		if err != nil {
			return fmt.Errorf("fetching specified chunks: %w", err)
		}
		for _, id := range chunkIDs {
			if c, ok := chunkMap[id]; ok && c.Status == "active" {
				chunks = append(chunks, c)
			}
		}
	} else {
		// Fetch all active chunks for the document.
		var zeroTime time.Time
		chunks, err = w.chunkStore.ListByDocument(ctx, job.DocumentID, 0, zeroTime)
		// coverage:ignore - requires DB failure after successful claim
		if err != nil {
			return fmt.Errorf("listing document chunks: %w", err)
		}
	}

	if len(chunks) < 2 {
		// Nothing to compact.
		return nil
	}

	// Build the content for the LLM.
	var parts []string
	sourceIDs := make([]string, len(chunks))
	for i, c := range chunks {
		parts = append(parts, fmt.Sprintf("--- Chunk %d ---\n%s", i+1, c.Content))
		sourceIDs[i] = c.ID
	}
	combinedContent := strings.Join(parts, "\n\n")

	// Call the LLM to produce a summary.
	prompt := fmt.Sprintf(DefaultCompactionUserPrompt, len(chunks), combinedContent)
	resp, err := w.llmProvider.Complete(ctx, []llm.Message{
		{Role: "system", Content: DefaultCompactionSystemPrompt},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return fmt.Errorf("LLM compaction: %w", err)
	}

	summaryContent := strings.TrimSpace(resp.Content)
	if summaryContent == "" {
		return fmt.Errorf("LLM returned empty compaction summary")
	}

	// Get the next sequence number.
	nextSeq, err := w.chunkStore.NextSequence(ctx, job.DocumentID)
	// coverage:ignore - requires DB failure after successful LLM call
	if err != nil {
		return fmt.Errorf("getting next sequence: %w", err)
	}

	// Create the summary chunk.
	meta := map[string]any{"compaction_source_count": len(chunks)}
	summaryChunk, err := w.chunkStore.Create(ctx, job.DocumentID, summaryContent, nextSeq, meta)
	// coverage:ignore - requires DB failure after successful operations
	if err != nil {
		return fmt.Errorf("creating summary chunk: %w", err)
	}

	// Compute and store embedding for the summary chunk.
	embeddings, err := w.embedProvider.Embed(ctx, []string{summaryContent})
	// coverage:ignore - requires embedding provider failure
	if err != nil {
		return fmt.Errorf("computing summary embedding: %w", err)
	}
	if len(embeddings) > 0 {
		// coverage:ignore - requires vector backend failure
		if err := w.vectorBackend.Store(ctx, summaryChunk.ID, embeddings[0], meta); err != nil {
			return fmt.Errorf("storing summary embedding: %w", err)
		}
		// coverage:ignore - requires DB failure after vector store
		if err := w.chunkStore.SetEmbeddingID(ctx, summaryChunk.ID, summaryChunk.ID); err != nil {
			return fmt.Errorf("setting embedding ID: %w", err)
		}
	}

	// Transfer links from source chunks to the summary chunk.
	for _, id := range sourceIDs {
		// coverage:ignore - requires DB failure mid-transfer
		if _, err := w.linkStore.TransferLinks(ctx, id, summaryChunk.ID); err != nil {
			return fmt.Errorf("transferring links from chunk %s: %w", id, err)
		}
	}

	// coverage:ignore - requires vector backend failure
	if err := w.vectorBackend.DeleteBatch(ctx, sourceIDs); err != nil {
		return fmt.Errorf("deleting source embeddings: %w", err)
	}

	// coverage:ignore - requires DB failure after successful operations
	if err := w.chunkStore.MarkCompacted(ctx, sourceIDs, summaryChunk.ID); err != nil {
		return fmt.Errorf("marking chunks as compacted: %w", err)
	}

	// Determine who requested the compaction.
	createdBy := "system:compaction-worker"
	if requestedBy, ok := job.Progress["requested_by"]; ok {
		if s, ok := requestedBy.(string); ok {
			createdBy = s
		}
	}

	// coverage:ignore - requires DB failure after successful operations
	if _, err := w.compactionStore.Create(ctx, summaryChunk.ID, sourceIDs, job.DocumentID, createdBy); err != nil {
		return fmt.Errorf("creating compaction record: %w", err)
	}

	return nil
}
