package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// ChunkServer implements the ChunkService gRPC service.
type ChunkServer struct {
	pb.UnimplementedChunkServiceServer
	chunkStore *store.ChunkStore
	docStore   *store.DocumentStore
	backend    vector.Backend
	authorizer auth.Authorizer
}

// NewChunkServer creates a new chunk service.
func NewChunkServer(chunkStore *store.ChunkStore, docStore *store.DocumentStore, backend vector.Backend, authorizer auth.Authorizer) *ChunkServer {
	return &ChunkServer{
		chunkStore: chunkStore,
		docStore:   docStore,
		backend:    backend,
		authorizer: authorizer,
	}
}

// IngestChunks creates chunks and stores their embeddings. Requires write permission.
func (s *ChunkServer) IngestChunks(ctx context.Context, req *pb.IngestChunksRequest) (*pb.IngestChunksResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetDocumentId() == "" || len(req.GetChunks()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "document_id and chunks are required")
	}

	// Check write permission on the document's topic.
	topicID, err := s.chunkStore.DocumentTopicID(ctx, req.GetDocumentId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	var pbChunks []*pb.Chunk
	for _, ci := range req.GetChunks() {
		meta := structToMap(ci.GetMetadata())
		c, err := s.chunkStore.Create(ctx, req.GetDocumentId(), ci.GetContent(), int(ci.GetSequence()), meta)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "creating chunk: %v", err)
		}

		// Store embedding in vector backend if provided.
		if len(ci.GetEmbedding()) > 0 {
			if err := s.backend.Store(ctx, c.ID, ci.GetEmbedding(), meta); err != nil {
				return nil, status.Errorf(codes.Internal, "storing embedding: %v", err)
			}
			if err := s.chunkStore.SetEmbeddingID(ctx, c.ID, c.ID); err != nil {
				return nil, status.Errorf(codes.Internal, "setting embedding ID: %v", err)
			}
			embID := c.ID
			c.EmbeddingID = &embID
		}

		pbChunks = append(pbChunks, storeChunkToProto(c))
	}

	return &pb.IngestChunksResponse{Chunks: pbChunks}, nil
}

// GetChunk retrieves a chunk. Requires read permission on its topic.
func (s *ChunkServer) GetChunk(ctx context.Context, req *pb.GetChunkRequest) (*pb.Chunk, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.chunkStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "chunk not found")
	}

	topicID, err := s.chunkStore.DocumentTopicID(ctx, c.DocumentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving topic: %v", err)
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	return storeChunkToProto(c), nil
}

// DeleteChunk deletes a chunk and its embedding. Requires admin permission.
func (s *ChunkServer) DeleteChunk(ctx context.Context, req *pb.DeleteChunkRequest) (*pb.DeleteChunkResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.chunkStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "chunk not found")
	}

	topicID, err := s.chunkStore.DocumentTopicID(ctx, c.DocumentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving topic: %v", err)
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// Delete from vector backend first.
	if err := s.backend.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting embedding: %v", err)
	}

	if err := s.chunkStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting chunk: %v", err)
	}

	return &pb.DeleteChunkResponse{}, nil
}

func storeChunkToProto(c *store.Chunk) *pb.Chunk {
	chunk := &pb.Chunk{
		Id:         c.ID,
		DocumentId: c.DocumentID,
		Sequence:   int32(c.Sequence),
		Content:    c.Content,
		Status:     stringToChunkStatus(c.Status),
		Metadata:   mapToStruct(c.Metadata),
		CreatedAt:  timestamppb.New(c.CreatedAt),
	}
	if c.EmbeddingID != nil {
		chunk.EmbeddingId = *c.EmbeddingID
	}
	if c.CompactedBy != nil {
		chunk.CompactedBy = *c.CompactedBy
	}
	return chunk
}

func stringToChunkStatus(s string) pb.ChunkStatus {
	switch s {
	case "active":
		return pb.ChunkStatus_CHUNK_STATUS_ACTIVE
	case "compacted":
		return pb.ChunkStatus_CHUNK_STATUS_COMPACTED
	default:
		return pb.ChunkStatus_CHUNK_STATUS_UNSPECIFIED
	}
}
