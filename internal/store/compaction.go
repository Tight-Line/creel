package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CompactionStore handles compaction record persistence.
type CompactionStore struct {
	pool DBTX
}

// NewCompactionStore creates a new compaction store.
func NewCompactionStore(pool DBTX) *CompactionStore {
	return &CompactionStore{pool: pool}
}

// CompactionRecord tracks which source chunks were compacted into a summary chunk.
type CompactionRecord struct {
	ID             string
	SummaryChunkID string
	SourceChunkIDs []string
	DocumentID     string
	CreatedBy      string
	CreatedAt      time.Time
}

// Create inserts a compaction record.
func (s *CompactionStore) Create(ctx context.Context, summaryChunkID string, sourceChunkIDs []string, documentID, createdBy string) (*CompactionRecord, error) {
	var r CompactionRecord
	err := s.pool.QueryRow(ctx,
		`INSERT INTO compaction_records (summary_chunk_id, source_chunk_ids, document_id, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, summary_chunk_id, source_chunk_ids, document_id, created_by, created_at`,
		summaryChunkID, sourceChunkIDs, documentID, createdBy,
	).Scan(&r.ID, &r.SummaryChunkID, &r.SourceChunkIDs, &r.DocumentID, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting compaction record: %w", err)
	}
	return &r, nil
}

// GetBySummaryChunkID retrieves a compaction record by the summary chunk it produced.
func (s *CompactionStore) GetBySummaryChunkID(ctx context.Context, summaryChunkID string) (*CompactionRecord, error) {
	var r CompactionRecord
	err := s.pool.QueryRow(ctx,
		`SELECT id, summary_chunk_id, source_chunk_ids, document_id, created_by, created_at
		 FROM compaction_records WHERE summary_chunk_id = $1`, summaryChunkID,
	).Scan(&r.ID, &r.SummaryChunkID, &r.SourceChunkIDs, &r.DocumentID, &r.CreatedBy, &r.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("compaction record not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying compaction record: %w", err)
	}
	return &r, nil
}

// ListByDocument returns all compaction records for a document.
func (s *CompactionStore) ListByDocument(ctx context.Context, documentID string) ([]*CompactionRecord, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, summary_chunk_id, source_chunk_ids, document_id, created_by, created_at
		 FROM compaction_records WHERE document_id = $1 ORDER BY created_at`, documentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing compaction records: %w", err)
	}
	defer rows.Close()

	var records []*CompactionRecord
	for rows.Next() {
		var r CompactionRecord
		// coverage:ignore - requires corrupted DB response during iteration
		if err := rows.Scan(&r.ID, &r.SummaryChunkID, &r.SourceChunkIDs, &r.DocumentID, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning compaction record: %w", err)
		}
		records = append(records, &r)
	}
	return records, rows.Err()
}

// Delete removes a compaction record.
func (s *CompactionStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM compaction_records WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting compaction record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("compaction record not found")
	}
	return nil
}
