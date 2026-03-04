// Package retrieval implements RAG search logic for Creel.
package retrieval

import (
	"context"
	"fmt"

	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// Result represents a single search result with context.
type Result struct {
	Chunk   *store.Chunk
	TopicID string
	Score   float64
}

// Searcher performs ACL-filtered vector similarity search.
type Searcher struct {
	chunkStore *store.ChunkStore
	authorizer auth.Authorizer
	backend    vector.Backend
}

// NewSearcher creates a new Searcher.
func NewSearcher(chunkStore *store.ChunkStore, authorizer auth.Authorizer, backend vector.Backend) *Searcher {
	return &Searcher{
		chunkStore: chunkStore,
		authorizer: authorizer,
		backend:    backend,
	}
}

// Search performs an ACL-filtered similarity search.
// If topicIDs is empty, it searches all topics accessible to the principal.
func (s *Searcher) Search(ctx context.Context, principal *auth.Principal, topicIDs []string, queryEmbedding []float64, topK int) ([]Result, error) {
	// Resolve accessible topics.
	accessible, err := s.resolveTopics(ctx, principal, topicIDs)
	if err != nil {
		return nil, err
	}
	if len(accessible) == 0 {
		return nil, nil
	}

	// Get chunk IDs in accessible topics.
	chunkIDs, err := s.chunkStore.ChunkIDsByTopics(ctx, accessible)
	if err != nil {
		return nil, fmt.Errorf("fetching chunk IDs: %w", err)
	}
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	// Search vector backend with ACL filter.
	filter := vector.Filter{ChunkIDs: chunkIDs}
	searchResults, err := s.backend.Search(ctx, queryEmbedding, filter, topK)
	if err != nil {
		return nil, fmt.Errorf("searching vector backend: %w", err)
	}

	// Hydrate results with chunk data.
	var results []Result
	for _, sr := range searchResults {
		chunk, err := s.chunkStore.Get(ctx, sr.ChunkID)
		if err != nil {
			continue
		}

		topicID, err := s.chunkStore.DocumentTopicID(ctx, chunk.DocumentID)
		if err != nil {
			continue
		}

		results = append(results, Result{
			Chunk:   chunk,
			TopicID: topicID,
			Score:   sr.Score,
		})
	}

	return results, nil
}

// resolveTopics determines which topics the principal can search.
func (s *Searcher) resolveTopics(ctx context.Context, principal *auth.Principal, requestedTopicIDs []string) ([]string, error) {
	if len(requestedTopicIDs) > 0 {
		// Validate that the principal has read access to each requested topic.
		var accessible []string
		for _, tid := range requestedTopicIDs {
			if err := s.authorizer.Check(ctx, principal, tid, auth.ActionRead); err == nil {
				accessible = append(accessible, tid)
			}
		}
		return accessible, nil
	}

	// No topics specified; search all accessible topics.
	return s.authorizer.AccessibleTopics(ctx, principal, auth.ActionRead)
}
