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
func (s *Searcher) Search(ctx context.Context, principal *auth.Principal, topicIDs []string, queryEmbedding []float64, topK int, metadataFilter map[string]any, excludeDocIDs []string) ([]Result, error) {
	// Resolve accessible topics.
	accessible, err := s.resolveTopics(ctx, principal, topicIDs)
	if err != nil {
		return nil, err
	}
	if len(accessible) == 0 {
		return nil, nil
	}

	// Get chunk IDs in accessible topics, excluding specified documents.
	chunkIDs, err := s.chunkStore.ChunkIDsByTopics(ctx, accessible, excludeDocIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching chunk IDs: %w", err)
	}
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	// Search vector backend with ACL filter.
	filter := vector.Filter{ChunkIDs: chunkIDs, Metadata: metadataFilter}
	searchResults, err := s.backend.Search(ctx, queryEmbedding, filter, topK)
	if err != nil {
		return nil, fmt.Errorf("searching vector backend: %w", err)
	}
	if len(searchResults) == 0 {
		return nil, nil
	}

	// Collect chunk IDs from search results, fetch all at once.
	resultChunkIDs := make([]string, len(searchResults))
	for i, sr := range searchResults {
		resultChunkIDs[i] = sr.ChunkID
	}

	chunks, err := s.chunkStore.GetMultiple(ctx, resultChunkIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching chunks: %w", err)
	}

	// Collect unique document IDs, fetch all topic mappings at once.
	docIDSet := make(map[string]struct{})
	for _, c := range chunks {
		docIDSet[c.DocumentID] = struct{}{}
	}
	docIDs := make([]string, 0, len(docIDSet))
	for id := range docIDSet {
		docIDs = append(docIDs, id)
	}

	docTopics, err := s.chunkStore.DocumentTopicIDs(ctx, docIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching document topics: %w", err)
	}

	// Assemble results preserving search result order.
	var results []Result
	for _, sr := range searchResults {
		chunk, ok := chunks[sr.ChunkID]
		if !ok {
			continue
		}
		topicID, ok := docTopics[chunk.DocumentID]
		if !ok {
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
// Uses a single batch query via AccessibleTopics, then intersects with
// the requested set if specific topics were provided.
func (s *Searcher) resolveTopics(ctx context.Context, principal *auth.Principal, requestedTopicIDs []string) ([]string, error) {
	allAccessible, err := s.authorizer.AccessibleTopics(ctx, principal, auth.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("fetching accessible topics: %w", err)
	}

	if len(requestedTopicIDs) == 0 {
		return allAccessible, nil
	}

	// Intersect requested with accessible.
	accessibleSet := make(map[string]struct{}, len(allAccessible))
	for _, id := range allAccessible {
		accessibleSet[id] = struct{}{}
	}

	var result []string
	for _, id := range requestedTopicIDs {
		if _, ok := accessibleSet[id]; ok {
			result = append(result, id)
		}
	}
	return result, nil
}
