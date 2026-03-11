package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// MemoryStore handles memory persistence.
type MemoryStore struct {
	pool DBTX
}

// NewMemoryStore creates a new memory store.
func NewMemoryStore(pool DBTX) *MemoryStore {
	return &MemoryStore{pool: pool}
}

// Memory represents a stored memory.
type Memory struct {
	ID            string
	Principal     string
	Scope         string
	Content       string
	EmbeddingID   *string
	Subject       *string
	Predicate     *string
	Object        *string
	SourceChunkID *string
	Status        string
	InvalidatedAt *time.Time
	Metadata      map[string]any
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Create inserts a new memory and returns it.
func (s *MemoryStore) Create(ctx context.Context, m *Memory) (*Memory, error) {
	if m.Metadata == nil {
		m.Metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(m.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	var result Memory
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO memories (principal, scope, content, subject, predicate, object, source_chunk_id, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, principal, scope, content, embedding_id, subject, predicate, object,
		           source_chunk_id, status, invalidated_at, metadata, created_at, updated_at`,
		m.Principal, m.Scope, m.Content, m.Subject, m.Predicate, m.Object, m.SourceChunkID, metaJSON,
	).Scan(&result.ID, &result.Principal, &result.Scope, &result.Content, &result.EmbeddingID,
		&result.Subject, &result.Predicate, &result.Object, &result.SourceChunkID,
		&result.Status, &result.InvalidatedAt, &metaBytes, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting memory: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &result.Metadata)
	return &result, nil
}

// Get retrieves a memory by ID.
func (s *MemoryStore) Get(ctx context.Context, id string) (*Memory, error) {
	var m Memory
	var metaBytes []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, principal, scope, content, embedding_id, subject, predicate, object,
		        source_chunk_id, status, invalidated_at, metadata, created_at, updated_at
		 FROM memories WHERE id = $1`, id,
	).Scan(&m.ID, &m.Principal, &m.Scope, &m.Content, &m.EmbeddingID,
		&m.Subject, &m.Predicate, &m.Object, &m.SourceChunkID,
		&m.Status, &m.InvalidatedAt, &metaBytes, &m.CreatedAt, &m.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("memory not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying memory: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &m.Metadata)
	return &m, nil
}

// GetByScope returns all active memories for a principal in a scope.
func (s *MemoryStore) GetByScope(ctx context.Context, principal, scope string) ([]*Memory, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, principal, scope, content, embedding_id, subject, predicate, object,
		        source_chunk_id, status, invalidated_at, metadata, created_at, updated_at
		 FROM memories WHERE principal = $1 AND scope = $2 AND status = 'active'
		 ORDER BY created_at`, principal, scope,
	)
	if err != nil {
		return nil, fmt.Errorf("querying memories by scope: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// GetByScopes returns all active memories for a principal, optionally filtered by scopes.
// When scopes is nil or empty, returns all active memories across all scopes.
func (s *MemoryStore) GetByScopes(ctx context.Context, principal string, scopes []string) ([]*Memory, error) {
	var rows pgx.Rows
	var err error
	if len(scopes) == 0 {
		rows, err = s.pool.Query(ctx,
			`SELECT id, principal, scope, content, embedding_id, subject, predicate, object,
			        source_chunk_id, status, invalidated_at, metadata, created_at, updated_at
			 FROM memories WHERE principal = $1 AND status = 'active'
			 ORDER BY created_at`, principal,
		)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT id, principal, scope, content, embedding_id, subject, predicate, object,
			        source_chunk_id, status, invalidated_at, metadata, created_at, updated_at
			 FROM memories WHERE principal = $1 AND scope = ANY($2) AND status = 'active'
			 ORDER BY created_at`, principal, scopes,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("querying memories by scopes: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// Update modifies a memory's content and metadata.
func (s *MemoryStore) Update(ctx context.Context, id string, content string, metadata map[string]any) (*Memory, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	var m Memory
	var metaBytes []byte
	err = s.pool.QueryRow(ctx,
		`UPDATE memories SET content = $2, metadata = $3, updated_at = now()
		 WHERE id = $1
		 RETURNING id, principal, scope, content, embedding_id, subject, predicate, object,
		           source_chunk_id, status, invalidated_at, metadata, created_at, updated_at`,
		id, content, metaJSON,
	).Scan(&m.ID, &m.Principal, &m.Scope, &m.Content, &m.EmbeddingID,
		&m.Subject, &m.Predicate, &m.Object, &m.SourceChunkID,
		&m.Status, &m.InvalidatedAt, &metaBytes, &m.CreatedAt, &m.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("memory not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating memory: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &m.Metadata)
	return &m, nil
}

// Invalidate soft-deletes a memory by setting status to invalidated.
func (s *MemoryStore) Invalidate(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE memories SET status = 'invalidated', invalidated_at = now(), updated_at = now()
		 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("invalidating memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("memory not found")
	}
	return nil
}

// ListByScope returns memories for a principal in a scope, optionally including invalidated ones.
func (s *MemoryStore) ListByScope(ctx context.Context, principal, scope string, includeInvalidated bool) ([]*Memory, error) {
	var query string
	if includeInvalidated {
		query = `SELECT id, principal, scope, content, embedding_id, subject, predicate, object,
		                source_chunk_id, status, invalidated_at, metadata, created_at, updated_at
		         FROM memories WHERE principal = $1 AND scope = $2
		         ORDER BY created_at`
	} else {
		query = `SELECT id, principal, scope, content, embedding_id, subject, predicate, object,
		                source_chunk_id, status, invalidated_at, metadata, created_at, updated_at
		         FROM memories WHERE principal = $1 AND scope = $2 AND status = 'active'
		         ORDER BY created_at`
	}
	rows, err := s.pool.Query(ctx, query, principal, scope)
	if err != nil {
		return nil, fmt.Errorf("listing memories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// ListScopes returns distinct scopes for a principal.
func (s *MemoryStore) ListScopes(ctx context.Context, principal string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT scope FROM memories WHERE principal = $1 ORDER BY scope`, principal)
	if err != nil {
		return nil, fmt.Errorf("listing scopes: %w", err)
	}
	defer rows.Close()

	var scopes []string
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return nil, fmt.Errorf("scanning scope: %w", err)
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}

// GetMultiple retrieves multiple memories by ID in a single query.
func (s *MemoryStore) GetMultiple(ctx context.Context, ids []string) (map[string]*Memory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, principal, scope, content, embedding_id, subject, predicate, object,
		        source_chunk_id, status, invalidated_at, metadata, created_at, updated_at
		 FROM memories WHERE id = ANY($1)`, ids,
	)
	if err != nil {
		return nil, fmt.Errorf("querying memories: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*Memory, len(ids))
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		result[m.ID] = m
	}
	return result, rows.Err()
}

func scanMemory(rows pgx.Rows) (*Memory, error) {
	var m Memory
	var metaBytes []byte
	if err := rows.Scan(&m.ID, &m.Principal, &m.Scope, &m.Content, &m.EmbeddingID,
		&m.Subject, &m.Predicate, &m.Object, &m.SourceChunkID,
		&m.Status, &m.InvalidatedAt, &metaBytes, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scanning memory: %w", err)
	}
	_ = json.Unmarshal(metaBytes, &m.Metadata)
	return &m, nil
}

func scanMemories(rows pgx.Rows) ([]*Memory, error) {
	var memories []*Memory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}
