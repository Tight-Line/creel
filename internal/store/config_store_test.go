package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/crypto"
)

const testEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// setupConfigTestDB creates a pool, ensures schema, runs migrations, and
// cleans all config tables. Skips if CREEL_POSTGRES_HOST is not set.
func setupConfigTestDB(t *testing.T) *pgxpool.Pool {
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

	// Clean config tables (order matters due to FK constraints).
	// Do NOT delete topics; ON DELETE SET NULL handles config FK references,
	// and deleting topics interferes with concurrent dbtest package runs.
	for _, table := range []string{"llm_configs", "embedding_configs", "extraction_prompt_configs", "api_key_configs"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("cleaning %s: %v", table, err)
		}
	}

	return pool
}

// ---------------------------------------------------------------------------
// APIKeyConfigStore integration tests
// ---------------------------------------------------------------------------

func TestAPIKeyConfigStore_Integration_CRUD(t *testing.T) {
	pool := setupConfigTestDB(t)
	enc, err := crypto.NewEncryptor(testEncryptionKey)
	if err != nil {
		t.Fatalf("creating encryptor: %v", err)
	}
	s := NewAPIKeyConfigStore(pool, enc)
	ctx := context.Background()

	// Create.
	c, err := s.Create(ctx, "openai-test", "openai", []byte("sk-test-key-123"), false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Name != "openai-test" || c.Provider != "openai" {
		t.Errorf("unexpected config: %+v", c)
	}

	// Get.
	got, err := s.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "openai-test" {
		t.Errorf("Get name = %q, want openai-test", got.Name)
	}

	// GetDecrypted.
	decrypted, err := s.GetDecrypted(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetDecrypted: %v", err)
	}
	if string(decrypted) != "sk-test-key-123" {
		t.Errorf("GetDecrypted = %q, want sk-test-key-123", string(decrypted))
	}

	// List.
	configs, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(configs) != 1 {
		t.Errorf("List len = %d, want 1", len(configs))
	}

	// Update without key.
	updated, err := s.Update(ctx, c.ID, "openai-renamed", "", nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "openai-renamed" {
		t.Errorf("Update name = %q, want openai-renamed", updated.Name)
	}

	// Update with new key.
	updated2, err := s.Update(ctx, c.ID, "", "", []byte("sk-new-key-456"))
	if err != nil {
		t.Fatalf("Update with key: %v", err)
	}
	if updated2.Name != "openai-renamed" {
		t.Errorf("name should be unchanged: %q", updated2.Name)
	}
	decrypted2, err := s.GetDecrypted(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetDecrypted after update: %v", err)
	}
	if string(decrypted2) != "sk-new-key-456" {
		t.Errorf("GetDecrypted = %q, want sk-new-key-456", string(decrypted2))
	}

	// Delete.
	if err := s.Delete(ctx, c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = s.Get(ctx, c.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestAPIKeyConfigStore_Integration_SetDefault(t *testing.T) {
	pool := setupConfigTestDB(t)
	enc, err := crypto.NewEncryptor(testEncryptionKey)
	if err != nil {
		t.Fatalf("creating encryptor: %v", err)
	}
	s := NewAPIKeyConfigStore(pool, enc)
	ctx := context.Background()

	// Create two configs.
	c1, err := s.Create(ctx, "key-1", "openai", []byte("k1"), true)
	if err != nil {
		t.Fatalf("Create c1: %v", err)
	}
	if !c1.IsDefault {
		t.Error("c1 should be default")
	}

	c2, err := s.Create(ctx, "key-2", "anthropic", []byte("k2"), false)
	if err != nil {
		t.Fatalf("Create c2: %v", err)
	}

	// Set c2 as default; c1 should lose default.
	c2d, err := s.SetDefault(ctx, c2.ID)
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !c2d.IsDefault {
		t.Error("c2 should be default after SetDefault")
	}

	c1r, err := s.Get(ctx, c1.ID)
	if err != nil {
		t.Fatalf("Get c1: %v", err)
	}
	if c1r.IsDefault {
		t.Error("c1 should no longer be default")
	}
}

// ---------------------------------------------------------------------------
// LLMConfigStore integration tests
// ---------------------------------------------------------------------------

func TestLLMConfigStore_Integration_CRUD(t *testing.T) {
	pool := setupConfigTestDB(t)
	enc, err := crypto.NewEncryptor(testEncryptionKey)
	if err != nil {
		t.Fatalf("creating encryptor: %v", err)
	}

	// LLM configs require an api_key_config FK.
	akStore := NewAPIKeyConfigStore(pool, enc)
	ctx := context.Background()
	ak, err := akStore.Create(ctx, "llm-test-key", "openai", []byte("key"), false)
	if err != nil {
		t.Fatalf("creating api key config: %v", err)
	}

	s := NewLLMConfigStore(pool)

	// Create.
	params := map[string]string{"temperature": "0.7"}
	c, err := s.Create(ctx, "gpt4o", "openai", "gpt-4o", params, ak.ID, false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Name != "gpt4o" || c.Provider != "openai" || c.Model != "gpt-4o" {
		t.Errorf("unexpected config: %+v", c)
	}
	if c.Parameters["temperature"] != "0.7" {
		t.Errorf("parameters = %v, want temperature=0.7", c.Parameters)
	}

	// Get.
	got, err := s.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.APIKeyConfigID != ak.ID {
		t.Errorf("APIKeyConfigID = %q, want %q", got.APIKeyConfigID, ak.ID)
	}

	// List.
	configs, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(configs) != 1 {
		t.Errorf("List len = %d, want 1", len(configs))
	}

	// Update without params.
	updated, err := s.Update(ctx, c.ID, "gpt4o-renamed", "", "", nil, "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "gpt4o-renamed" {
		t.Errorf("Update name = %q, want gpt4o-renamed", updated.Name)
	}

	// Update with params.
	newParams := map[string]string{"temperature": "0.9", "max_tokens": "1000"}
	updated2, err := s.Update(ctx, c.ID, "", "", "", newParams, "")
	if err != nil {
		t.Fatalf("Update with params: %v", err)
	}
	if updated2.Parameters["temperature"] != "0.9" {
		t.Errorf("parameters = %v after update", updated2.Parameters)
	}

	// GetDefault (none set).
	def, err := s.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if def != nil {
		t.Error("expected nil default")
	}

	// SetDefault.
	defC, err := s.SetDefault(ctx, c.ID)
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !defC.IsDefault {
		t.Error("should be default after SetDefault")
	}

	// GetDefault.
	def2, err := s.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if def2 == nil || def2.ID != c.ID {
		t.Error("GetDefault should return the default config")
	}

	// Delete.
	if err := s.Delete(ctx, c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = s.Get(ctx, c.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestLLMConfigStore_Integration_CreateWithDefault(t *testing.T) {
	pool := setupConfigTestDB(t)
	enc, err := crypto.NewEncryptor(testEncryptionKey)
	if err != nil {
		t.Fatalf("creating encryptor: %v", err)
	}
	akStore := NewAPIKeyConfigStore(pool, enc)
	ctx := context.Background()
	ak, err := akStore.Create(ctx, "llm-def-key", "openai", []byte("key"), false)
	if err != nil {
		t.Fatalf("creating api key config: %v", err)
	}

	s := NewLLMConfigStore(pool)

	// Create first as default.
	c1, err := s.Create(ctx, "llm-def-1", "openai", "gpt-4o", nil, ak.ID, true)
	if err != nil {
		t.Fatalf("Create c1: %v", err)
	}
	if !c1.IsDefault {
		t.Error("c1 should be default")
	}

	// Create second as default; first should lose it.
	c2, err := s.Create(ctx, "llm-def-2", "openai", "gpt-4o-mini", nil, ak.ID, true)
	if err != nil {
		t.Fatalf("Create c2: %v", err)
	}
	if !c2.IsDefault {
		t.Error("c2 should be default")
	}
	c1r, err := s.Get(ctx, c1.ID)
	if err != nil {
		t.Fatalf("Get c1: %v", err)
	}
	if c1r.IsDefault {
		t.Error("c1 should no longer be default")
	}
}

// ---------------------------------------------------------------------------
// EmbeddingConfigStore integration tests
// ---------------------------------------------------------------------------

func TestEmbeddingConfigStore_Integration_CRUD(t *testing.T) {
	pool := setupConfigTestDB(t)
	enc, err := crypto.NewEncryptor(testEncryptionKey)
	if err != nil {
		t.Fatalf("creating encryptor: %v", err)
	}
	akStore := NewAPIKeyConfigStore(pool, enc)
	ctx := context.Background()
	ak, err := akStore.Create(ctx, "emb-test-key", "openai", []byte("key"), false)
	if err != nil {
		t.Fatalf("creating api key config: %v", err)
	}

	s := NewEmbeddingConfigStore(pool)

	// Create.
	c, err := s.Create(ctx, "ada-small", "openai", "text-embedding-3-small", 1536, ak.ID, false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Dimensions != 1536 {
		t.Errorf("Dimensions = %d, want 1536", c.Dimensions)
	}

	// Get.
	got, err := s.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Model != "text-embedding-3-small" {
		t.Errorf("Model = %q", got.Model)
	}

	// List.
	configs, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(configs) != 1 {
		t.Errorf("List len = %d, want 1", len(configs))
	}

	// Update.
	updated, err := s.Update(ctx, c.ID, "ada-renamed", "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "ada-renamed" {
		t.Errorf("Update name = %q, want ada-renamed", updated.Name)
	}

	// GetDefault (none).
	def, err := s.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if def != nil {
		t.Error("expected nil default")
	}

	// SetDefault + GetDefault.
	defC, err := s.SetDefault(ctx, c.ID)
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !defC.IsDefault {
		t.Error("should be default")
	}
	def2, err := s.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if def2 == nil || def2.ID != c.ID {
		t.Error("GetDefault should return the config")
	}

	// Delete.
	if err := s.Delete(ctx, c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ExtractionPromptConfigStore integration tests
// ---------------------------------------------------------------------------

func TestExtractionPromptConfigStore_Integration_CRUD(t *testing.T) {
	pool := setupConfigTestDB(t)
	s := NewExtractionPromptConfigStore(pool)
	ctx := context.Background()

	// Create.
	c, err := s.Create(ctx, "default-extraction", "Extract key facts", "Standard extraction", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Prompt != "Extract key facts" {
		t.Errorf("Prompt = %q", c.Prompt)
	}

	// Get.
	got, err := s.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Standard extraction" {
		t.Errorf("Description = %q", got.Description)
	}

	// List.
	configs, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(configs) != 1 {
		t.Errorf("List len = %d, want 1", len(configs))
	}

	// Update.
	updated, err := s.Update(ctx, c.ID, "renamed", "Updated prompt", "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "renamed" || updated.Prompt != "Updated prompt" {
		t.Errorf("Update: %+v", updated)
	}

	// GetDefault (none).
	def, err := s.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if def != nil {
		t.Error("expected nil default")
	}

	// SetDefault + GetDefault.
	defC, err := s.SetDefault(ctx, c.ID)
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !defC.IsDefault {
		t.Error("should be default")
	}
	def2, err := s.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if def2 == nil || def2.ID != c.ID {
		t.Error("GetDefault should return the config")
	}

	// Delete.
	if err := s.Delete(ctx, c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Delete not-found tests (exercise the RowsAffected==0 branch)
// ---------------------------------------------------------------------------

func TestAPIKeyConfigStore_Integration_DeleteNotFound(t *testing.T) {
	pool := setupConfigTestDB(t)
	enc, _ := crypto.NewEncryptor(testEncryptionKey)
	s := NewAPIKeyConfigStore(pool, enc)
	err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestLLMConfigStore_Integration_DeleteNotFound(t *testing.T) {
	pool := setupConfigTestDB(t)
	s := NewLLMConfigStore(pool)
	err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestEmbeddingConfigStore_Integration_DeleteNotFound(t *testing.T) {
	pool := setupConfigTestDB(t)
	s := NewEmbeddingConfigStore(pool)
	err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestExtractionPromptConfigStore_Integration_DeleteNotFound(t *testing.T) {
	pool := setupConfigTestDB(t)
	s := NewExtractionPromptConfigStore(pool)
	err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

// ---------------------------------------------------------------------------
// Topic config binding integration tests
// ---------------------------------------------------------------------------

func TestTopicStore_Integration_ConfigBinding(t *testing.T) {
	pool := setupConfigTestDB(t)
	enc, _ := crypto.NewEncryptor(testEncryptionKey)
	ctx := context.Background()

	// Clean up stale test topics from previous runs.
	for _, slug := range []string{"bound-topic", "unbound-topic"} {
		_, _ = pool.Exec(ctx, `DELETE FROM topics WHERE slug = $1`, slug)
	}

	// Create prerequisite configs.
	akStore := NewAPIKeyConfigStore(pool, enc)
	ak, err := akStore.Create(ctx, "topic-bind-key", "openai", []byte("key"), false)
	if err != nil {
		t.Fatalf("creating api key config: %v", err)
	}

	llmStore := NewLLMConfigStore(pool)
	llm, err := llmStore.Create(ctx, "topic-bind-llm", "openai", "gpt-4o", nil, ak.ID, false)
	if err != nil {
		t.Fatalf("creating llm config: %v", err)
	}

	embStore := NewEmbeddingConfigStore(pool)
	emb, err := embStore.Create(ctx, "topic-bind-emb", "openai", "text-embedding-3-small", 1536, ak.ID, false)
	if err != nil {
		t.Fatalf("creating embedding config: %v", err)
	}

	promptStore := NewExtractionPromptConfigStore(pool)
	prompt, err := promptStore.Create(ctx, "topic-bind-prompt", "Extract facts", "", false)
	if err != nil {
		t.Fatalf("creating extraction prompt config: %v", err)
	}

	// Create topic with config bindings.
	topicStore := NewTopicStore(pool)
	topic, err := topicStore.Create(ctx, "bound-topic", "Bound Topic", "", "system:test", &llm.ID, &emb.ID, &prompt.ID, false, nil)
	if err != nil {
		t.Fatalf("Create topic: %v", err)
	}
	if topic.LLMConfigID == nil || *topic.LLMConfigID != llm.ID {
		t.Errorf("LLMConfigID = %v, want %s", topic.LLMConfigID, llm.ID)
	}
	if topic.EmbeddingConfigID == nil || *topic.EmbeddingConfigID != emb.ID {
		t.Errorf("EmbeddingConfigID = %v, want %s", topic.EmbeddingConfigID, emb.ID)
	}
	if topic.ExtractionPromptConfigID == nil || *topic.ExtractionPromptConfigID != prompt.ID {
		t.Errorf("ExtractionPromptConfigID = %v, want %s", topic.ExtractionPromptConfigID, prompt.ID)
	}

	// Create topic without config bindings.
	topic2, err := topicStore.Create(ctx, "unbound-topic", "Unbound", "", "system:test", nil, nil, nil, false, nil)
	if err != nil {
		t.Fatalf("Create topic2: %v", err)
	}
	if topic2.LLMConfigID != nil {
		t.Error("expected nil LLMConfigID")
	}

	// Update topic to bind configs.
	updated, err := topicStore.Update(ctx, topic2.ID, "Unbound Updated", "", &llm.ID, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Update topic2: %v", err)
	}
	if updated.LLMConfigID == nil || *updated.LLMConfigID != llm.ID {
		t.Error("expected LLMConfigID to be set after update")
	}

	// Verify ListForPrincipals returns config IDs.
	topics, err := topicStore.ListForPrincipals(ctx, nil)
	if err != nil {
		t.Fatalf("ListForPrincipals: %v", err)
	}
	if len(topics) < 2 {
		t.Fatalf("expected at least 2 topics, got %d", len(topics))
	}
}
