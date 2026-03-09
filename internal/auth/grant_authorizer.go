package auth

import (
	"context"
	"fmt"
)

// Grant represents a permission grant on a topic.
type Grant struct {
	TopicID    string
	Principal  string
	Permission Action
}

// GrantStore provides access to topic grants and ownership.
type GrantStore interface {
	// GrantsForPrincipal returns all grants matching the principal ID or any of its groups.
	GrantsForPrincipal(ctx context.Context, principals []string) ([]Grant, error)

	// TopicOwner returns the owner of a topic.
	TopicOwner(ctx context.Context, topicID string) (string, error)

	// TopicIDsByOwner returns the IDs of all topics owned by the given principal.
	TopicIDsByOwner(ctx context.Context, ownerID string) ([]string, error)
}

// GrantAuthorizer implements Authorizer using topic_grants and topic ownership.
type GrantAuthorizer struct {
	store GrantStore
}

// NewGrantAuthorizer creates an authorizer backed by the given grant store.
func NewGrantAuthorizer(store GrantStore) *GrantAuthorizer {
	return &GrantAuthorizer{store: store}
}

// Check returns nil if the principal has sufficient permission on the topic.
func (a *GrantAuthorizer) Check(ctx context.Context, principal *Principal, topicID string, action Action) error {
	// Topic owner always has implicit admin.
	owner, err := a.store.TopicOwner(ctx, topicID)
	if err != nil {
		return fmt.Errorf("checking topic owner: %w", err)
	}
	if owner == principal.ID {
		return nil
	}

	// Check grants for the principal and its groups.
	grants, err := a.store.GrantsForPrincipal(ctx, a.allIdentities(principal))
	if err != nil {
		return fmt.Errorf("fetching grants: %w", err)
	}

	requiredLevel := PermissionLevel(action)
	for _, g := range grants {
		if g.TopicID == topicID && PermissionLevel(g.Permission) >= requiredLevel {
			return nil
		}
	}

	return fmt.Errorf("permission denied: %s on topic %s", action, topicID)
}

// AccessibleTopics returns topic IDs the principal can access at the given level.
// Owned topics are always included (ownership implies admin).
func (a *GrantAuthorizer) AccessibleTopics(ctx context.Context, principal *Principal, minAction Action) ([]string, error) {
	grants, err := a.store.GrantsForPrincipal(ctx, a.allIdentities(principal))
	if err != nil {
		return nil, fmt.Errorf("fetching grants: %w", err)
	}

	seen := make(map[string]bool)
	var result []string

	// Owned topics have implicit admin access.
	owned, err := a.store.TopicIDsByOwner(ctx, principal.ID)
	if err != nil {
		return nil, fmt.Errorf("fetching owned topics: %w", err)
	}
	for _, id := range owned {
		seen[id] = true
		result = append(result, id)
	}

	minLevel := PermissionLevel(minAction)
	for _, g := range grants {
		if PermissionLevel(g.Permission) >= minLevel && !seen[g.TopicID] {
			seen[g.TopicID] = true
			result = append(result, g.TopicID)
		}
	}

	return result, nil
}

func (a *GrantAuthorizer) allIdentities(p *Principal) []string {
	ids := make([]string, 0, 1+len(p.Groups))
	ids = append(ids, p.ID)
	ids = append(ids, p.Groups...)
	return ids
}
