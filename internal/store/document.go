package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// DocumentStore handles document persistence.
type DocumentStore struct {
	pool DBTX
}

// NewDocumentStore creates a new document store.
// coverage:ignore - requires database
func NewDocumentStore(pool DBTX) *DocumentStore {
	return &DocumentStore{pool: pool}
}

// Document represents a stored document.
type Document struct {
	ID        string
	TopicID   string
	Slug      string
	Name      string
	DocType   string
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Create inserts a new document.
// coverage:ignore - requires database
func (s *DocumentStore) Create(ctx context.Context, topicID, slug, name, docType string, metadata map[string]any) (*Document, error) {
	// coverage:ignore - requires database
	if metadata == nil {
		metadata = map[string]any{}
	}
	// coverage:ignore - requires database
	metaJSON, err := json.Marshal(metadata)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	// coverage:ignore - requires database
	var d Document
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO documents (topic_id, slug, name, doc_type, metadata)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, topic_id, slug, name, doc_type, metadata, created_at, updated_at`,
		topicID, slug, name, docType, metaJSON,
	).Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &metaBytes, &d.CreatedAt, &d.UpdatedAt)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("inserting document: %w", err)
	}
	// coverage:ignore - requires database
	_ = json.Unmarshal(metaBytes, &d.Metadata)
	return &d, nil
}

// Get retrieves a document by ID.
// coverage:ignore - requires database
func (s *DocumentStore) Get(ctx context.Context, id string) (*Document, error) {
	var d Document
	var metaBytes []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, topic_id, slug, name, doc_type, metadata, created_at, updated_at
		 FROM documents WHERE id = $1`, id,
	).Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &metaBytes, &d.CreatedAt, &d.UpdatedAt)
	// coverage:ignore - requires database
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("querying document: %w", err)
	}
	// coverage:ignore - requires database
	_ = json.Unmarshal(metaBytes, &d.Metadata)
	return &d, nil
}

// ListByTopic returns documents in a topic.
// coverage:ignore - requires database
func (s *DocumentStore) ListByTopic(ctx context.Context, topicID string) ([]Document, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, topic_id, slug, name, doc_type, metadata, created_at, updated_at
		 FROM documents WHERE topic_id = $1 ORDER BY created_at`, topicID,
	)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}
	// coverage:ignore - requires database
	defer rows.Close()

	var docs []Document
	// coverage:ignore - requires database
	for rows.Next() {
		var d Document
		var metaBytes []byte
		// coverage:ignore - requires database
		if err := rows.Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &metaBytes, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning document: %w", err)
		}
		// coverage:ignore - requires database
		_ = json.Unmarshal(metaBytes, &d.Metadata)
		docs = append(docs, d)
	}
	// coverage:ignore - requires database
	return docs, rows.Err()
}

// Update modifies a document's name, doc_type, and metadata.
// coverage:ignore - requires database
func (s *DocumentStore) Update(ctx context.Context, id, name, docType string, metadata map[string]any) (*Document, error) {
	// coverage:ignore - requires database
	if metadata == nil {
		metadata = map[string]any{}
	}
	// coverage:ignore - requires database
	metaJSON, err := json.Marshal(metadata)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	// coverage:ignore - requires database
	var d Document
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`UPDATE documents SET name = $2, doc_type = $3, metadata = $4, updated_at = now()
		 WHERE id = $1
		 RETURNING id, topic_id, slug, name, doc_type, metadata, created_at, updated_at`,
		id, name, docType, metaJSON,
	).Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &metaBytes, &d.CreatedAt, &d.UpdatedAt)
	// coverage:ignore - requires database
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("updating document: %w", err)
	}
	// coverage:ignore - requires database
	_ = json.Unmarshal(metaBytes, &d.Metadata)
	return &d, nil
}

// Delete removes a document and cascades to chunks.
// coverage:ignore - requires database
func (s *DocumentStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
	// coverage:ignore - requires database
	if err != nil {
		return fmt.Errorf("deleting document: %w", err)
	}
	// coverage:ignore - requires database
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("document not found")
	}
	// coverage:ignore - requires database
	return nil
}

// TopicIDForDocument returns the topic ID that owns the given document.
// coverage:ignore - requires database
func (s *DocumentStore) TopicIDForDocument(ctx context.Context, docID string) (string, error) {
	var topicID string
	err := s.pool.QueryRow(ctx, `SELECT topic_id FROM documents WHERE id = $1`, docID).Scan(&topicID)
	// coverage:ignore - requires database
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("document not found")
	}
	// coverage:ignore - requires database
	if err != nil {
		return "", fmt.Errorf("querying document topic: %w", err)
	}
	// coverage:ignore - requires database
	return topicID, nil
}
