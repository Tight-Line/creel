// Package pgvector implements the vector.Backend interface using PostgreSQL with pgvector.
package pgvector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	pgvec "github.com/pgvector/pgvector-go"

	"github.com/Tight-Line/creel/internal/vector"
)

// Backend implements vector.Backend using pgvector cosine similarity.
type Backend struct {
	pool *pgxpool.Pool
	dim  int
}

// DefaultEmbeddingDimension is the default dimension for the pgvector
// chunk_embeddings column. Must match the vector(N) in the migration.
const DefaultEmbeddingDimension = 1536

// New creates a new pgvector backend with the default embedding dimension (1536).
// coverage:ignore - requires pgvector
func New(pool *pgxpool.Pool) *Backend {
	return &Backend{pool: pool, dim: DefaultEmbeddingDimension}
}

// NewWithDimension creates a new pgvector backend with a custom embedding dimension.
// coverage:ignore - requires pgvector
func NewWithDimension(pool *pgxpool.Pool, dim int) *Backend {
	return &Backend{pool: pool, dim: dim}
}

// EmbeddingDimension returns the number of dimensions this backend expects.
// coverage:ignore - requires pgvector
func (b *Backend) EmbeddingDimension() int {
	return b.dim
}

// Store persists a chunk's embedding.
// coverage:ignore - requires pgvector
func (b *Backend) Store(ctx context.Context, id string, embedding []float64, metadata map[string]any) error {
	// coverage:ignore - requires pgvector
	if metadata == nil {
		metadata = map[string]any{}
	}
	// coverage:ignore - requires pgvector
	metaJSON, err := json.Marshal(metadata)
	// coverage:ignore - requires pgvector
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// coverage:ignore - requires pgvector
	vec := toFloat32(embedding)
	_, err = b.pool.Exec(ctx,
		`INSERT INTO chunk_embeddings (chunk_id, embedding, metadata)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (chunk_id) DO UPDATE SET embedding = $2, metadata = $3`,
		id, pgvec.NewVector(vec), metaJSON,
	)
	// coverage:ignore - requires pgvector
	if err != nil {
		return fmt.Errorf("storing embedding: %w", err)
	}
	// coverage:ignore - requires pgvector
	return nil
}

// Delete removes a chunk's embedding.
// coverage:ignore - requires pgvector
func (b *Backend) Delete(ctx context.Context, id string) error {
	_, err := b.pool.Exec(ctx,
		`DELETE FROM chunk_embeddings WHERE chunk_id = $1`, id)
	// coverage:ignore - requires pgvector
	if err != nil {
		return fmt.Errorf("deleting embedding: %w", err)
	}
	// coverage:ignore - requires pgvector
	return nil
}

// Search returns chunk IDs ranked by cosine similarity.
// coverage:ignore - requires pgvector
func (b *Backend) Search(ctx context.Context, query []float64, filter vector.Filter, topK int) ([]vector.SearchResult, error) {
	vec := toFloat32(query)

	// Build query with optional chunk ID filtering.
	q := `SELECT chunk_id, 1 - (embedding <=> $1) AS score FROM chunk_embeddings`
	args := []any{pgvec.NewVector(vec)}
	argIdx := 2

	var hasWhere bool
	// coverage:ignore - requires pgvector
	if len(filter.ChunkIDs) > 0 {
		q += fmt.Sprintf(` WHERE chunk_id = ANY($%d)`, argIdx)
		args = append(args, filter.ChunkIDs)
		argIdx++
		hasWhere = true
	}

	// coverage:ignore - requires pgvector
	if len(filter.Metadata) > 0 {
		metaJSON, err := json.Marshal(filter.Metadata)
		// coverage:ignore - requires pgvector
		if err != nil {
			return nil, fmt.Errorf("marshaling metadata filter: %w", err)
		}
		// coverage:ignore - requires pgvector
		if hasWhere {
			q += fmt.Sprintf(` AND metadata @> $%d`, argIdx)
			// coverage:ignore - requires pgvector
		} else {
			q += fmt.Sprintf(` WHERE metadata @> $%d`, argIdx)
		}
		// coverage:ignore - requires pgvector
		args = append(args, metaJSON)
		argIdx++
	}

	// coverage:ignore - requires pgvector
	q += fmt.Sprintf(` ORDER BY embedding <=> $1 LIMIT $%d`, argIdx)
	args = append(args, topK)

	rows, err := b.pool.Query(ctx, q, args...)
	// coverage:ignore - requires pgvector
	if err != nil {
		return nil, fmt.Errorf("searching embeddings: %w", err)
	}
	// coverage:ignore - requires pgvector
	defer rows.Close()

	var results []vector.SearchResult
	// coverage:ignore - requires pgvector
	for rows.Next() {
		var r vector.SearchResult
		// coverage:ignore - requires pgvector
		if err := rows.Scan(&r.ChunkID, &r.Score); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		// coverage:ignore - requires pgvector
		results = append(results, r)
	}
	// coverage:ignore - requires pgvector
	return results, rows.Err()
}

// StoreBatch persists multiple embeddings.
// coverage:ignore - requires pgvector
func (b *Backend) StoreBatch(ctx context.Context, items []vector.StoreItem) error {
	// coverage:ignore - requires pgvector
	for _, item := range items {
		// coverage:ignore - requires pgvector
		if err := b.Store(ctx, item.ID, item.Embedding, item.Metadata); err != nil {
			return err
		}
	}
	// coverage:ignore - requires pgvector
	return nil
}

// DeleteBatch removes multiple embeddings.
// coverage:ignore - requires pgvector
func (b *Backend) DeleteBatch(ctx context.Context, ids []string) error {
	_, err := b.pool.Exec(ctx,
		`DELETE FROM chunk_embeddings WHERE chunk_id = ANY($1)`, ids)
	// coverage:ignore - requires pgvector
	if err != nil {
		return fmt.Errorf("batch deleting embeddings: %w", err)
	}
	// coverage:ignore - requires pgvector
	return nil
}

// Ping checks backend connectivity.
// coverage:ignore - requires pgvector
func (b *Backend) Ping(ctx context.Context) error {
	return b.pool.Ping(ctx)
}

// coverage:ignore - requires pgvector
func toFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	// coverage:ignore - requires pgvector
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	// coverage:ignore - requires pgvector
	return f32
}
