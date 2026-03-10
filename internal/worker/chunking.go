package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tight-Line/creel/internal/llm"
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
	docStore    *store.DocumentStore
	chunkStore  *store.ChunkStore
	jobStore    *store.JobStore
	topicStore  *store.TopicStore
	llmProvider llm.Provider
}

// NewChunkingWorker creates a new chunking worker. The llmProvider is optional;
// it is only needed when a topic uses semantic chunking. Pass nil if semantic
// chunking is not required.
func NewChunkingWorker(docStore *store.DocumentStore, chunkStore *store.ChunkStore, jobStore *store.JobStore, topicStore *store.TopicStore, llmProvider llm.Provider) *ChunkingWorker {
	return &ChunkingWorker{
		docStore:    docStore,
		chunkStore:  chunkStore,
		jobStore:    jobStore,
		topicStore:  topicStore,
		llmProvider: llmProvider,
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

	strategy := topic.ChunkingStrategy
	useSemantic := strategy != nil && strategy.Type == "semantic"

	var chunks []string
	if useSemantic {
		if w.llmProvider == nil {
			if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
				return fmt.Errorf("setting document status to failed: %w", setErr)
			}
			return fmt.Errorf("semantic chunking requires an LLM provider but none is configured")
		}
		var err error
		chunks, err = w.splitSemantic(ctx, content.ExtractedText)
		if err != nil {
			if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
				return fmt.Errorf("setting document status to failed after semantic chunking error: %w (original: %v)", setErr, err)
			}
			return fmt.Errorf("semantic chunking: %w", err)
		}
	} else {
		chunkSize := DefaultChunkSize
		chunkOverlap := DefaultChunkOverlap
		if strategy != nil {
			if strategy.ChunkSize > 0 {
				chunkSize = strategy.ChunkSize
			}
			if strategy.ChunkOverlap > 0 {
				chunkOverlap = strategy.ChunkOverlap
			}
		}
		chunks = SplitText(content.ExtractedText, chunkSize, chunkOverlap)
	}

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

const semanticChunkingPrompt = `You are a document chunking assistant. Split the following text into semantically coherent sections. Each section should cover a single topic or idea.

Return a JSON array of strings, where each string is one chunk. Preserve the original text exactly; do not summarize or rephrase. Example response format:

["First section text...", "Second section text...", "Third section text..."]

Text to split:
`

// splitSemantic uses an LLM to identify natural split points in the text.
func (w *ChunkingWorker) splitSemantic(ctx context.Context, text string) ([]string, error) {
	resp, err := w.llmProvider.Complete(ctx, []llm.Message{
		{Role: "system", Content: semanticChunkingPrompt},
		{Role: "user", Content: text},
	})
	if err != nil {
		return nil, fmt.Errorf("calling LLM for semantic chunking: %w", err)
	}

	// Parse JSON array response.
	content := strings.TrimSpace(resp.Content)
	// Strip markdown code fences if present.
	if strings.HasPrefix(content, "```") {
		if idx := strings.Index(content[3:], "\n"); idx >= 0 {
			content = content[3+idx+1:]
		}
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var chunks []string
	if err := json.Unmarshal([]byte(content), &chunks); err != nil {
		return nil, fmt.Errorf("parsing LLM chunking response: %w", err)
	}

	// Filter empty chunks.
	var result []string
	for _, c := range chunks {
		trimmed := strings.TrimSpace(c)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("LLM returned no chunks")
	}

	return result, nil
}
