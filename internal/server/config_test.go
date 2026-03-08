package server

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/crypto"
	"github.com/Tight-Line/creel/internal/store"
)

func configServer() *ConfigServer {
	enc, _ := crypto.NewEncryptor("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	db := failDBTX()
	return NewConfigServer(
		store.NewAPIKeyConfigStore(db, enc),
		store.NewLLMConfigStore(db),
		store.NewEmbeddingConfigStore(db),
		store.NewExtractionPromptConfigStore(db),
	)
}

// ---------------------------------------------------------------------------
// Auth tests
// ---------------------------------------------------------------------------

func TestConfigServer_RequiresSystemAccount(t *testing.T) {
	s := configServer()
	ctx := context.Background() // no principal

	_, err := s.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{Name: "x", Provider: "openai", ApiKey: "sk-x"})
	requireCode(t, err, codes.Unauthenticated)

	_, err = s.ListAPIKeyConfigs(ctx, &pb.ListAPIKeyConfigsRequest{})
	requireCode(t, err, codes.Unauthenticated)
}

func TestConfigServer_NonSystemDenied(t *testing.T) {
	s := configServer()
	ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{
		ID:       "user:test@example.com",
		IsSystem: false,
	})

	_, err := s.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{Name: "x", Provider: "openai", ApiKey: "sk-x"})
	requireCode(t, err, codes.PermissionDenied)
}

// ---------------------------------------------------------------------------
// API Key Config validation
// ---------------------------------------------------------------------------

func TestConfigServer_CreateAPIKeyConfig_Validation(t *testing.T) {
	s := configServer()
	ctx := systemCtx()

	_, err := s.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{Name: "", Provider: "openai", ApiKey: "sk-x"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{Name: "x", Provider: "", ApiKey: "sk-x"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{Name: "x", Provider: "openai", ApiKey: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_GetAPIKeyConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.GetAPIKeyConfig(ctx, &pb.GetAPIKeyConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_UpdateAPIKeyConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.UpdateAPIKeyConfig(ctx, &pb.UpdateAPIKeyConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_DeleteAPIKeyConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.DeleteAPIKeyConfig(ctx, &pb.DeleteAPIKeyConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_SetDefaultAPIKeyConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.SetDefaultAPIKeyConfig(ctx, &pb.SetDefaultAPIKeyConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

// ---------------------------------------------------------------------------
// LLM Config validation
// ---------------------------------------------------------------------------

func TestConfigServer_CreateLLMConfig_Validation(t *testing.T) {
	s := configServer()
	ctx := systemCtx()

	_, err := s.CreateLLMConfig(ctx, &pb.CreateLLMConfigRequest{Name: "", Provider: "openai", Model: "gpt-4o", ApiKeyConfigId: "id"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateLLMConfig(ctx, &pb.CreateLLMConfigRequest{Name: "x", Provider: "", Model: "gpt-4o", ApiKeyConfigId: "id"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateLLMConfig(ctx, &pb.CreateLLMConfigRequest{Name: "x", Provider: "openai", Model: "", ApiKeyConfigId: "id"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateLLMConfig(ctx, &pb.CreateLLMConfigRequest{Name: "x", Provider: "openai", Model: "gpt-4o", ApiKeyConfigId: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_GetLLMConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.GetLLMConfig(ctx, &pb.GetLLMConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_UpdateLLMConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.UpdateLLMConfig(ctx, &pb.UpdateLLMConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_DeleteLLMConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.DeleteLLMConfig(ctx, &pb.DeleteLLMConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_SetDefaultLLMConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.SetDefaultLLMConfig(ctx, &pb.SetDefaultLLMConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

// ---------------------------------------------------------------------------
// Embedding Config validation
// ---------------------------------------------------------------------------

func TestConfigServer_CreateEmbeddingConfig_Validation(t *testing.T) {
	s := configServer()
	ctx := systemCtx()

	_, err := s.CreateEmbeddingConfig(ctx, &pb.CreateEmbeddingConfigRequest{Name: "", Provider: "openai", Model: "ada", Dimensions: 1536, ApiKeyConfigId: "id"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateEmbeddingConfig(ctx, &pb.CreateEmbeddingConfigRequest{Name: "x", Provider: "", Model: "ada", Dimensions: 1536, ApiKeyConfigId: "id"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateEmbeddingConfig(ctx, &pb.CreateEmbeddingConfigRequest{Name: "x", Provider: "openai", Model: "", Dimensions: 1536, ApiKeyConfigId: "id"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateEmbeddingConfig(ctx, &pb.CreateEmbeddingConfigRequest{Name: "x", Provider: "openai", Model: "ada", Dimensions: 0, ApiKeyConfigId: "id"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateEmbeddingConfig(ctx, &pb.CreateEmbeddingConfigRequest{Name: "x", Provider: "openai", Model: "ada", Dimensions: 1536, ApiKeyConfigId: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_GetEmbeddingConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.GetEmbeddingConfig(ctx, &pb.GetEmbeddingConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_UpdateEmbeddingConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.UpdateEmbeddingConfig(ctx, &pb.UpdateEmbeddingConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_DeleteEmbeddingConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.DeleteEmbeddingConfig(ctx, &pb.DeleteEmbeddingConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_SetDefaultEmbeddingConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.SetDefaultEmbeddingConfig(ctx, &pb.SetDefaultEmbeddingConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

// ---------------------------------------------------------------------------
// Extraction Prompt Config validation
// ---------------------------------------------------------------------------

func TestConfigServer_CreateExtractionPromptConfig_Validation(t *testing.T) {
	s := configServer()
	ctx := systemCtx()

	_, err := s.CreateExtractionPromptConfig(ctx, &pb.CreateExtractionPromptConfigRequest{Name: "", Prompt: "extract"})
	requireCode(t, err, codes.InvalidArgument)

	_, err = s.CreateExtractionPromptConfig(ctx, &pb.CreateExtractionPromptConfigRequest{Name: "x", Prompt: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_GetExtractionPromptConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.GetExtractionPromptConfig(ctx, &pb.GetExtractionPromptConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_UpdateExtractionPromptConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.UpdateExtractionPromptConfig(ctx, &pb.UpdateExtractionPromptConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_DeleteExtractionPromptConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.DeleteExtractionPromptConfig(ctx, &pb.DeleteExtractionPromptConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

func TestConfigServer_SetDefaultExtractionPromptConfig_EmptyID(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.SetDefaultExtractionPromptConfig(ctx, &pb.SetDefaultExtractionPromptConfigRequest{Id: ""})
	requireCode(t, err, codes.InvalidArgument)
}

// ---------------------------------------------------------------------------
// Store error paths (DB failures hit Internal)
// ---------------------------------------------------------------------------

func TestConfigServer_CreateAPIKeyConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.CreateAPIKeyConfig(ctx, &pb.CreateAPIKeyConfigRequest{Name: "x", Provider: "openai", ApiKey: "sk-x"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_GetAPIKeyConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.GetAPIKeyConfig(ctx, &pb.GetAPIKeyConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.NotFound)
}

func TestConfigServer_ListAPIKeyConfigs_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.ListAPIKeyConfigs(ctx, &pb.ListAPIKeyConfigsRequest{})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_UpdateAPIKeyConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.UpdateAPIKeyConfig(ctx, &pb.UpdateAPIKeyConfigRequest{Id: "some-id", Name: "new"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_DeleteAPIKeyConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.DeleteAPIKeyConfig(ctx, &pb.DeleteAPIKeyConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_SetDefaultAPIKeyConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.SetDefaultAPIKeyConfig(ctx, &pb.SetDefaultAPIKeyConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_CreateLLMConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.CreateLLMConfig(ctx, &pb.CreateLLMConfigRequest{Name: "x", Provider: "openai", Model: "gpt-4o", ApiKeyConfigId: "id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_GetLLMConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.GetLLMConfig(ctx, &pb.GetLLMConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.NotFound)
}

func TestConfigServer_ListLLMConfigs_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.ListLLMConfigs(ctx, &pb.ListLLMConfigsRequest{})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_UpdateLLMConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.UpdateLLMConfig(ctx, &pb.UpdateLLMConfigRequest{Id: "some-id", Name: "new"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_DeleteLLMConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.DeleteLLMConfig(ctx, &pb.DeleteLLMConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_SetDefaultLLMConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.SetDefaultLLMConfig(ctx, &pb.SetDefaultLLMConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_CreateEmbeddingConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.CreateEmbeddingConfig(ctx, &pb.CreateEmbeddingConfigRequest{Name: "x", Provider: "openai", Model: "ada", Dimensions: 1536, ApiKeyConfigId: "id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_GetEmbeddingConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.GetEmbeddingConfig(ctx, &pb.GetEmbeddingConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.NotFound)
}

func TestConfigServer_ListEmbeddingConfigs_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.ListEmbeddingConfigs(ctx, &pb.ListEmbeddingConfigsRequest{})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_UpdateEmbeddingConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.UpdateEmbeddingConfig(ctx, &pb.UpdateEmbeddingConfigRequest{Id: "some-id", Name: "new"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_DeleteEmbeddingConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.DeleteEmbeddingConfig(ctx, &pb.DeleteEmbeddingConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_SetDefaultEmbeddingConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.SetDefaultEmbeddingConfig(ctx, &pb.SetDefaultEmbeddingConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_CreateExtractionPromptConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.CreateExtractionPromptConfig(ctx, &pb.CreateExtractionPromptConfigRequest{Name: "x", Prompt: "extract"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_GetExtractionPromptConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.GetExtractionPromptConfig(ctx, &pb.GetExtractionPromptConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.NotFound)
}

func TestConfigServer_ListExtractionPromptConfigs_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.ListExtractionPromptConfigs(ctx, &pb.ListExtractionPromptConfigsRequest{})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_UpdateExtractionPromptConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.UpdateExtractionPromptConfig(ctx, &pb.UpdateExtractionPromptConfigRequest{Id: "some-id", Name: "new"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_DeleteExtractionPromptConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.DeleteExtractionPromptConfig(ctx, &pb.DeleteExtractionPromptConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}

func TestConfigServer_SetDefaultExtractionPromptConfig_StoreError(t *testing.T) {
	s := configServer()
	ctx := systemCtx()
	_, err := s.SetDefaultExtractionPromptConfig(ctx, &pb.SetDefaultExtractionPromptConfigRequest{Id: "some-id"})
	requireCode(t, err, codes.Internal)
}
