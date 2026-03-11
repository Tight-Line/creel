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
func NewChunkStore(pool DBTX) *ChunkStore {
	return &ChunkStore{pool: pool}
}

// Chunk represents a stored chunk.
type Chunk struct {
	ID             string
	DocumentID     string
	Sequence       int
	Content        string
	EmbeddingID    *string
	Status         string
	CompactedBy    *string
	Metadata       map[string]any
	CreatedAt      time.Time
	EmbeddingModel string // from chunk_embeddings.metadata; not stored in chunks table
}

// Create inserts a new chunk and returns it.
func (s *ChunkStore) Create(ctx context.Context, documentID, content string, sequence int, metadata map[string]any) (*Chunk, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	var c Chunk
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO chunks (document_id, content, sequence, metadata)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at`,
		documentID, content, sequence, metaJSON,
	).Scan(&c.ID, &c.DocumentID, &c.Sequence, &c.Content, &c.EmbeddingID, &c.Status, &c.CompactedBy, &metaBytes, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting chunk: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &c.Metadata)
	return &c, nil
}

// Get retrieves a chunk by ID.
func (s *ChunkStore) Get(ctx context.Context, id string) (*Chunk, error) {
	var c Chunk
	var metaBytes []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
		 FROM chunks WHERE id = $1`, id,
	).Scan(&c.ID, &c.DocumentID, &c.Sequence, &c.Content, &c.EmbeddingID, &c.Status, &c.CompactedBy, &metaBytes, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("chunk not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying chunk: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &c.Metadata)
	return &c, nil
}

// Delete removes a chunk.
func (s *ChunkStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM chunks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting chunk: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("chunk not found")
	}
	return nil
}

// SetEmbeddingID updates the embedding_id for a chunk.
func (s *ChunkStore) SetEmbeddingID(ctx context.Context, chunkID, embeddingID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE chunks SET embedding_id = $2 WHERE id = $1`, chunkID, embeddingID)
	return err
}

// ChunkIDsByTopics returns all active chunk IDs belonging to the given topics.
func (s *ChunkStore) ChunkIDsByTopics(ctx context.Context, topicIDs []string, excludeDocIDs []string) ([]string, error) {
	var rows pgx.Rows
	var err error
	if len(excludeDocIDs) > 0 {
		rows, err = s.pool.Query(ctx,
			`SELECT c.id FROM chunks c
			 JOIN documents d ON d.id = c.document_id
			 WHERE d.topic_id = ANY($1) AND c.status = 'active'
			   AND c.document_id != ALL($2)`, topicIDs, excludeDocIDs,
		)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT c.id FROM chunks c
			 JOIN documents d ON d.id = c.document_id
			 WHERE d.topic_id = ANY($1) AND c.status = 'active'`, topicIDs,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("querying chunk IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning chunk ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetMultiple retrieves multiple chunks by ID in a single query.
func (s *ChunkStore) GetMultiple(ctx context.Context, ids []string) (map[string]*Chunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
		 FROM chunks WHERE id = ANY($1)`, ids,
	)
	if err != nil {
		return nil, fmt.Errorf("querying chunks: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*Chunk, len(ids))
	for rows.Next() {
		var c Chunk
		var metaBytes []byte
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Sequence, &c.Content, &c.EmbeddingID, &c.Status, &c.CompactedBy, &metaBytes, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning chunk: %w", err)
		}
		_ = json.Unmarshal(metaBytes, &c.Metadata)
		result[c.ID] = &c
	}
	return result, rows.Err()
}

// DocumentTopicIDs returns a mapping of document ID to topic ID for the given documents.
func (s *ChunkStore) DocumentTopicIDs(ctx context.Context, docIDs []string) (map[string]string, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, topic_id FROM documents WHERE id = ANY($1)`, docIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("querying document topics: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string, len(docIDs))
	for rows.Next() {
		var docID, topicID string
		if err := rows.Scan(&docID, &topicID); err != nil {
			return nil, fmt.Errorf("scanning document topic: %w", err)
		}
		result[docID] = topicID
	}
	return result, rows.Err()
}

// ListByDocument returns active chunks for a document in sequence order.
// If lastN > 0, returns only the last N chunks (ordered ascending by sequence).
// If since is non-zero, only returns chunks created at or after that time.
func (s *ChunkStore) ListByDocument(ctx context.Context, documentID string, lastN int, since time.Time) ([]*Chunk, error) {
	var args []any
	args = append(args, documentID)

	where := "document_id = $1 AND status = 'active'"
	if !since.IsZero() {
		args = append(args, since)
		where += fmt.Sprintf(" AND created_at >= $%d", len(args))
	}

	var query string
	if lastN > 0 {
		args = append(args, lastN)
		query = fmt.Sprintf(
			`SELECT id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
			 FROM (SELECT * FROM chunks WHERE %s ORDER BY sequence DESC LIMIT $%d) sub
			 ORDER BY sequence ASC`, where, len(args))
	} else {
		query = fmt.Sprintf(
			`SELECT id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
			 FROM chunks WHERE %s ORDER BY sequence ASC`, where)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing chunks by document: %w", err)
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		var c Chunk
		var metaBytes []byte
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Sequence, &c.Content, &c.EmbeddingID, &c.Status, &c.CompactedBy, &metaBytes, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning chunk: %w", err)
		}
		_ = json.Unmarshal(metaBytes, &c.Metadata)
		chunks = append(chunks, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating chunks: %w", err)
	}
	return chunks, nil
}

// MarkCompacted sets chunks to compacted status, recording which summary chunk replaced them.
func (s *ChunkStore) MarkCompacted(ctx context.Context, chunkIDs []string, summaryChunkID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE chunks SET status = 'compacted', compacted_by = $2, embedding_id = NULL WHERE id = ANY($1) AND status = 'active'`,
		chunkIDs, summaryChunkID)
	if err != nil {
		return fmt.Errorf("marking chunks as compacted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no active chunks found to compact")
	}
	return nil
}

// RestoreCompacted reactivates chunks that were compacted by the given summary chunk.
// Returns the IDs of the restored chunks.
func (s *ChunkStore) RestoreCompacted(ctx context.Context, summaryChunkID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`UPDATE chunks SET status = 'active', compacted_by = NULL
		 WHERE compacted_by = $1
		 RETURNING id`, summaryChunkID,
	)
	if err != nil {
		return nil, fmt.Errorf("restoring compacted chunks: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		// coverage:ignore - requires corrupted DB response during iteration
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning restored chunk ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// NextSequence returns the next available sequence number for a document's chunks.
func (s *ChunkStore) NextSequence(ctx context.Context, documentID string) (int, error) {
	var seq int
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(sequence), 0) + 1 FROM chunks WHERE document_id = $1`, documentID,
	).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("querying next sequence: %w", err)
	}
	return seq, nil
}

// DocumentTopicID returns the topic ID for a chunk's document.
func (s *ChunkStore) DocumentTopicID(ctx context.Context, documentID string) (string, error) {
	var topicID string
	err := s.pool.QueryRow(ctx,
		`SELECT topic_id FROM documents WHERE id = $1`, documentID,
	).Scan(&topicID)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("document not found")
	}
	if err != nil {
		return "", fmt.Errorf("querying document topic: %w", err)
	}
	return topicID, nil
}

// GetEmbeddingModels returns a map of chunk ID to embedding model name
// by reading the embedding_model key from chunk_embeddings.metadata.
func (s *ChunkStore) GetEmbeddingModels(ctx context.Context, chunkIDs []string) (map[string]string, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT chunk_id, metadata->>'embedding_model'
		 FROM chunk_embeddings
		 WHERE chunk_id = ANY($1) AND metadata->>'embedding_model' IS NOT NULL`,
		chunkIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("querying embedding models: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string, len(chunkIDs))
	// coverage:ignore - tested via integration
	for rows.Next() {
		var id, model string
		if err := rows.Scan(&id, &model); err != nil {
			return nil, fmt.Errorf("scanning embedding model: %w", err)
		}
		// coverage:ignore - tested via integration
		result[id] = model
	}
	return result, rows.Err()
}
