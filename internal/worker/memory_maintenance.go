package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/store"
)

// MemoryMaintenanceWorker processes memory_maintenance jobs by evaluating
// a candidate fact against existing memories and executing the appropriate action.
type MemoryMaintenanceWorker struct {
	memStore    *store.MemoryStore
	jobStore    *store.JobStore
	llmProvider llm.Provider
}

// NewMemoryMaintenanceWorker creates a new memory maintenance worker.
func NewMemoryMaintenanceWorker(
	memStore *store.MemoryStore,
	jobStore *store.JobStore,
	llmProvider llm.Provider,
) *MemoryMaintenanceWorker {
	return &MemoryMaintenanceWorker{
		memStore:    memStore,
		jobStore:    jobStore,
		llmProvider: llmProvider,
	}
}

// Type returns the job type this worker handles.
func (w *MemoryMaintenanceWorker) Type() string {
	return "memory_maintenance"
}

// maintenanceResponse is the expected JSON response from the LLM.
type maintenanceResponse struct {
	Action        string `json:"action"`
	MemoryID      string `json:"memory_id"`
	MergedContent string `json:"merged_content"`
}

// Process evaluates a candidate fact against existing memories and executes
// the appropriate action (ADD, UPDATE, DELETE, or NOOP).
func (w *MemoryMaintenanceWorker) Process(ctx context.Context, job *store.ProcessingJob) error {
	// Extract candidate fact and metadata from job progress.
	candidateFact, _ := job.Progress["candidate_fact"].(string)
	principal, _ := job.Progress["principal"].(string)
	scope, _ := job.Progress["scope"].(string)

	if candidateFact == "" || principal == "" {
		return fmt.Errorf("missing candidate_fact or principal in job progress")
	}
	if scope == "" {
		scope = "default"
	}

	// Fetch all existing memories in this scope for the principal.
	existingMemories, err := w.memStore.GetByScope(ctx, principal, scope)
	if err != nil {
		return fmt.Errorf("fetching existing memories: %w", err)
	}

	// Build the formatted existing memories for the prompt.
	var memoriesText string
	if len(existingMemories) == 0 {
		memoriesText = "(none)"
	} else {
		for _, m := range existingMemories {
			memoriesText += fmt.Sprintf("- ID: %s, Content: %s\n", m.ID, m.Content)
		}
	}

	// Call the LLM to decide what to do.
	messages := []llm.Message{
		{Role: "system", Content: DefaultMaintenanceSystemPrompt},
		{Role: "user", Content: fmt.Sprintf(DefaultMaintenanceUserPrompt, candidateFact, memoriesText)},
	}

	resp, err := w.llmProvider.Complete(ctx, messages)
	if err != nil {
		return fmt.Errorf("calling LLM for maintenance: %w", err)
	}

	var decision maintenanceResponse
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &decision); err != nil {
		return fmt.Errorf("parsing maintenance response: %w", err)
	}

	switch decision.Action {
	case "ADD":
		return w.handleAdd(ctx, candidateFact, principal, scope)
	case "UPDATE":
		return w.handleUpdate(ctx, decision.MemoryID, decision.MergedContent)
	case "DELETE":
		return w.handleDelete(ctx, decision.MemoryID)
	case "NOOP":
		return nil
	default:
		return fmt.Errorf("unknown maintenance action: %s", decision.Action)
	}
}

func (w *MemoryMaintenanceWorker) handleAdd(ctx context.Context, content, principal, scope string) error {
	_, err := w.memStore.Create(ctx, &store.Memory{
		Principal: principal,
		Scope:     scope,
		Content:   content,
	})
	if err != nil {
		return fmt.Errorf("creating memory: %w", err)
	}
	return nil
}

func (w *MemoryMaintenanceWorker) handleUpdate(ctx context.Context, memoryID, mergedContent string) error {
	if memoryID == "" || mergedContent == "" {
		return fmt.Errorf("UPDATE action requires memory_id and merged_content")
	}

	existing, err := w.memStore.Get(ctx, memoryID)
	if err != nil {
		return fmt.Errorf("getting memory for update: %w", err)
	}

	_, err = w.memStore.Update(ctx, memoryID, mergedContent, existing.Metadata)
	if err != nil {
		return fmt.Errorf("updating memory: %w", err)
	}

	return nil
}

func (w *MemoryMaintenanceWorker) handleDelete(ctx context.Context, memoryID string) error {
	if memoryID == "" {
		return fmt.Errorf("DELETE action requires memory_id")
	}
	if err := w.memStore.Invalidate(ctx, memoryID); err != nil {
		return fmt.Errorf("invalidating memory: %w", err)
	}
	return nil
}
