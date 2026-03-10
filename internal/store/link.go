package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// LinkStore handles link persistence.
type LinkStore struct {
	pool DBTX
}

// NewLinkStore creates a new link store.
func NewLinkStore(pool DBTX) *LinkStore {
	return &LinkStore{pool: pool}
}

// Link represents a stored link between two chunks.
type Link struct {
	ID            string
	SourceChunkID string
	TargetChunkID string
	LinkType      string
	CreatedBy     string
	Metadata      map[string]any
	CreatedAt     time.Time
}

// Create inserts a new link and returns it.
func (s *LinkStore) Create(ctx context.Context, sourceChunkID, targetChunkID, linkType, createdBy string, metadata map[string]any) (*Link, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	var l Link
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO links (source_chunk, target_chunk, link_type, created_by, metadata)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, source_chunk, target_chunk, link_type, created_by, metadata, created_at`,
		sourceChunkID, targetChunkID, linkType, createdBy, metaJSON,
	).Scan(&l.ID, &l.SourceChunkID, &l.TargetChunkID, &l.LinkType, &l.CreatedBy, &metaBytes, &l.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting link: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &l.Metadata)
	return &l, nil
}

// Get retrieves a link by ID.
func (s *LinkStore) Get(ctx context.Context, id string) (*Link, error) {
	var l Link
	var metaBytes []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, source_chunk, target_chunk, link_type, created_by, metadata, created_at
		 FROM links WHERE id = $1`, id,
	).Scan(&l.ID, &l.SourceChunkID, &l.TargetChunkID, &l.LinkType, &l.CreatedBy, &metaBytes, &l.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("link not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying link: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &l.Metadata)
	return &l, nil
}

// Delete removes a link.
func (s *LinkStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM links WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}

// ListByChunk returns links where the given chunk is the source.
// If includeBacklinks is true, also returns links where the chunk is the target.
func (s *LinkStore) ListByChunk(ctx context.Context, chunkID string, includeBacklinks bool) ([]*Link, error) {
	var query string
	if includeBacklinks {
		query = `SELECT id, source_chunk, target_chunk, link_type, created_by, metadata, created_at
				 FROM links WHERE source_chunk = $1 OR target_chunk = $1
				 ORDER BY created_at`
	} else {
		query = `SELECT id, source_chunk, target_chunk, link_type, created_by, metadata, created_at
				 FROM links WHERE source_chunk = $1
				 ORDER BY created_at`
	}

	rows, err := s.pool.Query(ctx, query, chunkID)
	if err != nil {
		return nil, fmt.Errorf("querying links: %w", err)
	}
	defer rows.Close()

	return scanLinks(rows)
}

// ListByChunks returns all links where any of the given chunk IDs is a source or target.
// Used for batch link retrieval during search result hydration.
func (s *LinkStore) ListByChunks(ctx context.Context, chunkIDs []string) ([]*Link, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, source_chunk, target_chunk, link_type, created_by, metadata, created_at
		 FROM links WHERE source_chunk = ANY($1) OR target_chunk = ANY($1)
		 ORDER BY created_at`, chunkIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("querying links by chunks: %w", err)
	}
	defer rows.Close()

	return scanLinks(rows)
}

// TransferLinks updates all links pointing to oldChunkID to point to newChunkID instead.
// Used during compaction to preserve link relationships.
func (s *LinkStore) TransferLinks(ctx context.Context, oldChunkID, newChunkID string) (int64, error) {
	var total int64

	tag, err := s.pool.Exec(ctx,
		`UPDATE links SET source_chunk = $2 WHERE source_chunk = $1`, oldChunkID, newChunkID)
	if err != nil {
		return 0, fmt.Errorf("transferring source links: %w", err)
	}
	total += tag.RowsAffected()

	tag, err = s.pool.Exec(ctx,
		`UPDATE links SET target_chunk = $2 WHERE target_chunk = $1`, oldChunkID, newChunkID)
	if err != nil {
		return 0, fmt.Errorf("transferring target links: %w", err)
	}
	total += tag.RowsAffected()

	return total, nil
}

// DeleteByChunk removes all links where the given chunk is source or target.
func (s *LinkStore) DeleteByChunk(ctx context.Context, chunkID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM links WHERE source_chunk = $1 OR target_chunk = $1`, chunkID)
	if err != nil {
		return fmt.Errorf("deleting links by chunk: %w", err)
	}
	return nil
}

func scanLinks(rows pgx.Rows) ([]*Link, error) {
	var links []*Link
	for rows.Next() {
		var l Link
		var metaBytes []byte
		if err := rows.Scan(&l.ID, &l.SourceChunkID, &l.TargetChunkID, &l.LinkType, &l.CreatedBy, &metaBytes, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning link: %w", err)
		}
		_ = json.Unmarshal(metaBytes, &l.Metadata)
		links = append(links, &l)
	}
	return links, rows.Err()
}
