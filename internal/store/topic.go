package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TopicStore handles topic and grant persistence.
type TopicStore struct {
	pool DBTX
}

// NewTopicStore creates a new topic store.
func NewTopicStore(pool DBTX) *TopicStore {
	return &TopicStore{pool: pool}
}

// ChunkingStrategy configures how documents in a topic are chunked.
// Type can be "fixed" (default) or "semantic". When "semantic", the chunking
// worker uses an LLM to identify natural split points in the text.
type ChunkingStrategy struct {
	Type         string `json:"type,omitempty"`
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap int    `json:"chunk_overlap"`
}

// Topic represents a stored topic.
type Topic struct {
	ID                       string
	Slug                     string
	Name                     string
	Description              string
	Owner                    string
	CreatedAt                time.Time
	UpdatedAt                time.Time
	LLMConfigID              *string
	EmbeddingConfigID        *string
	ExtractionPromptConfigID *string
	ChunkingStrategy         *ChunkingStrategy
	MemoryEnabled            bool
}

// TopicGrant represents a stored topic grant.
type TopicGrant struct {
	ID         string
	TopicID    string
	Principal  string
	Permission string
	GrantedBy  string
	CreatedAt  time.Time
}

// scanTopicChunkingStrategy scans the chunking_strategy JSONB column into a Topic.
func scanTopicChunkingStrategy(t *Topic, data []byte) {
	if data != nil {
		var cs ChunkingStrategy
		if err := json.Unmarshal(data, &cs); err == nil {
			t.ChunkingStrategy = &cs
		}
	}
}

// Create inserts a new topic with optional config IDs.
func (s *TopicStore) Create(ctx context.Context, slug, name, description, owner string, llmConfigID, embeddingConfigID, extractionPromptConfigID *string, memoryEnabled bool) (*Topic, error) {
	var t Topic
	var chunkingBytes []byte
	err := s.pool.QueryRow(ctx,
		`INSERT INTO topics (slug, name, description, owner, llm_config_id, embedding_config_id, extraction_prompt_config_id, memory_enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, slug, name, description, owner, created_at, updated_at,
		           llm_config_id, embedding_config_id, extraction_prompt_config_id, chunking_strategy, memory_enabled`,
		slug, name, description, owner, llmConfigID, embeddingConfigID, extractionPromptConfigID, memoryEnabled,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Owner, &t.CreatedAt, &t.UpdatedAt,
		&t.LLMConfigID, &t.EmbeddingConfigID, &t.ExtractionPromptConfigID, &chunkingBytes, &t.MemoryEnabled)
	if err != nil {
		return nil, fmt.Errorf("inserting topic: %w", err)
	}
	scanTopicChunkingStrategy(&t, chunkingBytes)
	return &t, nil
}

// Get retrieves a topic by ID.
func (s *TopicStore) Get(ctx context.Context, id string) (*Topic, error) {
	var t Topic
	var chunkingBytes []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, slug, name, description, owner, created_at, updated_at,
		        llm_config_id, embedding_config_id, extraction_prompt_config_id, chunking_strategy, memory_enabled
		 FROM topics WHERE id = $1`, id,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Owner, &t.CreatedAt, &t.UpdatedAt,
		&t.LLMConfigID, &t.EmbeddingConfigID, &t.ExtractionPromptConfigID, &chunkingBytes, &t.MemoryEnabled)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("topic not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying topic: %w", err)
	}
	scanTopicChunkingStrategy(&t, chunkingBytes)
	return &t, nil
}

// ListForPrincipals returns topics accessible to the given principal identities.
// If principals is nil, returns all topics (for system accounts).
func (s *TopicStore) ListForPrincipals(ctx context.Context, principals []string) ([]Topic, error) {
	var rows pgx.Rows
	var err error

	if principals == nil {
		rows, err = s.pool.Query(ctx,
			`SELECT id, slug, name, description, owner, created_at, updated_at,
			        llm_config_id, embedding_config_id, extraction_prompt_config_id, chunking_strategy, memory_enabled
			 FROM topics ORDER BY created_at`)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT DISTINCT t.id, t.slug, t.name, t.description, t.owner, t.created_at, t.updated_at,
			        t.llm_config_id, t.embedding_config_id, t.extraction_prompt_config_id, t.chunking_strategy, t.memory_enabled
			 FROM topics t
			 LEFT JOIN topic_grants g ON g.topic_id = t.id
			 WHERE t.owner = ANY($1) OR g.principal = ANY($1)
			 ORDER BY t.created_at`, principals)
	}
	if err != nil {
		return nil, fmt.Errorf("listing topics: %w", err)
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		var t Topic
		var chunkingBytes []byte
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Owner, &t.CreatedAt, &t.UpdatedAt,
			&t.LLMConfigID, &t.EmbeddingConfigID, &t.ExtractionPromptConfigID, &chunkingBytes, &t.MemoryEnabled); err != nil {
			return nil, fmt.Errorf("scanning topic: %w", err)
		}
		scanTopicChunkingStrategy(&t, chunkingBytes)
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// Update modifies a topic's name, description, and config bindings.
func (s *TopicStore) Update(ctx context.Context, id, name, description string, llmConfigID, embeddingConfigID, extractionPromptConfigID *string, memoryEnabled *bool) (*Topic, error) {
	var t Topic
	var chunkingBytes []byte
	err := s.pool.QueryRow(ctx,
		`UPDATE topics
		 SET name = $2, description = $3,
		     llm_config_id = COALESCE($4, llm_config_id),
		     embedding_config_id = COALESCE($5, embedding_config_id),
		     extraction_prompt_config_id = COALESCE($6, extraction_prompt_config_id),
		     memory_enabled = COALESCE($7, memory_enabled),
		     updated_at = now()
		 WHERE id = $1
		 RETURNING id, slug, name, description, owner, created_at, updated_at,
		           llm_config_id, embedding_config_id, extraction_prompt_config_id, chunking_strategy, memory_enabled`,
		id, name, description, llmConfigID, embeddingConfigID, extractionPromptConfigID, memoryEnabled,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Owner, &t.CreatedAt, &t.UpdatedAt,
		&t.LLMConfigID, &t.EmbeddingConfigID, &t.ExtractionPromptConfigID, &chunkingBytes, &t.MemoryEnabled)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("topic not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating topic: %w", err)
	}
	scanTopicChunkingStrategy(&t, chunkingBytes)
	return &t, nil
}

// Delete removes a topic and cascades to grants, documents, chunks.
func (s *TopicStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM topics WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting topic: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("topic not found")
	}
	return nil
}

// Grant creates or updates a topic grant.
func (s *TopicStore) Grant(ctx context.Context, topicID, principal, permission, grantedBy string) (*TopicGrant, error) {
	var g TopicGrant
	err := s.pool.QueryRow(ctx,
		`INSERT INTO topic_grants (topic_id, principal, permission, granted_by)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (topic_id, principal) DO UPDATE SET permission = $3
		 RETURNING id, topic_id, principal, permission, granted_by, created_at`,
		topicID, principal, permission, grantedBy,
	).Scan(&g.ID, &g.TopicID, &g.Principal, &g.Permission, &g.GrantedBy, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("granting access: %w", err)
	}
	return &g, nil
}

// Revoke removes a topic grant.
func (s *TopicStore) Revoke(ctx context.Context, topicID, principal string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM topic_grants WHERE topic_id = $1 AND principal = $2`,
		topicID, principal,
	)
	if err != nil {
		return fmt.Errorf("revoking access: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("grant not found")
	}
	return nil
}

// ListGrants returns all grants for a topic.
func (s *TopicStore) ListGrants(ctx context.Context, topicID string) ([]TopicGrant, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, topic_id, principal, permission, granted_by, created_at
		 FROM topic_grants WHERE topic_id = $1 ORDER BY created_at`, topicID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing grants: %w", err)
	}
	defer rows.Close()

	var grants []TopicGrant
	for rows.Next() {
		var g TopicGrant
		if err := rows.Scan(&g.ID, &g.TopicID, &g.Principal, &g.Permission, &g.GrantedBy, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning grant: %w", err)
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}
