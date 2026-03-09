package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// MemoryMaintenanceWorker processes memory_maintenance jobs by evaluating
// a candidate fact against existing memories and executing the appropriate action.
type MemoryMaintenanceWorker struct {
	memStore      *store.MemoryStore
	jobStore      *store.JobStore
	vectorBackend vector.Backend
	embedProvider EmbeddingProvider
	llmProvider   llm.Provider
}

// NewMemoryMaintenanceWorker creates a new memory maintenance worker.
func NewMemoryMaintenanceWorker(
	memStore *store.MemoryStore,
	jobStore *store.JobStore,
	vectorBackend vector.Backend,
	embedProvider EmbeddingProvider,
	llmProvider llm.Provider,
) *MemoryMaintenanceWorker {
	return &MemoryMaintenanceWorker{
		memStore:      memStore,
		jobStore:      jobStore,
		vectorBackend: vectorBackend,
		embedProvider: embedProvider,
		llmProvider:   llmProvider,
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

	// Embed the candidate fact.
	embeddings, err := w.embedProvider.Embed(ctx, []string{candidateFact})
	if err != nil {
		return fmt.Errorf("embedding candidate fact: %w", err)
	}
	if len(embeddings) == 0 {
		return fmt.Errorf("no embedding returned for candidate fact")
	}
	factEmbedding := embeddings[0]

	// Search existing memories in the same scope via vector backend (top 10).
	embeddingIDs, err := w.memStore.EmbeddingIDsByPrincipalScope(ctx, principal, scope)
	if err != nil {
		return fmt.Errorf("fetching memory embedding IDs: %w", err)
	}

	var existingMemories []*store.Memory
	if len(embeddingIDs) > 0 {
		filter := vector.Filter{ChunkIDs: embeddingIDs}
		searchResults, err := w.vectorBackend.Search(ctx, factEmbedding, filter, 10)
		if err != nil {
			return fmt.Errorf("searching existing memories: %w", err)
		}

		if len(searchResults) > 0 {
			resultEmbIDs := make([]string, len(searchResults))
			for i, sr := range searchResults {
				resultEmbIDs[i] = sr.ChunkID
			}
			memsByEmbID, err := w.memStore.GetByEmbeddingIDs(ctx, resultEmbIDs)
			if err != nil {
				return fmt.Errorf("fetching memories by embedding IDs: %w", err)
			}
			for _, sr := range searchResults {
				if m, ok := memsByEmbID[sr.ChunkID]; ok {
					existingMemories = append(existingMemories, m)
				}
			}
		}
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
	if err := json.Unmarshal([]byte(resp.Content), &decision); err != nil {
		return fmt.Errorf("parsing maintenance response: %w", err)
	}

	switch decision.Action {
	case "ADD":
		return w.handleAdd(ctx, candidateFact, principal, scope, factEmbedding)
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

func (w *MemoryMaintenanceWorker) handleAdd(ctx context.Context, content, principal, scope string, embedding []float64) error {
	mem, err := w.memStore.Create(ctx, &store.Memory{
		Principal: principal,
		Scope:     scope,
		Content:   content,
	})
	if err != nil {
		return fmt.Errorf("creating memory: %w", err)
	}

	// Store embedding in vector backend.
	embeddingID := fmt.Sprintf("mem_%s", mem.ID)
	if err := w.vectorBackend.Store(ctx, embeddingID, embedding, nil); err != nil {
		return fmt.Errorf("storing memory embedding: %w", err)
	}

	if err := w.memStore.SetEmbeddingID(ctx, mem.ID, embeddingID); err != nil {
		return fmt.Errorf("setting memory embedding ID: %w", err)
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

	// Re-embed the updated content.
	embeddings, err := w.embedProvider.Embed(ctx, []string{mergedContent})
	if err != nil {
		return fmt.Errorf("embedding updated memory: %w", err)
	}
	if len(embeddings) == 0 {
		return fmt.Errorf("no embedding returned for updated memory")
	}

	embeddingID := fmt.Sprintf("mem_%s", memoryID)
	if err := w.vectorBackend.Store(ctx, embeddingID, embeddings[0], nil); err != nil {
		return fmt.Errorf("storing updated memory embedding: %w", err)
	}

	if err := w.memStore.SetEmbeddingID(ctx, memoryID, embeddingID); err != nil {
		return fmt.Errorf("setting updated memory embedding ID: %w", err)
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
