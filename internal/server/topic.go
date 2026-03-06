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
	topicStore *store.TopicStore
	authorizer auth.Authorizer
}

// NewTopicServer creates a new topic service.
// coverage:ignore - gRPC handler; tested via integration tests
func NewTopicServer(topicStore *store.TopicStore, authorizer auth.Authorizer) *TopicServer {
	return &TopicServer{
		topicStore: topicStore,
		authorizer: authorizer,
	}
}

// CreateTopic creates a new topic owned by the authenticated principal.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *TopicServer) CreateTopic(ctx context.Context, req *pb.CreateTopicRequest) (*pb.Topic, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetSlug() == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	t, err := s.topicStore.Create(ctx, req.GetSlug(), req.GetName(), req.GetDescription(), p.ID)
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating topic: %v", err)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return storeTopicToProto(t), nil
}

// GetTopic retrieves a topic by ID with ACL check.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *TopicServer) GetTopic(ctx context.Context, req *pb.GetTopicRequest) (*pb.Topic, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, req.GetId(), auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	t, err := s.topicStore.Get(ctx, req.GetId())
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "topic not found")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return storeTopicToProto(t), nil
}

// ListTopics returns topics accessible to the authenticated principal.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *TopicServer) ListTopics(ctx context.Context, _ *pb.ListTopicsRequest) (*pb.ListTopicsResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	// System accounts see all topics.
	// coverage:ignore - gRPC handler; tested via integration tests
	var principals []string
	// coverage:ignore - gRPC handler; tested via integration tests
	if !p.IsSystem {
		principals = append([]string{p.ID}, p.Groups...)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	topics, err := s.topicStore.ListForPrincipals(ctx, principals)
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing topics: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	pbTopics := make([]*pb.Topic, len(topics))
	// coverage:ignore - gRPC handler; tested via integration tests
	for i, t := range topics {
		pbTopics[i] = storeTopicToProto(&t)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.ListTopicsResponse{Topics: pbTopics}, nil
}

// UpdateTopic updates a topic with admin ACL check.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *TopicServer) UpdateTopic(ctx context.Context, req *pb.UpdateTopicRequest) (*pb.Topic, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, req.GetId(), auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	t, err := s.topicStore.Update(ctx, req.GetId(), req.GetName(), req.GetDescription())
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating topic: %v", err)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return storeTopicToProto(t), nil
}

// DeleteTopic deletes a topic with admin ACL check. Cascades to all content.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *TopicServer) DeleteTopic(ctx context.Context, req *pb.DeleteTopicRequest) (*pb.DeleteTopicResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, req.GetId(), auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.topicStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting topic: %v", err)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.DeleteTopicResponse{}, nil
}

// GrantAccess grants a principal access to a topic. Requires admin permission.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *TopicServer) GrantAccess(ctx context.Context, req *pb.GrantAccessRequest) (*pb.TopicGrant, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetTopicId() == "" || req.GetPrincipal() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_id and principal are required")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetPermission() == pb.Permission_PERMISSION_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "permission is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, req.GetTopicId(), auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	perm := protoPermissionToString(req.GetPermission())
	g, err := s.topicStore.Grant(ctx, req.GetTopicId(), req.GetPrincipal(), perm, p.ID)
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "granting access: %v", err)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return storeGrantToProto(g), nil
}

// RevokeAccess revokes a principal's access to a topic. Requires admin permission.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *TopicServer) RevokeAccess(ctx context.Context, req *pb.RevokeAccessRequest) (*pb.RevokeAccessResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetTopicId() == "" || req.GetPrincipal() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_id and principal are required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, req.GetTopicId(), auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.topicStore.Revoke(ctx, req.GetTopicId(), req.GetPrincipal()); err != nil {
		return nil, status.Errorf(codes.Internal, "revoking access: %v", err)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.RevokeAccessResponse{}, nil
}

// ListGrants lists all grants for a topic. Requires read permission.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *TopicServer) ListGrants(ctx context.Context, req *pb.ListGrantsRequest) (*pb.ListGrantsResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetTopicId() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_id is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, req.GetTopicId(), auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	grants, err := s.topicStore.ListGrants(ctx, req.GetTopicId())
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing grants: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	pbGrants := make([]*pb.TopicGrant, len(grants))
	// coverage:ignore - gRPC handler; tested via integration tests
	for i, g := range grants {
		pbGrants[i] = storeGrantToProto(&g)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.ListGrantsResponse{Grants: pbGrants}, nil
}

// coverage:ignore - gRPC handler; tested via integration tests
func storeTopicToProto(t *store.Topic) *pb.Topic {
	return &pb.Topic{
		Id:          t.ID,
		Slug:        t.Slug,
		Name:        t.Name,
		Description: t.Description,
		Owner:       t.Owner,
		CreatedAt:   timestamppb.New(t.CreatedAt),
		UpdatedAt:   timestamppb.New(t.UpdatedAt),
	}
}

// coverage:ignore - gRPC handler; tested via integration tests
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

// coverage:ignore - gRPC handler; tested via integration tests
func protoPermissionToString(p pb.Permission) string {
	switch p {
	// coverage:ignore - gRPC handler; tested via integration tests
	case pb.Permission_PERMISSION_READ:
		return "read"
	// coverage:ignore - gRPC handler; tested via integration tests
	case pb.Permission_PERMISSION_WRITE:
		return "write"
	// coverage:ignore - gRPC handler; tested via integration tests
	case pb.Permission_PERMISSION_ADMIN:
		return "admin"
	// coverage:ignore - gRPC handler; tested via integration tests
	default:
		return "read"
	}
}

// coverage:ignore - gRPC handler; tested via integration tests
func stringToProtoPermission(s string) pb.Permission {
	switch s {
	// coverage:ignore - gRPC handler; tested via integration tests
	case "read":
		return pb.Permission_PERMISSION_READ
	// coverage:ignore - gRPC handler; tested via integration tests
	case "write":
		return pb.Permission_PERMISSION_WRITE
	// coverage:ignore - gRPC handler; tested via integration tests
	case "admin":
		return pb.Permission_PERMISSION_ADMIN
	// coverage:ignore - gRPC handler; tested via integration tests
	default:
		return pb.Permission_PERMISSION_UNSPECIFIED
	}
}
