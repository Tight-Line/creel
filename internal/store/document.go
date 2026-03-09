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
func NewDocumentStore(pool DBTX) *DocumentStore {
	return &DocumentStore{pool: pool}
}

// Document represents a stored document.
type Document struct {
	ID          string
	TopicID     string
	Slug        string
	Name        string
	DocType     string
	Status      string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	URL         *string
	Author      *string
	PublishedAt *time.Time
}

// DocumentContent holds raw uploaded content and extracted text for a document.
type DocumentContent struct {
	DocumentID    string
	RawContent    []byte
	ContentType   string
	ExtractedText string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Create inserts a new document.
func (s *DocumentStore) Create(ctx context.Context, topicID, slug, name, docType string, metadata map[string]any, url, author *string, publishedAt *time.Time) (*Document, error) {
	return s.CreateWithStatus(ctx, topicID, slug, name, docType, "ready", metadata, url, author, publishedAt)
}

// CreateWithStatus inserts a new document with an explicit status.
func (s *DocumentStore) CreateWithStatus(ctx context.Context, topicID, slug, name, docType, docStatus string, metadata map[string]any, url, author *string, publishedAt *time.Time) (*Document, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	var d Document
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO documents (topic_id, slug, name, doc_type, status, metadata, url, author, published_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, topic_id, slug, name, doc_type, status, metadata, created_at, updated_at, url, author, published_at`,
		topicID, slug, name, docType, docStatus, metaJSON, url, author, publishedAt,
	).Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &d.Status, &metaBytes, &d.CreatedAt, &d.UpdatedAt, &d.URL, &d.Author, &d.PublishedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting document: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &d.Metadata)
	return &d, nil
}

// Get retrieves a document by ID.
func (s *DocumentStore) Get(ctx context.Context, id string) (*Document, error) {
	var d Document
	var metaBytes []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, topic_id, slug, name, doc_type, status, metadata, created_at, updated_at, url, author, published_at
		 FROM documents WHERE id = $1`, id,
	).Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &d.Status, &metaBytes, &d.CreatedAt, &d.UpdatedAt, &d.URL, &d.Author, &d.PublishedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying document: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &d.Metadata)
	return &d, nil
}

// ListByTopic returns documents in a topic.
func (s *DocumentStore) ListByTopic(ctx context.Context, topicID string) ([]Document, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, topic_id, slug, name, doc_type, status, metadata, created_at, updated_at, url, author, published_at
		 FROM documents WHERE topic_id = $1 ORDER BY created_at`, topicID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		var metaBytes []byte
		if err := rows.Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &d.Status, &metaBytes, &d.CreatedAt, &d.UpdatedAt, &d.URL, &d.Author, &d.PublishedAt); err != nil {
			return nil, fmt.Errorf("scanning document: %w", err)
		}
		_ = json.Unmarshal(metaBytes, &d.Metadata)
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// Update modifies a document's mutable fields.
func (s *DocumentStore) Update(ctx context.Context, id, name, docType string, metadata map[string]any, url, author *string, publishedAt *time.Time) (*Document, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	var d Document
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`UPDATE documents SET name = $2, doc_type = $3, metadata = $4, url = $5, author = $6, published_at = $7, updated_at = now()
		 WHERE id = $1
		 RETURNING id, topic_id, slug, name, doc_type, status, metadata, created_at, updated_at, url, author, published_at`,
		id, name, docType, metaJSON, url, author, publishedAt,
	).Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &d.Status, &metaBytes, &d.CreatedAt, &d.UpdatedAt, &d.URL, &d.Author, &d.PublishedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating document: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &d.Metadata)
	return &d, nil
}

// GetMultiple retrieves multiple documents by ID in a single query.
func (s *DocumentStore) GetMultiple(ctx context.Context, ids []string) (map[string]*Document, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, topic_id, slug, name, doc_type, status, metadata, created_at, updated_at, url, author, published_at
		 FROM documents WHERE id = ANY($1)`, ids,
	)
	if err != nil {
		return nil, fmt.Errorf("querying documents: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*Document, len(ids))
	for rows.Next() {
		var d Document
		var metaBytes []byte
		if err := rows.Scan(&d.ID, &d.TopicID, &d.Slug, &d.Name, &d.DocType, &d.Status, &metaBytes, &d.CreatedAt, &d.UpdatedAt, &d.URL, &d.Author, &d.PublishedAt); err != nil {
			return nil, fmt.Errorf("scanning document: %w", err)
		}
		_ = json.Unmarshal(metaBytes, &d.Metadata)
		result[d.ID] = &d
	}
	return result, rows.Err()
}

// Delete removes a document and cascades to chunks.
func (s *DocumentStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

// TopicIDForDocument returns the topic ID that owns the given document.
func (s *DocumentStore) TopicIDForDocument(ctx context.Context, docID string) (string, error) {
	var topicID string
	err := s.pool.QueryRow(ctx, `SELECT topic_id FROM documents WHERE id = $1`, docID).Scan(&topicID)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("document not found")
	}
	if err != nil {
		return "", fmt.Errorf("querying document topic: %w", err)
	}
	return topicID, nil
}

// UpdateStatus sets the status of a document.
func (s *DocumentStore) UpdateStatus(ctx context.Context, id, status string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE documents SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("updating document status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

// SaveContent stores raw content and content type for a document.
func (s *DocumentStore) SaveContent(ctx context.Context, docID string, rawContent []byte, contentType string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO document_content (document_id, raw_content, content_type)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (document_id) DO UPDATE SET raw_content = $2, content_type = $3, updated_at = now()`,
		docID, rawContent, contentType,
	)
	if err != nil {
		return fmt.Errorf("saving document content: %w", err)
	}
	return nil
}

// GetContent retrieves raw content for a document.
func (s *DocumentStore) GetContent(ctx context.Context, docID string) (*DocumentContent, error) {
	var dc DocumentContent
	err := s.pool.QueryRow(ctx,
		`SELECT document_id, raw_content, content_type, extracted_text, created_at, updated_at
		 FROM document_content WHERE document_id = $1`, docID,
	).Scan(&dc.DocumentID, &dc.RawContent, &dc.ContentType, &dc.ExtractedText, &dc.CreatedAt, &dc.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document content not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying document content: %w", err)
	}
	return &dc, nil
}

// SaveExtractedText stores extracted text for a document.
func (s *DocumentStore) SaveExtractedText(ctx context.Context, docID, text string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE document_content SET extracted_text = $2, updated_at = now() WHERE document_id = $1`,
		docID, text,
	)
	if err != nil {
		return fmt.Errorf("saving extracted text: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("document content not found")
	}
	return nil
}
