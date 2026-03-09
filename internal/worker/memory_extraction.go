package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/store"
)

// MemoryExtractionWorker processes memory_extraction jobs by extracting
// candidate facts from conversation chunks using an LLM.
type MemoryExtractionWorker struct {
	docStore    *store.DocumentStore
	chunkStore  *store.ChunkStore
	topicStore  *store.TopicStore
	jobStore    *store.JobStore
	llmProvider llm.Provider
}

// NewMemoryExtractionWorker creates a new memory extraction worker.
func NewMemoryExtractionWorker(
	docStore *store.DocumentStore,
	chunkStore *store.ChunkStore,
	topicStore *store.TopicStore,
	jobStore *store.JobStore,
	llmProvider llm.Provider,
) *MemoryExtractionWorker {
	return &MemoryExtractionWorker{
		docStore:    docStore,
		chunkStore:  chunkStore,
		topicStore:  topicStore,
		jobStore:    jobStore,
		llmProvider: llmProvider,
	}
}

// Type returns the job type this worker handles.
func (w *MemoryExtractionWorker) Type() string {
	return "memory_extraction"
}

// extractionResponse is the expected JSON response from the LLM.
type extractionResponse struct {
	Facts []string `json:"facts"`
}

// Process extracts candidate facts from a document's chunks and creates
// memory_maintenance jobs for each extracted fact.
func (w *MemoryExtractionWorker) Process(ctx context.Context, job *store.ProcessingJob) error {
	// Get the document to find its topic.
	doc, err := w.docStore.Get(ctx, job.DocumentID)
	if err != nil {
		return fmt.Errorf("getting document: %w", err)
	}

	// Get the topic to check memory_enabled.
	topic, err := w.topicStore.Get(ctx, doc.TopicID)
	if err != nil {
		return fmt.Errorf("getting topic: %w", err)
	}

	if !topic.MemoryEnabled {
		return nil // Skip; memory not enabled for this topic.
	}

	// Get recent chunks from the document.
	var zeroTime time.Time
	chunks, err := w.chunkStore.ListByDocument(ctx, job.DocumentID, 0, zeroTime)
	if err != nil {
		return fmt.Errorf("listing chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil
	}

	// Build the chunk content for the LLM prompt.
	var content string
	for _, c := range chunks {
		content += c.Content + "\n"
	}

	// Call the LLM to extract facts.
	messages := []llm.Message{
		{Role: "system", Content: DefaultExtractionSystemPrompt},
		{Role: "user", Content: fmt.Sprintf(DefaultExtractionUserPrompt, content)},
	}

	resp, err := w.llmProvider.Complete(ctx, messages)
	if err != nil {
		return fmt.Errorf("calling LLM for extraction: %w", err)
	}

	// Parse the LLM response.
	var extracted extractionResponse
	if err := json.Unmarshal([]byte(resp.Content), &extracted); err != nil {
		return fmt.Errorf("parsing extraction response: %w", err)
	}

	// Create a memory_maintenance job for each extracted fact.
	for _, fact := range extracted.Facts {
		if fact == "" {
			continue
		}
		progress := map[string]any{
			"candidate_fact": fact,
			"principal":      topic.Owner,
			"scope":          topic.Slug,
		}
		if _, err := w.jobStore.CreateWithProgress(ctx, job.DocumentID, "memory_maintenance", progress); err != nil {
			return fmt.Errorf("creating memory_maintenance job: %w", err)
		}
	}

	return nil
}
