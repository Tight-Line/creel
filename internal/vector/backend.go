// Package vector defines the interface for pluggable vector storage backends.
package vector

import "context"

// Backend is the interface that all vector store implementations must satisfy.
type Backend interface {
	// EmbeddingDimension returns the number of dimensions this backend expects.
	EmbeddingDimension() int

	// Store persists a chunk's embedding.
	Store(ctx context.Context, id string, embedding []float64, metadata map[string]any) error

	// Delete removes a chunk's embedding.
	Delete(ctx context.Context, id string) error

	// Search returns chunk IDs ranked by similarity to the query vector.
	Search(ctx context.Context, query []float64, filter Filter, topK int) ([]SearchResult, error)

	// StoreBatch persists multiple embeddings.
	StoreBatch(ctx context.Context, items []StoreItem) error

	// DeleteBatch removes multiple embeddings.
	DeleteBatch(ctx context.Context, ids []string) error

	// Ping checks backend connectivity.
	Ping(ctx context.Context) error
}

// StoreItem represents a single embedding to store in a batch operation.
type StoreItem struct {
	ID        string
	Embedding []float64
	Metadata  map[string]any
}

// Filter constrains a similarity search.
type Filter struct {
	// ChunkIDs restricts search to specific chunks (used for ACL filtering).
	ChunkIDs []string
	// Metadata filters on chunk metadata fields.
	Metadata map[string]any
}

// SearchResult is a single result from a similarity search.
type SearchResult struct {
	ChunkID string
	Score   float64
}
