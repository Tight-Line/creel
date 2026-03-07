package retrieval

import (
	"context"
	"fmt"
	"time"

	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

// ContextFetcher retrieves chunks from a single document in sequence order,
// with ACL enforcement.
type ContextFetcher struct {
	chunkStore *store.ChunkStore
	authorizer auth.Authorizer
}

// NewContextFetcher creates a new ContextFetcher.
func NewContextFetcher(chunkStore *store.ChunkStore, authorizer auth.Authorizer) *ContextFetcher {
	return &ContextFetcher{
		chunkStore: chunkStore,
		authorizer: authorizer,
	}
}

// GetContext returns active chunks for a document in sequence order.
// It validates that the principal has read access to the document's topic.
func (f *ContextFetcher) GetContext(ctx context.Context, principal *auth.Principal, documentID string, lastN int, since time.Time) ([]*store.Chunk, error) {
	topicID, err := f.chunkStore.DocumentTopicID(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("resolving document topic: %w", err)
	}

	if err := f.authorizer.Check(ctx, principal, topicID, auth.ActionRead); err != nil {
		return nil, fmt.Errorf("access denied: %w", err)
	}

	chunks, err := f.chunkStore.ListByDocument(ctx, documentID, lastN, since)
	if err != nil {
		return nil, fmt.Errorf("listing chunks: %w", err)
	}
	return chunks, nil
}
