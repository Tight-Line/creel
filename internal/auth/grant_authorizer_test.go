package auth

import (
	"context"
	"testing"
)

type mockGrantStore struct {
	grants []Grant
	owners map[string]string // topicID -> owner
}

func (m *mockGrantStore) GrantsForPrincipal(_ context.Context, principals []string) ([]Grant, error) {
	pset := make(map[string]bool, len(principals))
	for _, p := range principals {
		pset[p] = true
	}
	var result []Grant
	for _, g := range m.grants {
		if pset[g.Principal] {
			result = append(result, g)
		}
	}
	return result, nil
}

func (m *mockGrantStore) TopicOwner(_ context.Context, topicID string) (string, error) {
	return m.owners[topicID], nil
}

func TestGrantAuthorizer_OwnerImplicitAdmin(t *testing.T) {
	store := &mockGrantStore{
		owners: map[string]string{"topic-1": "user:alice"},
	}
	authz := NewGrantAuthorizer(store)

	principal := &Principal{ID: "user:alice"}
	err := authz.Check(context.Background(), principal, "topic-1", ActionAdmin)
	if err != nil {
		t.Errorf("owner should have implicit admin: %v", err)
	}
}

func TestGrantAuthorizer_DirectGrant(t *testing.T) {
	store := &mockGrantStore{
		owners: map[string]string{"topic-1": "user:alice"},
		grants: []Grant{
			{TopicID: "topic-1", Principal: "user:bob", Permission: ActionWrite},
		},
	}
	authz := NewGrantAuthorizer(store)

	bob := &Principal{ID: "user:bob"}

	// Write should succeed.
	if err := authz.Check(context.Background(), bob, "topic-1", ActionWrite); err != nil {
		t.Errorf("bob should have write: %v", err)
	}

	// Read should succeed (write >= read).
	if err := authz.Check(context.Background(), bob, "topic-1", ActionRead); err != nil {
		t.Errorf("bob should have read via write: %v", err)
	}

	// Admin should fail.
	if err := authz.Check(context.Background(), bob, "topic-1", ActionAdmin); err == nil {
		t.Error("bob should not have admin")
	}
}

func TestGrantAuthorizer_GroupGrant(t *testing.T) {
	store := &mockGrantStore{
		owners: map[string]string{"topic-1": "user:alice"},
		grants: []Grant{
			{TopicID: "topic-1", Principal: "group:engineering", Permission: ActionRead},
		},
	}
	authz := NewGrantAuthorizer(store)

	bob := &Principal{ID: "user:bob", Groups: []string{"group:engineering"}}
	if err := authz.Check(context.Background(), bob, "topic-1", ActionRead); err != nil {
		t.Errorf("bob should have read via group: %v", err)
	}
}

func TestGrantAuthorizer_NoAccess(t *testing.T) {
	store := &mockGrantStore{
		owners: map[string]string{"topic-1": "user:alice"},
	}
	authz := NewGrantAuthorizer(store)

	stranger := &Principal{ID: "user:eve"}
	if err := authz.Check(context.Background(), stranger, "topic-1", ActionRead); err == nil {
		t.Error("eve should not have access")
	}
}

func TestGrantAuthorizer_AccessibleTopics(t *testing.T) {
	store := &mockGrantStore{
		grants: []Grant{
			{TopicID: "topic-1", Principal: "user:bob", Permission: ActionRead},
			{TopicID: "topic-2", Principal: "user:bob", Permission: ActionWrite},
			{TopicID: "topic-3", Principal: "group:eng", Permission: ActionAdmin},
		},
	}
	authz := NewGrantAuthorizer(store)

	bob := &Principal{ID: "user:bob", Groups: []string{"group:eng"}}
	topics, err := authz.AccessibleTopics(context.Background(), bob, ActionWrite)
	if err != nil {
		t.Fatalf("AccessibleTopics: %v", err)
	}

	// Should include topic-2 (write) and topic-3 (admin via group), but not topic-1 (read only).
	if len(topics) != 2 {
		t.Errorf("expected 2 topics, got %d: %v", len(topics), topics)
	}
}
