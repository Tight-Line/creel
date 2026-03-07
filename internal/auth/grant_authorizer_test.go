package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type mockGrantStore struct {
	grants   []Grant
	owners   map[string]string // topicID -> owner
	grantErr error
	ownerErr error
}

func (m *mockGrantStore) GrantsForPrincipal(_ context.Context, principals []string) ([]Grant, error) {
	if m.grantErr != nil {
		return nil, m.grantErr
	}
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
	if m.ownerErr != nil {
		return "", m.ownerErr
	}
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

func TestPermissionLevel(t *testing.T) {
	tests := []struct {
		action Action
		want   int
	}{
		{ActionRead, 1},
		{ActionWrite, 2},
		{ActionAdmin, 3},
		{Action("bogus"), 0},
	}
	for _, tt := range tests {
		got := PermissionLevel(tt.action)
		if got != tt.want {
			t.Errorf("PermissionLevel(%q) = %d, want %d", tt.action, got, tt.want)
		}
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

func TestGrantAuthorizer_Check_TopicOwnerError(t *testing.T) {
	store := &mockGrantStore{
		ownerErr: errors.New("db connection lost"),
	}
	authz := NewGrantAuthorizer(store)

	err := authz.Check(context.Background(), &Principal{ID: "user:alice"}, "topic-1", ActionRead)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "checking topic owner") {
		t.Errorf("expected 'checking topic owner' in error, got: %v", err)
	}
}

func TestGrantAuthorizer_Check_GrantsError(t *testing.T) {
	store := &mockGrantStore{
		owners:   map[string]string{"topic-1": "user:alice"},
		grantErr: errors.New("db timeout"),
	}
	authz := NewGrantAuthorizer(store)

	// Principal is not the owner, so it falls through to GrantsForPrincipal.
	err := authz.Check(context.Background(), &Principal{ID: "user:bob"}, "topic-1", ActionRead)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fetching grants") {
		t.Errorf("expected 'fetching grants' in error, got: %v", err)
	}
}

func TestGrantAuthorizer_Check_PermissionDenied(t *testing.T) {
	store := &mockGrantStore{
		owners: map[string]string{"topic-1": "user:alice"},
		grants: []Grant{
			{TopicID: "topic-1", Principal: "user:bob", Permission: ActionRead},
		},
	}
	authz := NewGrantAuthorizer(store)

	// Bob has read but requests write; should be denied.
	err := authz.Check(context.Background(), &Principal{ID: "user:bob"}, "topic-1", ActionWrite)
	if err == nil {
		t.Fatal("expected permission denied error, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected 'permission denied' in error, got: %v", err)
	}
}

func TestGrantAuthorizer_AccessibleTopics_Error(t *testing.T) {
	store := &mockGrantStore{
		grantErr: errors.New("db unavailable"),
	}
	authz := NewGrantAuthorizer(store)

	_, err := authz.AccessibleTopics(context.Background(), &Principal{ID: "user:bob"}, ActionRead)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fetching grants") {
		t.Errorf("expected 'fetching grants' in error, got: %v", err)
	}
}

func TestGrantAuthorizer_AccessibleTopics_FiltersByLevel(t *testing.T) {
	store := &mockGrantStore{
		grants: []Grant{
			{TopicID: "topic-1", Principal: "user:bob", Permission: ActionRead},
			{TopicID: "topic-2", Principal: "user:bob", Permission: ActionWrite},
			{TopicID: "topic-3", Principal: "user:bob", Permission: ActionAdmin},
		},
	}
	authz := NewGrantAuthorizer(store)

	// Request admin level; only topic-3 qualifies.
	topics, err := authz.AccessibleTopics(context.Background(), &Principal{ID: "user:bob"}, ActionAdmin)
	if err != nil {
		t.Fatalf("AccessibleTopics: %v", err)
	}
	if len(topics) != 1 || topics[0] != "topic-3" {
		t.Errorf("expected [topic-3], got %v", topics)
	}

	// Request read level; all three qualify.
	topics, err = authz.AccessibleTopics(context.Background(), &Principal{ID: "user:bob"}, ActionRead)
	if err != nil {
		t.Fatalf("AccessibleTopics: %v", err)
	}
	if len(topics) != 3 {
		t.Errorf("expected 3 topics, got %d: %v", len(topics), topics)
	}
}
