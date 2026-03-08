package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

// TopicServer implements the TopicService gRPC service.
type TopicServer struct {
	pb.UnimplementedTopicServiceServer
	topicStore     *store.TopicStore
	authorizer     auth.Authorizer
	embeddingStore *store.EmbeddingConfigStore
}

// NewTopicServer creates a new topic service.
func NewTopicServer(topicStore *store.TopicStore, authorizer auth.Authorizer, embeddingStore *store.EmbeddingConfigStore) *TopicServer {
	return &TopicServer{
		topicStore:     topicStore,
		authorizer:     authorizer,
		embeddingStore: embeddingStore,
	}
}

// CreateTopic creates a new topic owned by the authenticated principal.
func (s *TopicServer) CreateTopic(ctx context.Context, req *pb.CreateTopicRequest) (*pb.Topic, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetSlug() == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// Enforce: if extraction prompt is set, LLM config must also be set.
	if req.ExtractionPromptConfigId != nil && req.GetExtractionPromptConfigId() != "" {
		if req.LlmConfigId == nil || req.GetLlmConfigId() == "" {
			return nil, status.Error(codes.InvalidArgument, "extraction_prompt_config_id requires llm_config_id")
		}
	}

	var llmCfgID, embCfgID, promptCfgID *string
	if req.LlmConfigId != nil {
		v := req.GetLlmConfigId()
		llmCfgID = &v
	}
	if req.EmbeddingConfigId != nil {
		v := req.GetEmbeddingConfigId()
		embCfgID = &v
	}
	if req.ExtractionPromptConfigId != nil {
		v := req.GetExtractionPromptConfigId()
		promptCfgID = &v
	}

	t, err := s.topicStore.Create(ctx, req.GetSlug(), req.GetName(), req.GetDescription(), p.ID, llmCfgID, embCfgID, promptCfgID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating topic: %v", err)
	}
	return storeTopicToProto(t), nil
}

// GetTopic retrieves a topic by ID with ACL check.
func (s *TopicServer) GetTopic(ctx context.Context, req *pb.GetTopicRequest) (*pb.Topic, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.authorizer.Check(ctx, p, req.GetId(), auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	t, err := s.topicStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "topic not found")
	}
	return storeTopicToProto(t), nil
}

// ListTopics returns topics accessible to the authenticated principal.
func (s *TopicServer) ListTopics(ctx context.Context, _ *pb.ListTopicsRequest) (*pb.ListTopicsResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	// System accounts see all topics.
	var principals []string
	if !p.IsSystem {
		principals = append([]string{p.ID}, p.Groups...)
	}

	topics, err := s.topicStore.ListForPrincipals(ctx, principals)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing topics: %v", err)
	}

	pbTopics := make([]*pb.Topic, len(topics))
	for i, t := range topics {
		pbTopics[i] = storeTopicToProto(&t)
	}
	return &pb.ListTopicsResponse{Topics: pbTopics}, nil
}

// UpdateTopic updates a topic with admin ACL check.
func (s *TopicServer) UpdateTopic(ctx context.Context, req *pb.UpdateTopicRequest) (*pb.Topic, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.authorizer.Check(ctx, p, req.GetId(), auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// Fetch current topic to validate constraints.
	existing, err := s.topicStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "topic not found")
	}

	// Determine effective config IDs after update.
	var llmCfgID, embCfgID, promptCfgID *string
	if req.LlmConfigId != nil {
		v := req.GetLlmConfigId()
		llmCfgID = &v
	}
	if req.EmbeddingConfigId != nil {
		v := req.GetEmbeddingConfigId()
		embCfgID = &v
	}
	if req.ExtractionPromptConfigId != nil {
		v := req.GetExtractionPromptConfigId()
		promptCfgID = &v
	}

	// Enforce: if extraction prompt will be set, LLM config must exist.
	effectivePromptID := existing.ExtractionPromptConfigID
	if promptCfgID != nil {
		effectivePromptID = promptCfgID
	}
	effectiveLLMID := existing.LLMConfigID
	if llmCfgID != nil {
		effectiveLLMID = llmCfgID
	}
	if effectivePromptID != nil && *effectivePromptID != "" {
		if effectiveLLMID == nil || *effectiveLLMID == "" {
			return nil, status.Error(codes.InvalidArgument, "extraction_prompt_config_id requires llm_config_id")
		}
	}

	// Enforce: embedding config can only change if provider+model match.
	if embCfgID != nil && existing.EmbeddingConfigID != nil && *existing.EmbeddingConfigID != "" && *embCfgID != *existing.EmbeddingConfigID {
		if s.embeddingStore != nil {
			oldCfg, err := s.embeddingStore.Get(ctx, *existing.EmbeddingConfigID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "fetching current embedding config: %v", err)
			}
			newCfg, err := s.embeddingStore.Get(ctx, *embCfgID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "fetching new embedding config: %v", err)
			}
			if oldCfg.Provider != newCfg.Provider || oldCfg.Model != newCfg.Model {
				return nil, status.Error(codes.InvalidArgument, "embedding config can only change to one with the same provider and model")
			}
		}
	}

	t, err := s.topicStore.Update(ctx, req.GetId(), req.GetName(), req.GetDescription(), llmCfgID, embCfgID, promptCfgID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating topic: %v", err)
	}
	return storeTopicToProto(t), nil
}

// DeleteTopic deletes a topic with admin ACL check. Cascades to all content.
func (s *TopicServer) DeleteTopic(ctx context.Context, req *pb.DeleteTopicRequest) (*pb.DeleteTopicResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.authorizer.Check(ctx, p, req.GetId(), auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	if err := s.topicStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting topic: %v", err)
	}
	return &pb.DeleteTopicResponse{}, nil
}

// GrantAccess grants a principal access to a topic. Requires admin permission.
func (s *TopicServer) GrantAccess(ctx context.Context, req *pb.GrantAccessRequest) (*pb.TopicGrant, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetTopicId() == "" || req.GetPrincipal() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_id and principal are required")
	}
	if req.GetPermission() == pb.Permission_PERMISSION_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "permission is required")
	}

	if err := s.authorizer.Check(ctx, p, req.GetTopicId(), auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	perm := protoPermissionToString(req.GetPermission())
	g, err := s.topicStore.Grant(ctx, req.GetTopicId(), req.GetPrincipal(), perm, p.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "granting access: %v", err)
	}
	return storeGrantToProto(g), nil
}

// RevokeAccess revokes a principal's access to a topic. Requires admin permission.
func (s *TopicServer) RevokeAccess(ctx context.Context, req *pb.RevokeAccessRequest) (*pb.RevokeAccessResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetTopicId() == "" || req.GetPrincipal() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_id and principal are required")
	}

	if err := s.authorizer.Check(ctx, p, req.GetTopicId(), auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	if err := s.topicStore.Revoke(ctx, req.GetTopicId(), req.GetPrincipal()); err != nil {
		return nil, status.Errorf(codes.Internal, "revoking access: %v", err)
	}
	return &pb.RevokeAccessResponse{}, nil
}

// ListGrants lists all grants for a topic. Requires read permission.
func (s *TopicServer) ListGrants(ctx context.Context, req *pb.ListGrantsRequest) (*pb.ListGrantsResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetTopicId() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_id is required")
	}

	if err := s.authorizer.Check(ctx, p, req.GetTopicId(), auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	grants, err := s.topicStore.ListGrants(ctx, req.GetTopicId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing grants: %v", err)
	}

	pbGrants := make([]*pb.TopicGrant, len(grants))
	for i, g := range grants {
		pbGrants[i] = storeGrantToProto(&g)
	}
	return &pb.ListGrantsResponse{Grants: pbGrants}, nil
}

func storeTopicToProto(t *store.Topic) *pb.Topic {
	p := &pb.Topic{
		Id:          t.ID,
		Slug:        t.Slug,
		Name:        t.Name,
		Description: t.Description,
		Owner:       t.Owner,
		CreatedAt:   timestamppb.New(t.CreatedAt),
		UpdatedAt:   timestamppb.New(t.UpdatedAt),
	}
	if t.LLMConfigID != nil {
		p.LlmConfigId = t.LLMConfigID
	}
	if t.EmbeddingConfigID != nil {
		p.EmbeddingConfigId = t.EmbeddingConfigID
	}
	if t.ExtractionPromptConfigID != nil {
		p.ExtractionPromptConfigId = t.ExtractionPromptConfigID
	}
	return p
}

func storeGrantToProto(g *store.TopicGrant) *pb.TopicGrant {
	return &pb.TopicGrant{
		Id:         g.ID,
		TopicId:    g.TopicID,
		Principal:  g.Principal,
		Permission: stringToProtoPermission(g.Permission),
		GrantedBy:  g.GrantedBy,
		CreatedAt:  timestamppb.New(g.CreatedAt),
	}
}

func protoPermissionToString(p pb.Permission) string {
	switch p {
	case pb.Permission_PERMISSION_READ:
		return "read"
	case pb.Permission_PERMISSION_WRITE:
		return "write"
	case pb.Permission_PERMISSION_ADMIN:
		return "admin"
	default:
		return "read"
	}
}

func stringToProtoPermission(s string) pb.Permission {
	switch s {
	case "read":
		return pb.Permission_PERMISSION_READ
	case "write":
		return pb.Permission_PERMISSION_WRITE
	case "admin":
		return pb.Permission_PERMISSION_ADMIN
	default:
		return pb.Permission_PERMISSION_UNSPECIFIED
	}
}
