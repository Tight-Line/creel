package store

import (
	"context"
	"fmt"

	"github.com/Tight-Line/creel/internal/auth"
)

// GrantStore provides access to topic grants and ownership from PostgreSQL.
type GrantStore struct {
	pool DBTX
}

// NewGrantStore creates a new grant store.
func NewGrantStore(pool DBTX) *GrantStore {
	return &GrantStore{pool: pool}
}

// GrantsForPrincipal returns all grants matching any of the given principals.
func (s *GrantStore) GrantsForPrincipal(ctx context.Context, principals []string) ([]auth.Grant, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT topic_id, principal, permission
		 FROM topic_grants
		 WHERE principal = ANY($1)`,
		principals,
	)
	if err != nil {
		return nil, fmt.Errorf("querying grants: %w", err)
	}
	defer rows.Close()

	var grants []auth.Grant
	for rows.Next() {
		var g auth.Grant
		if err := rows.Scan(&g.TopicID, &g.Principal, &g.Permission); err != nil {
			return nil, fmt.Errorf("scanning grant: %w", err)
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

// TopicOwner returns the owner of the given topic.
func (s *GrantStore) TopicOwner(ctx context.Context, topicID string) (string, error) {
	var owner string
	err := s.pool.QueryRow(ctx,
		`SELECT owner FROM topics WHERE id = $1`, topicID,
	).Scan(&owner)
	if err != nil {
		return "", fmt.Errorf("querying topic owner: %w", err)
	}
	return owner, nil
}

// TopicIDsByOwner returns the IDs of all topics owned by the given principal.
func (s *GrantStore) TopicIDsByOwner(ctx context.Context, ownerID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM topics WHERE owner = $1`, ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying owned topics: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning owned topic: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
