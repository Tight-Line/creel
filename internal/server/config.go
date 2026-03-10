package server

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

// ConfigServer implements the ConfigService gRPC service.
type ConfigServer struct {
	pb.UnimplementedConfigServiceServer
	apiKeyStore           *store.APIKeyConfigStore
	llmStore              *store.LLMConfigStore
	embeddingStore        *store.EmbeddingConfigStore
	extractionPromptStore *store.ExtractionPromptConfigStore
	vectorBackendStore    *store.VectorBackendConfigStore
}

// NewConfigServer creates a new config service.
func NewConfigServer(
	apiKeyStore *store.APIKeyConfigStore,
	llmStore *store.LLMConfigStore,
	embeddingStore *store.EmbeddingConfigStore,
	extractionPromptStore *store.ExtractionPromptConfigStore,
	vectorBackendStore *store.VectorBackendConfigStore,
) *ConfigServer {
	return &ConfigServer{
		apiKeyStore:           apiKeyStore,
		llmStore:              llmStore,
		embeddingStore:        embeddingStore,
		extractionPromptStore: extractionPromptStore,
		vectorBackendStore:    vectorBackendStore,
	}
}

func requireSystem(ctx context.Context) error {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return status.Error(codes.Unauthenticated, "not authenticated")
	}
	if !p.IsSystem {
		return status.Error(codes.PermissionDenied, "system account required")
	}
	return nil
}

// ---------------------------------------------------------------------------
// API Key Config RPCs
// ---------------------------------------------------------------------------

func (s *ConfigServer) CreateAPIKeyConfig(ctx context.Context, req *pb.CreateAPIKeyConfigRequest) (*pb.APIKeyConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.GetApiKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	c, err := s.apiKeyStore.Create(ctx, req.GetName(), req.GetProvider(), []byte(req.GetApiKey()), req.GetIsDefault())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating API key config: %v", err)
	}
	return storeAPIKeyConfigToProto(c), nil
}

func (s *ConfigServer) GetAPIKeyConfig(ctx context.Context, req *pb.GetAPIKeyConfigRequest) (*pb.APIKeyConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.apiKeyStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "API key config not found")
	}
	return storeAPIKeyConfigToProto(c), nil
}

func (s *ConfigServer) ListAPIKeyConfigs(ctx context.Context, _ *pb.ListAPIKeyConfigsRequest) (*pb.ListAPIKeyConfigsResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}

	configs, err := s.apiKeyStore.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing API key configs: %v", err)
	}

	pbConfigs := make([]*pb.APIKeyConfig, len(configs))
	for i, c := range configs {
		pbConfigs[i] = storeAPIKeyConfigToProto(&c)
	}
	return &pb.ListAPIKeyConfigsResponse{Configs: pbConfigs}, nil
}

func (s *ConfigServer) UpdateAPIKeyConfig(ctx context.Context, req *pb.UpdateAPIKeyConfigRequest) (*pb.APIKeyConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	var apiKey []byte
	if req.GetApiKey() != "" {
		apiKey = []byte(req.GetApiKey())
	}

	c, err := s.apiKeyStore.Update(ctx, req.GetId(), req.GetName(), req.GetProvider(), apiKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating API key config: %v", err)
	}
	return storeAPIKeyConfigToProto(c), nil
}

func (s *ConfigServer) DeleteAPIKeyConfig(ctx context.Context, req *pb.DeleteAPIKeyConfigRequest) (*pb.DeleteAPIKeyConfigResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.apiKeyStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting API key config: %v", err)
	}
	return &pb.DeleteAPIKeyConfigResponse{}, nil
}

func (s *ConfigServer) SetDefaultAPIKeyConfig(ctx context.Context, req *pb.SetDefaultAPIKeyConfigRequest) (*pb.APIKeyConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.apiKeyStore.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "setting default API key config: %v", err)
	}
	return storeAPIKeyConfigToProto(c), nil
}

// ---------------------------------------------------------------------------
// LLM Config RPCs
// ---------------------------------------------------------------------------

func (s *ConfigServer) CreateLLMConfig(ctx context.Context, req *pb.CreateLLMConfigRequest) (*pb.LLMConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.GetModel() == "" {
		return nil, status.Error(codes.InvalidArgument, "model is required")
	}
	if req.GetApiKeyConfigId() == "" {
		return nil, status.Error(codes.InvalidArgument, "api_key_config_id is required")
	}

	c, err := s.llmStore.Create(ctx, req.GetName(), req.GetProvider(), req.GetModel(), req.GetParameters(), req.GetApiKeyConfigId(), req.GetIsDefault())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating LLM config: %v", err)
	}
	return storeLLMConfigToProto(c), nil
}

func (s *ConfigServer) GetLLMConfig(ctx context.Context, req *pb.GetLLMConfigRequest) (*pb.LLMConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.llmStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "LLM config not found")
	}
	return storeLLMConfigToProto(c), nil
}

func (s *ConfigServer) ListLLMConfigs(ctx context.Context, _ *pb.ListLLMConfigsRequest) (*pb.ListLLMConfigsResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}

	configs, err := s.llmStore.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing LLM configs: %v", err)
	}

	pbConfigs := make([]*pb.LLMConfig, len(configs))
	for i, c := range configs {
		pbConfigs[i] = storeLLMConfigToProto(&c)
	}
	return &pb.ListLLMConfigsResponse{Configs: pbConfigs}, nil
}

func (s *ConfigServer) UpdateLLMConfig(ctx context.Context, req *pb.UpdateLLMConfigRequest) (*pb.LLMConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	var params map[string]string
	if len(req.GetParameters()) > 0 {
		params = req.GetParameters()
	}

	c, err := s.llmStore.Update(ctx, req.GetId(), req.GetName(), req.GetProvider(), req.GetModel(), params, req.GetApiKeyConfigId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating LLM config: %v", err)
	}
	return storeLLMConfigToProto(c), nil
}

func (s *ConfigServer) DeleteLLMConfig(ctx context.Context, req *pb.DeleteLLMConfigRequest) (*pb.DeleteLLMConfigResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.llmStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting LLM config: %v", err)
	}
	return &pb.DeleteLLMConfigResponse{}, nil
}

func (s *ConfigServer) SetDefaultLLMConfig(ctx context.Context, req *pb.SetDefaultLLMConfigRequest) (*pb.LLMConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.llmStore.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "setting default LLM config: %v", err)
	}
	return storeLLMConfigToProto(c), nil
}

// ---------------------------------------------------------------------------
// Embedding Config RPCs
// ---------------------------------------------------------------------------

func (s *ConfigServer) CreateEmbeddingConfig(ctx context.Context, req *pb.CreateEmbeddingConfigRequest) (*pb.EmbeddingConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.GetModel() == "" {
		return nil, status.Error(codes.InvalidArgument, "model is required")
	}
	if req.GetDimensions() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "dimensions must be positive")
	}
	if req.GetApiKeyConfigId() == "" {
		return nil, status.Error(codes.InvalidArgument, "api_key_config_id is required")
	}

	c, err := s.embeddingStore.Create(ctx, req.GetName(), req.GetProvider(), req.GetModel(), int(req.GetDimensions()), req.GetApiKeyConfigId(), req.GetIsDefault())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating embedding config: %v", err)
	}
	return storeEmbeddingConfigToProto(c), nil
}

func (s *ConfigServer) GetEmbeddingConfig(ctx context.Context, req *pb.GetEmbeddingConfigRequest) (*pb.EmbeddingConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.embeddingStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "embedding config not found")
	}
	return storeEmbeddingConfigToProto(c), nil
}

func (s *ConfigServer) ListEmbeddingConfigs(ctx context.Context, _ *pb.ListEmbeddingConfigsRequest) (*pb.ListEmbeddingConfigsResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}

	configs, err := s.embeddingStore.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing embedding configs: %v", err)
	}

	pbConfigs := make([]*pb.EmbeddingConfig, len(configs))
	for i, c := range configs {
		pbConfigs[i] = storeEmbeddingConfigToProto(&c)
	}
	return &pb.ListEmbeddingConfigsResponse{Configs: pbConfigs}, nil
}

func (s *ConfigServer) UpdateEmbeddingConfig(ctx context.Context, req *pb.UpdateEmbeddingConfigRequest) (*pb.EmbeddingConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.embeddingStore.Update(ctx, req.GetId(), req.GetName(), req.GetApiKeyConfigId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating embedding config: %v", err)
	}
	return storeEmbeddingConfigToProto(c), nil
}

func (s *ConfigServer) DeleteEmbeddingConfig(ctx context.Context, req *pb.DeleteEmbeddingConfigRequest) (*pb.DeleteEmbeddingConfigResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.embeddingStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting embedding config: %v", err)
	}
	return &pb.DeleteEmbeddingConfigResponse{}, nil
}

func (s *ConfigServer) SetDefaultEmbeddingConfig(ctx context.Context, req *pb.SetDefaultEmbeddingConfigRequest) (*pb.EmbeddingConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.embeddingStore.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "setting default embedding config: %v", err)
	}
	return storeEmbeddingConfigToProto(c), nil
}

// ---------------------------------------------------------------------------
// Extraction Prompt Config RPCs
// ---------------------------------------------------------------------------

func (s *ConfigServer) CreateExtractionPromptConfig(ctx context.Context, req *pb.CreateExtractionPromptConfigRequest) (*pb.ExtractionPromptConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetPrompt() == "" {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}

	c, err := s.extractionPromptStore.Create(ctx, req.GetName(), req.GetPrompt(), req.GetDescription(), req.GetIsDefault())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating extraction prompt config: %v", err)
	}
	return storeExtractionPromptConfigToProto(c), nil
}

func (s *ConfigServer) GetExtractionPromptConfig(ctx context.Context, req *pb.GetExtractionPromptConfigRequest) (*pb.ExtractionPromptConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.extractionPromptStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "extraction prompt config not found")
	}
	return storeExtractionPromptConfigToProto(c), nil
}

func (s *ConfigServer) ListExtractionPromptConfigs(ctx context.Context, _ *pb.ListExtractionPromptConfigsRequest) (*pb.ListExtractionPromptConfigsResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}

	configs, err := s.extractionPromptStore.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing extraction prompt configs: %v", err)
	}

	pbConfigs := make([]*pb.ExtractionPromptConfig, len(configs))
	for i, c := range configs {
		pbConfigs[i] = storeExtractionPromptConfigToProto(&c)
	}
	return &pb.ListExtractionPromptConfigsResponse{Configs: pbConfigs}, nil
}

func (s *ConfigServer) UpdateExtractionPromptConfig(ctx context.Context, req *pb.UpdateExtractionPromptConfigRequest) (*pb.ExtractionPromptConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.extractionPromptStore.Update(ctx, req.GetId(), req.GetName(), req.GetPrompt(), req.GetDescription())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating extraction prompt config: %v", err)
	}
	return storeExtractionPromptConfigToProto(c), nil
}

func (s *ConfigServer) DeleteExtractionPromptConfig(ctx context.Context, req *pb.DeleteExtractionPromptConfigRequest) (*pb.DeleteExtractionPromptConfigResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.extractionPromptStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting extraction prompt config: %v", err)
	}
	return &pb.DeleteExtractionPromptConfigResponse{}, nil
}

func (s *ConfigServer) SetDefaultExtractionPromptConfig(ctx context.Context, req *pb.SetDefaultExtractionPromptConfigRequest) (*pb.ExtractionPromptConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.extractionPromptStore.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "setting default extraction prompt config: %v", err)
	}
	return storeExtractionPromptConfigToProto(c), nil
}

// ---------------------------------------------------------------------------
// Proto converters
// ---------------------------------------------------------------------------

func storeAPIKeyConfigToProto(c *store.APIKeyConfig) *pb.APIKeyConfig {
	return &pb.APIKeyConfig{
		Id:        c.ID,
		Name:      c.Name,
		Provider:  c.Provider,
		IsDefault: c.IsDefault,
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
}

func storeLLMConfigToProto(c *store.LLMConfig) *pb.LLMConfig {
	return &pb.LLMConfig{
		Id:             c.ID,
		Name:           c.Name,
		Provider:       c.Provider,
		Model:          c.Model,
		Parameters:     c.Parameters,
		ApiKeyConfigId: c.APIKeyConfigID,
		IsDefault:      c.IsDefault,
		CreatedAt:      timestamppb.New(c.CreatedAt),
		UpdatedAt:      timestamppb.New(c.UpdatedAt),
	}
}

func storeEmbeddingConfigToProto(c *store.EmbeddingConfig) *pb.EmbeddingConfig {
	return &pb.EmbeddingConfig{
		Id:             c.ID,
		Name:           c.Name,
		Provider:       c.Provider,
		Model:          c.Model,
		Dimensions:     int32(c.Dimensions),
		ApiKeyConfigId: c.APIKeyConfigID,
		IsDefault:      c.IsDefault,
		CreatedAt:      timestamppb.New(c.CreatedAt),
		UpdatedAt:      timestamppb.New(c.UpdatedAt),
	}
}

func storeExtractionPromptConfigToProto(c *store.ExtractionPromptConfig) *pb.ExtractionPromptConfig {
	return &pb.ExtractionPromptConfig{
		Id:          c.ID,
		Name:        c.Name,
		Prompt:      c.Prompt,
		Description: c.Description,
		IsDefault:   c.IsDefault,
		CreatedAt:   timestamppb.New(c.CreatedAt),
		UpdatedAt:   timestamppb.New(c.UpdatedAt),
	}
}

// ---------------------------------------------------------------------------
// Vector Backend Config RPCs
// ---------------------------------------------------------------------------

func (s *ConfigServer) CreateVectorBackendConfig(ctx context.Context, req *pb.CreateVectorBackendConfigRequest) (*pb.VectorBackendConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetBackend() == "" {
		return nil, status.Error(codes.InvalidArgument, "backend is required")
	}

	config := stringMapToAnyMap(req.GetConfig())
	c, err := s.vectorBackendStore.Create(ctx, req.GetName(), req.GetBackend(), config, req.GetIsDefault())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating vector backend config: %v", err)
	}
	return storeVectorBackendConfigToProto(c), nil
}

func (s *ConfigServer) GetVectorBackendConfig(ctx context.Context, req *pb.GetVectorBackendConfigRequest) (*pb.VectorBackendConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.vectorBackendStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "vector backend config not found")
	}
	return storeVectorBackendConfigToProto(c), nil
}

func (s *ConfigServer) ListVectorBackendConfigs(ctx context.Context, _ *pb.ListVectorBackendConfigsRequest) (*pb.ListVectorBackendConfigsResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}

	configs, err := s.vectorBackendStore.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing vector backend configs: %v", err)
	}

	pbConfigs := make([]*pb.VectorBackendConfig, len(configs))
	for i, c := range configs {
		pbConfigs[i] = storeVectorBackendConfigToProto(&c)
	}
	return &pb.ListVectorBackendConfigsResponse{Configs: pbConfigs}, nil
}

func (s *ConfigServer) UpdateVectorBackendConfig(ctx context.Context, req *pb.UpdateVectorBackendConfigRequest) (*pb.VectorBackendConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	var config map[string]any
	if len(req.GetConfig()) > 0 {
		config = stringMapToAnyMap(req.GetConfig())
	}

	c, err := s.vectorBackendStore.Update(ctx, req.GetId(), req.GetName(), config)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating vector backend config: %v", err)
	}
	return storeVectorBackendConfigToProto(c), nil
}

func (s *ConfigServer) DeleteVectorBackendConfig(ctx context.Context, req *pb.DeleteVectorBackendConfigRequest) (*pb.DeleteVectorBackendConfigResponse, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.vectorBackendStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting vector backend config: %v", err)
	}
	return &pb.DeleteVectorBackendConfigResponse{}, nil
}

func (s *ConfigServer) SetDefaultVectorBackendConfig(ctx context.Context, req *pb.SetDefaultVectorBackendConfigRequest) (*pb.VectorBackendConfig, error) {
	if err := requireSystem(ctx); err != nil {
		return nil, err
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.vectorBackendStore.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "setting default vector backend config: %v", err)
	}
	return storeVectorBackendConfigToProto(c), nil
}

func storeVectorBackendConfigToProto(c *store.VectorBackendConfig) *pb.VectorBackendConfig {
	config := make(map[string]string, len(c.Config))
	for k, v := range c.Config {
		config[k] = fmt.Sprintf("%v", v)
	}
	return &pb.VectorBackendConfig{
		Id:        c.ID,
		Name:      c.Name,
		Backend:   c.Backend,
		Config:    config,
		IsDefault: c.IsDefault,
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
}

// stringMapToAnyMap converts map[string]string to map[string]any.
func stringMapToAnyMap(m map[string]string) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
