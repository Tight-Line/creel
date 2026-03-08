package server

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/crypto"
	"github.com/Tight-Line/creel/internal/store"
)

const testEncKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setupConfigServerTest(t *testing.T) (*ConfigServer, func()) {
	t.Helper()
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()
	if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if err := store.RunMigrations(pgCfg.URL(), "../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgCfg.URL())
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}

	// Clean config tables.
	for _, table := range []string{"topics", "llm_configs", "embedding_configs", "extraction_prompt_configs", "api_key_configs"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("cleaning %s: %v", table, err)
		}
	}

	enc, err := crypto.NewEncryptor(testEncKey)
	if err != nil {
		t.Fatalf("creating encryptor: %v", err)
	}

	srv := NewConfigServer(
		store.NewAPIKeyConfigStore(pool, enc),
		store.NewLLMConfigStore(pool),
		store.NewEmbeddingConfigStore(pool),
		store.NewExtractionPromptConfigStore(pool),
	)

	return srv, pool.Close
}

func TestConfigServer_Integration_APIKeyLifecycle(t *testing.T) {
	srv, cleanup := setupConfigServerTest(t)
	defer cleanup()
	ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{ID: "system:test", IsSystem: true})

	// Create.
	c, err := srv.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{
		Name: "test-key", Provider: "openai", ApiKey: "sk-test-123",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Name != "test-key" {
		t.Errorf("Name = %q", c.Name)
	}

	// Get.
	got, err := srv.GetAPIKeyConfig(ctx, &pb.GetAPIKeyConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Provider != "openai" {
		t.Errorf("Provider = %q", got.Provider)
	}

	// List.
	list, err := srv.ListAPIKeyConfigs(ctx, &pb.ListAPIKeyConfigsRequest{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Configs) != 1 {
		t.Errorf("List len = %d", len(list.Configs))
	}

	// Update (name only).
	upd, err := srv.UpdateAPIKeyConfig(ctx, &pb.UpdateAPIKeyConfigRequest{
		Id: c.Id, Name: "renamed",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Name != "renamed" {
		t.Errorf("Name = %q after update", upd.Name)
	}

	// Update (with new API key).
	upd, err = srv.UpdateAPIKeyConfig(ctx, &pb.UpdateAPIKeyConfigRequest{
		Id: c.Id, Name: "renamed-again", ApiKey: "sk-new-456",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Name != "renamed-again" {
		t.Errorf("Name = %q after update with key", upd.Name)
	}

	// SetDefault.
	def, err := srv.SetDefaultAPIKeyConfig(ctx, &pb.SetDefaultAPIKeyConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !def.IsDefault {
		t.Error("should be default")
	}

	// Delete.
	_, err = srv.DeleteAPIKeyConfig(ctx, &pb.DeleteAPIKeyConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestConfigServer_Integration_LLMLifecycle(t *testing.T) {
	srv, cleanup := setupConfigServerTest(t)
	defer cleanup()
	ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{ID: "system:test", IsSystem: true})

	// Create API key config first (FK requirement).
	ak, err := srv.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{
		Name: "llm-key", Provider: "openai", ApiKey: "sk-llm",
	})
	if err != nil {
		t.Fatalf("Create API key: %v", err)
	}

	// Create LLM config.
	c, err := srv.CreateLLMConfig(ctx, &pb.CreateLLMConfigRequest{
		Name: "gpt4o", Provider: "openai", Model: "gpt-4o",
		ApiKeyConfigId: ak.Id,
		Parameters:     map[string]string{"temperature": "0.7"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Parameters["temperature"] != "0.7" {
		t.Errorf("Parameters = %v", c.Parameters)
	}

	// Get.
	got, err := srv.GetLLMConfig(ctx, &pb.GetLLMConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ApiKeyConfigId != ak.Id {
		t.Errorf("ApiKeyConfigId = %q", got.ApiKeyConfigId)
	}

	// List.
	list, err := srv.ListLLMConfigs(ctx, &pb.ListLLMConfigsRequest{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Configs) != 1 {
		t.Errorf("List len = %d", len(list.Configs))
	}

	// Update.
	upd, err := srv.UpdateLLMConfig(ctx, &pb.UpdateLLMConfigRequest{
		Id: c.Id, Name: "renamed",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Name != "renamed" {
		t.Errorf("Name = %q", upd.Name)
	}

	// Update with params.
	upd2, err := srv.UpdateLLMConfig(ctx, &pb.UpdateLLMConfigRequest{
		Id: c.Id, Parameters: map[string]string{"temperature": "0.9"},
	})
	if err != nil {
		t.Fatalf("Update with params: %v", err)
	}
	if upd2.Parameters["temperature"] != "0.9" {
		t.Errorf("Parameters = %v", upd2.Parameters)
	}

	// SetDefault.
	def, err := srv.SetDefaultLLMConfig(ctx, &pb.SetDefaultLLMConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !def.IsDefault {
		t.Error("should be default")
	}

	// Delete.
	_, err = srv.DeleteLLMConfig(ctx, &pb.DeleteLLMConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestConfigServer_Integration_EmbeddingLifecycle(t *testing.T) {
	srv, cleanup := setupConfigServerTest(t)
	defer cleanup()
	ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{ID: "system:test", IsSystem: true})

	ak, err := srv.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{
		Name: "emb-key", Provider: "openai", ApiKey: "sk-emb",
	})
	if err != nil {
		t.Fatalf("Create API key: %v", err)
	}

	c, err := srv.CreateEmbeddingConfig(ctx, &pb.CreateEmbeddingConfigRequest{
		Name: "ada", Provider: "openai", Model: "text-embedding-3-small",
		Dimensions: 1536, ApiKeyConfigId: ak.Id,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Dimensions != 1536 {
		t.Errorf("Dimensions = %d", c.Dimensions)
	}

	got, err := srv.GetEmbeddingConfig(ctx, &pb.GetEmbeddingConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Model != "text-embedding-3-small" {
		t.Errorf("Model = %q", got.Model)
	}

	list, err := srv.ListEmbeddingConfigs(ctx, &pb.ListEmbeddingConfigsRequest{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Configs) != 1 {
		t.Errorf("List len = %d", len(list.Configs))
	}

	upd, err := srv.UpdateEmbeddingConfig(ctx, &pb.UpdateEmbeddingConfigRequest{
		Id: c.Id, Name: "ada-renamed",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Name != "ada-renamed" {
		t.Errorf("Name = %q", upd.Name)
	}

	def, err := srv.SetDefaultEmbeddingConfig(ctx, &pb.SetDefaultEmbeddingConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !def.IsDefault {
		t.Error("should be default")
	}

	_, err = srv.DeleteEmbeddingConfig(ctx, &pb.DeleteEmbeddingConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestConfigServer_Integration_ExtractionPromptLifecycle(t *testing.T) {
	srv, cleanup := setupConfigServerTest(t)
	defer cleanup()
	ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{ID: "system:test", IsSystem: true})

	c, err := srv.CreateExtractionPromptConfig(ctx, &pb.CreateExtractionPromptConfigRequest{
		Name: "default", Prompt: "Extract key facts", Description: "Standard",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Prompt != "Extract key facts" {
		t.Errorf("Prompt = %q", c.Prompt)
	}

	got, err := srv.GetExtractionPromptConfig(ctx, &pb.GetExtractionPromptConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Standard" {
		t.Errorf("Description = %q", got.Description)
	}

	list, err := srv.ListExtractionPromptConfigs(ctx, &pb.ListExtractionPromptConfigsRequest{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Configs) != 1 {
		t.Errorf("List len = %d", len(list.Configs))
	}

	upd, err := srv.UpdateExtractionPromptConfig(ctx, &pb.UpdateExtractionPromptConfigRequest{
		Id: c.Id, Name: "renamed", Prompt: "Updated prompt",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Name != "renamed" {
		t.Errorf("Name = %q", upd.Name)
	}

	def, err := srv.SetDefaultExtractionPromptConfig(ctx, &pb.SetDefaultExtractionPromptConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !def.IsDefault {
		t.Error("should be default")
	}

	_, err = srv.DeleteExtractionPromptConfig(ctx, &pb.DeleteExtractionPromptConfigRequest{Id: c.Id})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
