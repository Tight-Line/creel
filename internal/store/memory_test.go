package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/config"
)

// setupMemoryTestDB creates a pool, ensures schema, runs migrations, and
// cleans the memories table. Skips if CREEL_POSTGRES_HOST is not set.
func setupMemoryTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()
	if err := EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if err := RunMigrations(pgCfg.URL(), "../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgCfg.URL())
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, "DELETE FROM memories"); err != nil {
		t.Fatalf("cleaning memories: %v", err)
	}

	return pool
}

func TestMemoryStore_Integration_CRUD(t *testing.T) {
	pool := setupMemoryTestDB(t)
	ctx := context.Background()
	s := NewMemoryStore(pool)

	// Create
	subj := "user"
	pred := "prefers"
	obj := "concise answers"
	m, err := s.Create(ctx, &Memory{
		Principal: "user:alice",
		Scope:     "default",
		Content:   "User prefers concise answers",
		Subject:   &subj,
		Predicate: &pred,
		Object:    &obj,
		Metadata:  map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatalf("creating memory: %v", err)
	}
	if m.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if m.Status != "active" {
		t.Fatalf("expected status 'active', got %q", m.Status)
	}
	if m.Subject == nil || *m.Subject != "user" {
		t.Fatalf("expected subject 'user', got %v", m.Subject)
	}

	// Get
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("getting memory: %v", err)
	}
	if got.Content != m.Content {
		t.Fatalf("expected content %q, got %q", m.Content, got.Content)
	}

	// GetByScope
	memories, err := s.GetByScope(ctx, "user:alice", "default")
	if err != nil {
		t.Fatalf("getting by scope: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}

	// Update
	updated, err := s.Update(ctx, m.ID, "Updated content", map[string]any{"source": "updated"})
	if err != nil {
		t.Fatalf("updating memory: %v", err)
	}
	if updated.Content != "Updated content" {
		t.Fatalf("expected updated content, got %q", updated.Content)
	}

	// Create a second memory in a different scope
	m2, err := s.Create(ctx, &Memory{
		Principal: "user:alice",
		Scope:     "work",
		Content:   "Work memory",
	})
	if err != nil {
		t.Fatalf("creating second memory: %v", err)
	}

	// ListScopes
	scopes, err := s.ListScopes(ctx, "user:alice")
	if err != nil {
		t.Fatalf("listing scopes: %v", err)
	}
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}

	// ListByScope (active only)
	all, err := s.ListByScope(ctx, "user:alice", "default", false)
	if err != nil {
		t.Fatalf("listing by scope: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 active memory, got %d", len(all))
	}

	// Invalidate
	if err := s.Invalidate(ctx, m.ID); err != nil {
		t.Fatalf("invalidating memory: %v", err)
	}

	// Verify invalidated memory is excluded from active list
	active, err := s.ListByScope(ctx, "user:alice", "default", false)
	if err != nil {
		t.Fatalf("listing active: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected 0 active memories, got %d", len(active))
	}

	// Verify invalidated memory is included with flag
	withInvalidated, err := s.ListByScope(ctx, "user:alice", "default", true)
	if err != nil {
		t.Fatalf("listing with invalidated: %v", err)
	}
	if len(withInvalidated) != 1 {
		t.Fatalf("expected 1 memory with invalidated, got %d", len(withInvalidated))
	}
	if withInvalidated[0].Status != "invalidated" {
		t.Fatalf("expected status 'invalidated', got %q", withInvalidated[0].Status)
	}
	if withInvalidated[0].InvalidatedAt == nil {
		t.Fatal("expected invalidated_at to be set")
	}

	// SetEmbeddingID
	if err := s.SetEmbeddingID(ctx, m2.ID, "emb_123"); err != nil {
		t.Fatalf("setting embedding ID: %v", err)
	}

	// GetWithEmbedding
	withEmb, err := s.GetWithEmbedding(ctx, "user:alice", "work")
	if err != nil {
		t.Fatalf("getting with embedding: %v", err)
	}
	if len(withEmb) != 1 {
		t.Fatalf("expected 1 memory with embedding, got %d", len(withEmb))
	}
	if withEmb[0].EmbeddingID == nil || *withEmb[0].EmbeddingID != "emb_123" {
		t.Fatalf("expected embedding ID 'emb_123', got %v", withEmb[0].EmbeddingID)
	}

	// EmbeddingIDsByPrincipalScope
	embIDs, err := s.EmbeddingIDsByPrincipalScope(ctx, "user:alice", "work")
	if err != nil {
		t.Fatalf("getting embedding IDs: %v", err)
	}
	if len(embIDs) != 1 || embIDs[0] != "emb_123" {
		t.Fatalf("expected ['emb_123'], got %v", embIDs)
	}

	// GetMultiple
	multi, err := s.GetMultiple(ctx, []string{m.ID, m2.ID})
	if err != nil {
		t.Fatalf("getting multiple: %v", err)
	}
	if len(multi) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(multi))
	}

	// GetByEmbeddingIDs
	byEmb, err := s.GetByEmbeddingIDs(ctx, []string{"emb_123"})
	if err != nil {
		t.Fatalf("getting by embedding IDs: %v", err)
	}
	if len(byEmb) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(byEmb))
	}
	if byEmb["emb_123"].ID != m2.ID {
		t.Fatalf("expected memory ID %s, got %s", m2.ID, byEmb["emb_123"].ID)
	}

	// Get not found
	_, err = s.Get(ctx, "00000000-0000-0000-0000-000000000000")
	expectErr(t, err, "memory not found")

	// Invalidate not found
	err = s.Invalidate(ctx, "00000000-0000-0000-0000-000000000000")
	expectErr(t, err, "memory not found")

	// Update not found
	_, err = s.Update(ctx, "00000000-0000-0000-0000-000000000000", "content", nil)
	expectErr(t, err, "memory not found")
}

func TestMemoryStore_Integration_NilMetadata(t *testing.T) {
	pool := setupMemoryTestDB(t)
	ctx := context.Background()
	s := NewMemoryStore(pool)

	// Create with nil metadata (should default to empty object)
	m, err := s.Create(ctx, &Memory{
		Principal: "user:bob",
		Scope:     "test",
		Content:   "test memory",
	})
	if err != nil {
		t.Fatalf("creating memory: %v", err)
	}
	if m.Metadata == nil {
		t.Fatal("expected non-nil metadata")
	}
}

func TestMemoryStore_Integration_GetByScopeEmpty(t *testing.T) {
	pool := setupMemoryTestDB(t)
	ctx := context.Background()
	s := NewMemoryStore(pool)

	memories, err := s.GetByScope(ctx, "nonexistent", "scope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(memories) != 0 {
		t.Fatalf("expected 0 memories, got %d", len(memories))
	}
}

func TestMemoryStore_Integration_ListScopesEmpty(t *testing.T) {
	pool := setupMemoryTestDB(t)
	ctx := context.Background()
	s := NewMemoryStore(pool)

	scopes, err := s.ListScopes(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 0 {
		t.Fatalf("expected 0 scopes, got %d", len(scopes))
	}
}
