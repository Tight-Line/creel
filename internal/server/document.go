package server

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

// DocumentServer implements the DocumentService gRPC service.
type DocumentServer struct {
	pb.UnimplementedDocumentServiceServer
	docStore   *store.DocumentStore
	authorizer auth.Authorizer
}

// NewDocumentServer creates a new document service.
func NewDocumentServer(docStore *store.DocumentStore, authorizer auth.Authorizer) *DocumentServer {
	return &DocumentServer{
		docStore:   docStore,
		authorizer: authorizer,
	}
}

// CreateDocument creates a new document. Requires write permission on the topic.
func (s *DocumentServer) CreateDocument(ctx context.Context, req *pb.CreateDocumentRequest) (*pb.Document, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetTopicId() == "" || req.GetSlug() == "" || req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_id, slug, and name are required")
	}

	if err := s.authorizer.Check(ctx, p, req.GetTopicId(), auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	meta := structToMap(req.GetMetadata())
	docType := req.GetDocType()
	if docType == "" {
		docType = "reference"
	}

	d, err := s.docStore.Create(ctx, req.GetTopicId(), req.GetSlug(), req.GetName(), docType, meta)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating document: %v", err)
	}
	return storeDocToProto(d), nil
}

// GetDocument retrieves a document. Requires read permission on its topic.
func (s *DocumentServer) GetDocument(ctx context.Context, req *pb.GetDocumentRequest) (*pb.Document, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	d, err := s.docStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}

	if err := s.authorizer.Check(ctx, p, d.TopicID, auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	return storeDocToProto(d), nil
}

// ListDocuments lists documents in a topic. Requires read permission.
func (s *DocumentServer) ListDocuments(ctx context.Context, req *pb.ListDocumentsRequest) (*pb.ListDocumentsResponse, error) {
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

	docs, err := s.docStore.ListByTopic(ctx, req.GetTopicId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing documents: %v", err)
	}

	pbDocs := make([]*pb.Document, len(docs))
	for i, d := range docs {
		pbDocs[i] = storeDocToProto(&d)
	}
	return &pb.ListDocumentsResponse{Documents: pbDocs}, nil
}

// UpdateDocument updates a document. Requires write permission on its topic.
func (s *DocumentServer) UpdateDocument(ctx context.Context, req *pb.UpdateDocumentRequest) (*pb.Document, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	topicID, err := s.docStore.TopicIDForDocument(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}

	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	meta := structToMap(req.GetMetadata())
	d, err := s.docStore.Update(ctx, req.GetId(), req.GetName(), req.GetDocType(), meta)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating document: %v", err)
	}
	return storeDocToProto(d), nil
}

// DeleteDocument deletes a document. Requires admin permission on its topic.
func (s *DocumentServer) DeleteDocument(ctx context.Context, req *pb.DeleteDocumentRequest) (*pb.DeleteDocumentResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	topicID, err := s.docStore.TopicIDForDocument(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}

	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	if err := s.docStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting document: %v", err)
	}
	return &pb.DeleteDocumentResponse{}, nil
}

func storeDocToProto(d *store.Document) *pb.Document {
	return &pb.Document{
		Id:        d.ID,
		TopicId:   d.TopicID,
		Slug:      d.Slug,
		Name:      d.Name,
		DocType:   d.DocType,
		Metadata:  mapToStruct(d.Metadata),
		CreatedAt: timestamppb.New(d.CreatedAt),
		UpdatedAt: timestamppb.New(d.UpdatedAt),
	}
}

func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	// Round-trip through JSON for reliable conversion.
	b, err := s.MarshalJSON()
	if err != nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func mapToStruct(m map[string]any) *structpb.Struct {
	if m == nil {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
}
