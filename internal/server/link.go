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

// LinkServer implements the LinkService gRPC service.
type LinkServer struct {
	pb.UnimplementedLinkServiceServer
	linkStore  *store.LinkStore
	chunkStore *store.ChunkStore
	authorizer auth.Authorizer
}

// NewLinkServer creates a new link service.
func NewLinkServer(linkStore *store.LinkStore, chunkStore *store.ChunkStore, authorizer auth.Authorizer) *LinkServer {
	return &LinkServer{
		linkStore:  linkStore,
		chunkStore: chunkStore,
		authorizer: authorizer,
	}
}

// CreateLink creates a new link between two chunks. Requires write permission
// on the source chunk's topic.
func (s *LinkServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.Link, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetSourceChunkId() == "" || req.GetTargetChunkId() == "" {
		return nil, status.Error(codes.InvalidArgument, "source_chunk_id and target_chunk_id are required")
	}

	// Look up the source chunk to find its topic for auth.
	sourceChunk, err := s.chunkStore.Get(ctx, req.GetSourceChunkId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "source chunk not found")
	}

	topicID, err := s.chunkStore.DocumentTopicID(ctx, sourceChunk.DocumentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving topic: %v", err) // coverage:ignore - requires DB failure
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// Verify target chunk exists.
	if _, err := s.chunkStore.Get(ctx, req.GetTargetChunkId()); err != nil {
		return nil, status.Error(codes.NotFound, "target chunk not found")
	}

	linkType := protoLinkTypeToStore(req.GetLinkType())
	meta := structToMap(req.GetMetadata())
	link, err := s.linkStore.Create(ctx, req.GetSourceChunkId(), req.GetTargetChunkId(), linkType, p.ID, meta)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating link: %v", err)
	}

	return storeLinkToProto(link), nil
}

// DeleteLink removes a link. Requires write permission on the source chunk's topic.
func (s *LinkServer) DeleteLink(ctx context.Context, req *pb.DeleteLinkRequest) (*pb.DeleteLinkResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Look up the link to find its source chunk's topic for auth.
	link, err := s.linkStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "link not found")
	}

	sourceChunk, err := s.chunkStore.Get(ctx, link.SourceChunkID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving source chunk: %v", err) // coverage:ignore - requires DB inconsistency
	}

	topicID, err := s.chunkStore.DocumentTopicID(ctx, sourceChunk.DocumentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving topic: %v", err) // coverage:ignore - requires DB failure
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	if err := s.linkStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting link: %v", err) // coverage:ignore - requires DB failure after successful Get
	}

	return &pb.DeleteLinkResponse{}, nil
}

// ListLinks returns links for a chunk. Requires read permission on the chunk's topic.
func (s *LinkServer) ListLinks(ctx context.Context, req *pb.ListLinksRequest) (*pb.ListLinksResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetChunkId() == "" {
		return nil, status.Error(codes.InvalidArgument, "chunk_id is required")
	}

	chunk, err := s.chunkStore.Get(ctx, req.GetChunkId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "chunk not found")
	}

	topicID, err := s.chunkStore.DocumentTopicID(ctx, chunk.DocumentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving topic: %v", err) // coverage:ignore - requires DB failure
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	links, err := s.linkStore.ListByChunk(ctx, req.GetChunkId(), req.GetIncludeBacklinks())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing links: %v", err)
	}

	var pbLinks []*pb.Link
	for _, l := range links {
		pbLinks = append(pbLinks, storeLinkToProto(l))
	}

	return &pb.ListLinksResponse{Links: pbLinks}, nil
}

func storeLinkToProto(l *store.Link) *pb.Link {
	return &pb.Link{
		Id:            l.ID,
		SourceChunkId: l.SourceChunkID,
		TargetChunkId: l.TargetChunkID,
		LinkType:      storeLinkTypeToProto(l.LinkType),
		CreatedBy:     l.CreatedBy,
		Metadata:      mapToStruct(l.Metadata),
		CreatedAt:     timestamppb.New(l.CreatedAt),
	}
}

func protoLinkTypeToStore(lt pb.LinkType) string {
	switch lt {
	case pb.LinkType_LINK_TYPE_MANUAL:
		return "manual"
	case pb.LinkType_LINK_TYPE_AUTO:
		return "auto"
	case pb.LinkType_LINK_TYPE_COMPACTION_TRANSFER:
		return "compaction_transfer"
	default:
		return "manual"
	}
}

func storeLinkTypeToProto(lt string) pb.LinkType {
	switch lt {
	case "manual":
		return pb.LinkType_LINK_TYPE_MANUAL
	case "auto":
		return pb.LinkType_LINK_TYPE_AUTO
	case "compaction_transfer":
		return pb.LinkType_LINK_TYPE_COMPACTION_TRANSFER
	default:
		return pb.LinkType_LINK_TYPE_UNSPECIFIED
	}
}
