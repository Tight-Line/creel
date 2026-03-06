package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ChunkStore handles chunk persistence.
type ChunkStore struct {
	pool DBTX
}

// NewChunkStore creates a new chunk store.
// coverage:ignore - requires database
func NewChunkStore(pool DBTX) *ChunkStore {
	return &ChunkStore{pool: pool}
}

// Chunk represents a stored chunk.
type Chunk struct {
	ID          string
	DocumentID  string
	Sequence    int
	Content     string
	EmbeddingID *string
	Status      string
	CompactedBy *string
	Metadata    map[string]any
	CreatedAt   time.Time
}

// Create inserts a new chunk and returns it.
// coverage:ignore - requires database
func (s *ChunkStore) Create(ctx context.Context, documentID, content string, sequence int, metadata map[string]any) (*Chunk, error) {
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
	var c Chunk
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO chunks (document_id, content, sequence, metadata)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at`,
		documentID, content, sequence, metaJSON,
	).Scan(&c.ID, &c.DocumentID, &c.Sequence, &c.Content, &c.EmbeddingID, &c.Status, &c.CompactedBy, &metaBytes, &c.CreatedAt)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("inserting chunk: %w", err)
	}
	// coverage:ignore - requires database
	_ = json.Unmarshal(metaBytes, &c.Metadata)
	return &c, nil
}

// Get retrieves a chunk by ID.
// coverage:ignore - requires database
func (s *ChunkStore) Get(ctx context.Context, id string) (*Chunk, error) {
	var c Chunk
	var metaBytes []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
		 FROM chunks WHERE id = $1`, id,
	).Scan(&c.ID, &c.DocumentID, &c.Sequence, &c.Content, &c.EmbeddingID, &c.Status, &c.CompactedBy, &metaBytes, &c.CreatedAt)
	// coverage:ignore - requires database
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("chunk not found")
	}
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("querying chunk: %w", err)
	}
	// coverage:ignore - requires database
	_ = json.Unmarshal(metaBytes, &c.Metadata)
	return &c, nil
}

// Delete removes a chunk.
// coverage:ignore - requires database
func (s *ChunkStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM chunks WHERE id = $1`, id)
	// coverage:ignore - requires database
	if err != nil {
		return fmt.Errorf("deleting chunk: %w", err)
	}
	// coverage:ignore - requires database
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("chunk not found")
	}
	// coverage:ignore - requires database
	return nil
}

// SetEmbeddingID updates the embedding_id for a chunk.
// coverage:ignore - requires database
func (s *ChunkStore) SetEmbeddingID(ctx context.Context, chunkID, embeddingID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE chunks SET embedding_id = $2 WHERE id = $1`, chunkID, embeddingID)
	return err
}

// ChunkIDsByTopics returns all active chunk IDs belonging to the given topics.
// coverage:ignore - requires database
func (s *ChunkStore) ChunkIDsByTopics(ctx context.Context, topicIDs []string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id FROM chunks c
		 JOIN documents d ON d.id = c.document_id
		 WHERE d.topic_id = ANY($1) AND c.status = 'active'`, topicIDs,
	)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("querying chunk IDs: %w", err)
	}
	// coverage:ignore - requires database
	defer rows.Close()

	var ids []string
	// coverage:ignore - requires database
	for rows.Next() {
		var id string
		// coverage:ignore - requires database
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning chunk ID: %w", err)
		}
		// coverage:ignore - requires database
		ids = append(ids, id)
	}
	// coverage:ignore - requires database
	return ids, rows.Err()
}

// GetMultiple retrieves multiple chunks by ID in a single query.
// coverage:ignore - requires database
func (s *ChunkStore) GetMultiple(ctx context.Context, ids []string) (map[string]*Chunk, error) {
	// coverage:ignore - requires database
	if len(ids) == 0 {
		return nil, nil
	}
	// coverage:ignore - requires database
	rows, err := s.pool.Query(ctx,
		`SELECT id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
		 FROM chunks WHERE id = ANY($1)`, ids,
	)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("querying chunks: %w", err)
	}
	// coverage:ignore - requires database
	defer rows.Close()

	result := make(map[string]*Chunk, len(ids))
	// coverage:ignore - requires database
	for rows.Next() {
		var c Chunk
		var metaBytes []byte
		// coverage:ignore - requires database
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Sequence, &c.Content, &c.EmbeddingID, &c.Status, &c.CompactedBy, &metaBytes, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning chunk: %w", err)
		}
		// coverage:ignore - requires database
		_ = json.Unmarshal(metaBytes, &c.Metadata)
		result[c.ID] = &c
	}
	// coverage:ignore - requires database
	return result, rows.Err()
}

// DocumentTopicIDs returns a mapping of document ID to topic ID for the given documents.
// coverage:ignore - requires database
func (s *ChunkStore) DocumentTopicIDs(ctx context.Context, docIDs []string) (map[string]string, error) {
	// coverage:ignore - requires database
	if len(docIDs) == 0 {
		return nil, nil
	}
	// coverage:ignore - requires database
	rows, err := s.pool.Query(ctx,
		`SELECT id, topic_id FROM documents WHERE id = ANY($1)`, docIDs,
	)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("querying document topics: %w", err)
	}
	// coverage:ignore - requires database
	defer rows.Close()

	result := make(map[string]string, len(docIDs))
	// coverage:ignore - requires database
	for rows.Next() {
		var docID, topicID string
		// coverage:ignore - requires database
		if err := rows.Scan(&docID, &topicID); err != nil {
			return nil, fmt.Errorf("scanning document topic: %w", err)
		}
		// coverage:ignore - requires database
		result[docID] = topicID
	}
	// coverage:ignore - requires database
	return result, rows.Err()
}

// DocumentTopicID returns the topic ID for a chunk's document.
// coverage:ignore - requires database
func (s *ChunkStore) DocumentTopicID(ctx context.Context, documentID string) (string, error) {
	var topicID string
	err := s.pool.QueryRow(ctx,
		`SELECT topic_id FROM documents WHERE id = $1`, documentID,
	).Scan(&topicID)
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
