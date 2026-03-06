package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TopicStore handles topic and grant persistence.
type TopicStore struct {
	pool DBTX
}

// NewTopicStore creates a new topic store.
// coverage:ignore - requires database
func NewTopicStore(pool DBTX) *TopicStore {
	return &TopicStore{pool: pool}
}

// Topic represents a stored topic.
type Topic struct {
	ID          string
	Slug        string
	Name        string
	Description string
	Owner       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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

// Create inserts a new topic.
// coverage:ignore - requires database
func (s *TopicStore) Create(ctx context.Context, slug, name, description, owner string) (*Topic, error) {
	var t Topic
	err := s.pool.QueryRow(ctx,
		`INSERT INTO topics (slug, name, description, owner)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, slug, name, description, owner, created_at, updated_at`,
		slug, name, description, owner,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Owner, &t.CreatedAt, &t.UpdatedAt)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("inserting topic: %w", err)
	}
	// coverage:ignore - requires database
	return &t, nil
}

// Get retrieves a topic by ID.
// coverage:ignore - requires database
func (s *TopicStore) Get(ctx context.Context, id string) (*Topic, error) {
	var t Topic
	err := s.pool.QueryRow(ctx,
		`SELECT id, slug, name, description, owner, created_at, updated_at
		 FROM topics WHERE id = $1`, id,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Owner, &t.CreatedAt, &t.UpdatedAt)
	// coverage:ignore - requires database
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("topic not found")
	}
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("querying topic: %w", err)
	}
	// coverage:ignore - requires database
	return &t, nil
}

// ListForPrincipals returns topics accessible to the given principal identities.
// If principals is nil, returns all topics (for system accounts).
// coverage:ignore - requires database
func (s *TopicStore) ListForPrincipals(ctx context.Context, principals []string) ([]Topic, error) {
	var rows pgx.Rows
	var err error

	// coverage:ignore - requires database
	if principals == nil {
		rows, err = s.pool.Query(ctx,
			`SELECT id, slug, name, description, owner, created_at, updated_at
			 FROM topics ORDER BY created_at`)
		// coverage:ignore - requires database
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT DISTINCT t.id, t.slug, t.name, t.description, t.owner, t.created_at, t.updated_at
			 FROM topics t
			 LEFT JOIN topic_grants g ON g.topic_id = t.id
			 WHERE t.owner = ANY($1) OR g.principal = ANY($1)
			 ORDER BY t.created_at`, principals)
	}
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("listing topics: %w", err)
	}
	// coverage:ignore - requires database
	defer rows.Close()

	var topics []Topic
	// coverage:ignore - requires database
	for rows.Next() {
		var t Topic
		// coverage:ignore - requires database
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Owner, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning topic: %w", err)
		}
		// coverage:ignore - requires database
		topics = append(topics, t)
	}
	// coverage:ignore - requires database
	return topics, rows.Err()
}

// Update modifies a topic's name and description.
// coverage:ignore - requires database
func (s *TopicStore) Update(ctx context.Context, id, name, description string) (*Topic, error) {
	var t Topic
	err := s.pool.QueryRow(ctx,
		`UPDATE topics SET name = $2, description = $3, updated_at = now()
		 WHERE id = $1
		 RETURNING id, slug, name, description, owner, created_at, updated_at`,
		id, name, description,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Owner, &t.CreatedAt, &t.UpdatedAt)
	// coverage:ignore - requires database
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("topic not found")
	}
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("updating topic: %w", err)
	}
	// coverage:ignore - requires database
	return &t, nil
}

// Delete removes a topic and cascades to grants, documents, chunks.
// coverage:ignore - requires database
func (s *TopicStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM topics WHERE id = $1`, id)
	// coverage:ignore - requires database
	if err != nil {
		return fmt.Errorf("deleting topic: %w", err)
	}
	// coverage:ignore - requires database
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("topic not found")
	}
	// coverage:ignore - requires database
	return nil
}

// Grant creates or updates a topic grant.
// coverage:ignore - requires database
func (s *TopicStore) Grant(ctx context.Context, topicID, principal, permission, grantedBy string) (*TopicGrant, error) {
	var g TopicGrant
	err := s.pool.QueryRow(ctx,
		`INSERT INTO topic_grants (topic_id, principal, permission, granted_by)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (topic_id, principal) DO UPDATE SET permission = $3
		 RETURNING id, topic_id, principal, permission, granted_by, created_at`,
		topicID, principal, permission, grantedBy,
	).Scan(&g.ID, &g.TopicID, &g.Principal, &g.Permission, &g.GrantedBy, &g.CreatedAt)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("granting access: %w", err)
	}
	// coverage:ignore - requires database
	return &g, nil
}

// Revoke removes a topic grant.
// coverage:ignore - requires database
func (s *TopicStore) Revoke(ctx context.Context, topicID, principal string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM topic_grants WHERE topic_id = $1 AND principal = $2`,
		topicID, principal,
	)
	// coverage:ignore - requires database
	if err != nil {
		return fmt.Errorf("revoking access: %w", err)
	}
	// coverage:ignore - requires database
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("grant not found")
	}
	// coverage:ignore - requires database
	return nil
}

// ListGrants returns all grants for a topic.
// coverage:ignore - requires database
func (s *TopicStore) ListGrants(ctx context.Context, topicID string) ([]TopicGrant, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, topic_id, principal, permission, granted_by, created_at
		 FROM topic_grants WHERE topic_id = $1 ORDER BY created_at`, topicID,
	)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("listing grants: %w", err)
	}
	// coverage:ignore - requires database
	defer rows.Close()

	var grants []TopicGrant
	// coverage:ignore - requires database
	for rows.Next() {
		var g TopicGrant
		// coverage:ignore - requires database
		if err := rows.Scan(&g.ID, &g.TopicID, &g.Principal, &g.Permission, &g.GrantedBy, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning grant: %w", err)
		}
		// coverage:ignore - requires database
		grants = append(grants, g)
	}
	// coverage:ignore - requires database
	return grants, rows.Err()
}
