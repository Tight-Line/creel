package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/store"
)

// MemoryMessagesWorker processes memory_messages jobs by extracting candidate
// facts from conversation messages and creating memory_maintenance jobs.
type MemoryMessagesWorker struct {
	jobStore    *store.JobStore
	llmProvider llm.Provider
}

// NewMemoryMessagesWorker creates a new memory messages worker.
func NewMemoryMessagesWorker(jobStore *store.JobStore, llmProvider llm.Provider) *MemoryMessagesWorker {
	return &MemoryMessagesWorker{
		jobStore:    jobStore,
		llmProvider: llmProvider,
	}
}

// Type returns the job type this worker handles.
func (w *MemoryMessagesWorker) Type() string {
	return "memory_messages"
}

// Process extracts candidate facts from conversation messages and creates
// memory_maintenance jobs for each.
func (w *MemoryMessagesWorker) Process(ctx context.Context, job *store.ProcessingJob) error {
	principal, _ := job.Progress["principal"].(string)
	scope, _ := job.Progress["scope"].(string)

	if principal == "" {
		return fmt.Errorf("missing principal in job progress")
	}
	if scope == "" {
		scope = "default"
	}

	// Extract messages from progress.
	rawMessages, ok := job.Progress["messages"]
	if !ok {
		return fmt.Errorf("missing messages in job progress")
	}

	// Format messages into a conversation transcript.
	messagesJSON, err := json.Marshal(rawMessages)
	if err != nil { // coverage:ignore - rawMessages is always JSON-serializable (came from map[string]any)
		return fmt.Errorf("marshaling messages: %w", err)
	}

	var messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		return fmt.Errorf("parsing messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	var transcript string
	for _, m := range messages {
		transcript += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	// Call LLM to extract facts.
	llmMessages := []llm.Message{
		{Role: "system", Content: DefaultMessagesExtractionSystemPrompt},
		{Role: "user", Content: fmt.Sprintf(DefaultMessagesExtractionUserPrompt, transcript)},
	}

	resp, err := w.llmProvider.Complete(ctx, llmMessages)
	if err != nil {
		return fmt.Errorf("calling LLM for fact extraction: %w", err)
	}

	var extracted struct {
		Facts []string `json:"facts"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &extracted); err != nil {
		return fmt.Errorf("parsing extraction response: %w", err)
	}

	// Create memory_maintenance jobs for each extracted fact.
	for _, fact := range extracted.Facts {
		if fact == "" {
			continue
		}
		progress := map[string]any{
			"candidate_fact": fact,
			"principal":      principal,
			"scope":          scope,
		}
		if _, err := w.jobStore.CreateDocless(ctx, "memory_maintenance", progress); err != nil {
			return fmt.Errorf("creating memory_maintenance job: %w", err)
		}
	}

	return nil
}
