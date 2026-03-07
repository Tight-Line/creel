// Package pgvector implements the vector.Backend interface using PostgreSQL with pgvector.
package pgvector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pgvec "github.com/pgvector/pgvector-go"

	"github.com/Tight-Line/creel/internal/vector"
)

// DBTX is the database interface used by Backend. Both *pgxpool.Pool and *pgx.Conn
// satisfy this interface.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Backend implements vector.Backend using pgvector cosine similarity.
type Backend struct {
	db  DBTX
	dim int
}

// DefaultEmbeddingDimension is the default dimension for the pgvector
// chunk_embeddings column. Must match the vector(N) in the migration.
const DefaultEmbeddingDimension = 1536

// New creates a new pgvector backend with the default embedding dimension (1536).
func New(db DBTX) *Backend {
	return &Backend{db: db, dim: DefaultEmbeddingDimension}
}

// NewWithDimension creates a new pgvector backend with a custom embedding dimension.
func NewWithDimension(db DBTX, dim int) *Backend {
	return &Backend{db: db, dim: dim}
}

// EmbeddingDimension returns the number of dimensions this backend expects.
func (b *Backend) EmbeddingDimension() int {
	return b.dim
}

// Store persists a chunk's embedding.
func (b *Backend) Store(ctx context.Context, id string, embedding []float64, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	vec := toFloat32(embedding)
	_, err = b.db.Exec(ctx,
		`INSERT INTO chunk_embeddings (chunk_id, embedding, metadata)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (chunk_id) DO UPDATE SET embedding = $2, metadata = $3`,
		id, pgvec.NewVector(vec), metaJSON,
	)
	if err != nil {
		return fmt.Errorf("storing embedding: %w", err)
	}
	return nil
}

// Delete removes a chunk's embedding.
func (b *Backend) Delete(ctx context.Context, id string) error {
	_, err := b.db.Exec(ctx,
		`DELETE FROM chunk_embeddings WHERE chunk_id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting embedding: %w", err)
	}
	return nil
}

// Search returns chunk IDs ranked by cosine similarity.
func (b *Backend) Search(ctx context.Context, query []float64, filter vector.Filter, topK int) ([]vector.SearchResult, error) {
	vec := toFloat32(query)

	// Build query with optional chunk ID filtering.
	q := `SELECT chunk_id, 1 - (embedding <=> $1) AS score FROM chunk_embeddings`
	args := []any{pgvec.NewVector(vec)}
	argIdx := 2

	var hasWhere bool
	if len(filter.ChunkIDs) > 0 {
		q += fmt.Sprintf(` WHERE chunk_id = ANY($%d)`, argIdx)
		args = append(args, filter.ChunkIDs)
		argIdx++
		hasWhere = true
	}

	if len(filter.Metadata) > 0 {
		metaJSON, err := json.Marshal(filter.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshaling metadata filter: %w", err)
		}
		if hasWhere {
			q += fmt.Sprintf(` AND metadata @> $%d`, argIdx)
		} else {
			q += fmt.Sprintf(` WHERE metadata @> $%d`, argIdx)
		}
		args = append(args, metaJSON)
		argIdx++
	}

	q += fmt.Sprintf(` ORDER BY embedding <=> $1 LIMIT $%d`, argIdx)
	args = append(args, topK)

	rows, err := b.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("searching embeddings: %w", err)
	}
	defer rows.Close()

	var results []vector.SearchResult
	for rows.Next() {
		var r vector.SearchResult
		if err := rows.Scan(&r.ChunkID, &r.Score); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// StoreBatch persists multiple embeddings.
func (b *Backend) StoreBatch(ctx context.Context, items []vector.StoreItem) error {
	for _, item := range items {
		if err := b.Store(ctx, item.ID, item.Embedding, item.Metadata); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBatch removes multiple embeddings.
func (b *Backend) DeleteBatch(ctx context.Context, ids []string) error {
	_, err := b.db.Exec(ctx,
		`DELETE FROM chunk_embeddings WHERE chunk_id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("batch deleting embeddings: %w", err)
	}
	return nil
}

// Ping checks backend connectivity.
func (b *Backend) Ping(ctx context.Context) error {
	_, err := b.db.Exec(ctx, "SELECT 1")
	return err
}

func toFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}
