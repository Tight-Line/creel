package auth

import (
	"context"
	"testing"
)

func TestPrincipalContext(t *testing.T) {
	p := &Principal{ID: "user:alice", Groups: []string{"group:eng"}}
	ctx := ContextWithPrincipal(context.Background(), p)

	got := PrincipalFromContext(ctx)
	if got == nil || got.ID != "user:alice" {
		t.Errorf("PrincipalFromContext = %v, want user:alice", got)
	}
	if len(got.Groups) != 1 || got.Groups[0] != "group:eng" {
		t.Errorf("Groups = %v, want [group:eng]", got.Groups)
	}
}

func TestPrincipalFromContext_Nil(t *testing.T) {
	p := PrincipalFromContext(context.Background())
	if p != nil {
		t.Errorf("expected nil, got %v", p)
	}
}
