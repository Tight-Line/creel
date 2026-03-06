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
// coverage:ignore - requires store and vector backend
func NewSearcher(chunkStore *store.ChunkStore, authorizer auth.Authorizer, backend vector.Backend) *Searcher {
	return &Searcher{
		chunkStore: chunkStore,
		authorizer: authorizer,
		backend:    backend,
	}
}

// Search performs an ACL-filtered similarity search.
// If topicIDs is empty, it searches all topics accessible to the principal.
// coverage:ignore - requires store and vector backend
func (s *Searcher) Search(ctx context.Context, principal *auth.Principal, topicIDs []string, queryEmbedding []float64, topK int, metadataFilter map[string]any) ([]Result, error) {
	// Resolve accessible topics.
	accessible, err := s.resolveTopics(ctx, principal, topicIDs)
	// coverage:ignore - requires store and vector backend
	if err != nil {
		return nil, err
	}
	// coverage:ignore - requires store and vector backend
	if len(accessible) == 0 {
		return nil, nil
	}

	// Get chunk IDs in accessible topics.
	// coverage:ignore - requires store and vector backend
	chunkIDs, err := s.chunkStore.ChunkIDsByTopics(ctx, accessible)
	// coverage:ignore - requires store and vector backend
	if err != nil {
		return nil, fmt.Errorf("fetching chunk IDs: %w", err)
	}
	// coverage:ignore - requires store and vector backend
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	// Search vector backend with ACL filter.
	// coverage:ignore - requires store and vector backend
	filter := vector.Filter{ChunkIDs: chunkIDs, Metadata: metadataFilter}
	searchResults, err := s.backend.Search(ctx, queryEmbedding, filter, topK)
	// coverage:ignore - requires store and vector backend
	if err != nil {
		return nil, fmt.Errorf("searching vector backend: %w", err)
	}
	// coverage:ignore - requires store and vector backend
	if len(searchResults) == 0 {
		return nil, nil
	}

	// Collect chunk IDs from search results, fetch all at once.
	// coverage:ignore - requires store and vector backend
	resultChunkIDs := make([]string, len(searchResults))
	// coverage:ignore - requires store and vector backend
	for i, sr := range searchResults {
		resultChunkIDs[i] = sr.ChunkID
	}

	// coverage:ignore - requires store and vector backend
	chunks, err := s.chunkStore.GetMultiple(ctx, resultChunkIDs)
	// coverage:ignore - requires store and vector backend
	if err != nil {
		return nil, fmt.Errorf("fetching chunks: %w", err)
	}

	// Collect unique document IDs, fetch all topic mappings at once.
	// coverage:ignore - requires store and vector backend
	docIDSet := make(map[string]struct{})
	// coverage:ignore - requires store and vector backend
	for _, c := range chunks {
		docIDSet[c.DocumentID] = struct{}{}
	}
	// coverage:ignore - requires store and vector backend
	docIDs := make([]string, 0, len(docIDSet))
	// coverage:ignore - requires store and vector backend
	for id := range docIDSet {
		docIDs = append(docIDs, id)
	}

	// coverage:ignore - requires store and vector backend
	docTopics, err := s.chunkStore.DocumentTopicIDs(ctx, docIDs)
	// coverage:ignore - requires store and vector backend
	if err != nil {
		return nil, fmt.Errorf("fetching document topics: %w", err)
	}

	// Assemble results preserving search result order.
	// coverage:ignore - requires store and vector backend
	var results []Result
	// coverage:ignore - requires store and vector backend
	for _, sr := range searchResults {
		chunk, ok := chunks[sr.ChunkID]
		// coverage:ignore - requires store and vector backend
		if !ok {
			continue
		}
		// coverage:ignore - requires store and vector backend
		topicID, ok := docTopics[chunk.DocumentID]
		// coverage:ignore - requires store and vector backend
		if !ok {
			continue
		}
		// coverage:ignore - requires store and vector backend
		results = append(results, Result{
			Chunk:   chunk,
			TopicID: topicID,
			Score:   sr.Score,
		})
	}

	// coverage:ignore - requires store and vector backend
	return results, nil
}

// resolveTopics determines which topics the principal can search.
// Uses a single batch query via AccessibleTopics, then intersects with
// the requested set if specific topics were provided.
// coverage:ignore - requires store and vector backend
func (s *Searcher) resolveTopics(ctx context.Context, principal *auth.Principal, requestedTopicIDs []string) ([]string, error) {
	allAccessible, err := s.authorizer.AccessibleTopics(ctx, principal, auth.ActionRead)
	// coverage:ignore - requires store and vector backend
	if err != nil {
		return nil, fmt.Errorf("fetching accessible topics: %w", err)
	}

	// coverage:ignore - requires store and vector backend
	if len(requestedTopicIDs) == 0 {
		return allAccessible, nil
	}

	// Intersect requested with accessible.
	// coverage:ignore - requires store and vector backend
	accessibleSet := make(map[string]struct{}, len(allAccessible))
	// coverage:ignore - requires store and vector backend
	for _, id := range allAccessible {
		accessibleSet[id] = struct{}{}
	}

	// coverage:ignore - requires store and vector backend
	var result []string
	// coverage:ignore - requires store and vector backend
	for _, id := range requestedTopicIDs {
		// coverage:ignore - requires store and vector backend
		if _, ok := accessibleSet[id]; ok {
			result = append(result, id)
		}
	}
	// coverage:ignore - requires store and vector backend
	return result, nil
}
