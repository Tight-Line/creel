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
// coverage:ignore - requires database
func NewGrantStore(pool DBTX) *GrantStore {
	return &GrantStore{pool: pool}
}

// GrantsForPrincipal returns all grants matching any of the given principals.
// coverage:ignore - requires database
func (s *GrantStore) GrantsForPrincipal(ctx context.Context, principals []string) ([]auth.Grant, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT topic_id, principal, permission
		 FROM topic_grants
		 WHERE principal = ANY($1)`,
		principals,
	)
	// coverage:ignore - requires database
	if err != nil {
		return nil, fmt.Errorf("querying grants: %w", err)
	}
	// coverage:ignore - requires database
	defer rows.Close()

	var grants []auth.Grant
	// coverage:ignore - requires database
	for rows.Next() {
		var g auth.Grant
		// coverage:ignore - requires database
		if err := rows.Scan(&g.TopicID, &g.Principal, &g.Permission); err != nil {
			return nil, fmt.Errorf("scanning grant: %w", err)
		}
		// coverage:ignore - requires database
		grants = append(grants, g)
	}
	// coverage:ignore - requires database
	return grants, rows.Err()
}

// TopicOwner returns the owner of the given topic.
// coverage:ignore - requires database
func (s *GrantStore) TopicOwner(ctx context.Context, topicID string) (string, error) {
	var owner string
	err := s.pool.QueryRow(ctx,
		`SELECT owner FROM topics WHERE id = $1`, topicID,
	).Scan(&owner)
	// coverage:ignore - requires database
	if err != nil {
		return "", fmt.Errorf("querying topic owner: %w", err)
	}
	// coverage:ignore - requires database
	return owner, nil
}
