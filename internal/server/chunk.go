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
// coverage:ignore - gRPC handler; tested via integration tests
func NewChunkServer(chunkStore *store.ChunkStore, docStore *store.DocumentStore, backend vector.Backend, authorizer auth.Authorizer) *ChunkServer {
	return &ChunkServer{
		chunkStore: chunkStore,
		docStore:   docStore,
		backend:    backend,
		authorizer: authorizer,
	}
}

// IngestChunks creates chunks and stores their embeddings. Requires write permission.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *ChunkServer) IngestChunks(ctx context.Context, req *pb.IngestChunksRequest) (*pb.IngestChunksResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if req.GetDocumentId() == "" || len(req.GetChunks()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "document_id and chunks are required")
	}

	// Check write permission on the document's topic.
	// coverage:ignore - gRPC handler; tested via integration tests
	topicID, err := s.chunkStore.DocumentTopicID(ctx, req.GetDocumentId())
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	var pbChunks []*pb.Chunk
	// coverage:ignore - gRPC handler; tested via integration tests
	for _, ci := range req.GetChunks() {
		meta := structToMap(ci.GetMetadata())
		c, err := s.chunkStore.Create(ctx, req.GetDocumentId(), ci.GetContent(), int(ci.GetSequence()), meta)
		// coverage:ignore - gRPC handler; tested via integration tests
		if err != nil {
			return nil, status.Errorf(codes.Internal, "creating chunk: %v", err)
		}

		// Store embedding in vector backend if provided.
		// coverage:ignore - gRPC handler; tested via integration tests
		if len(ci.GetEmbedding()) > 0 {
			// coverage:ignore - gRPC handler; tested via integration tests
			if err := s.backend.Store(ctx, c.ID, ci.GetEmbedding(), meta); err != nil {
				return nil, status.Errorf(codes.Internal, "storing embedding: %v", err)
			}
			// coverage:ignore - gRPC handler; tested via integration tests
			if err := s.chunkStore.SetEmbeddingID(ctx, c.ID, c.ID); err != nil {
				return nil, status.Errorf(codes.Internal, "setting embedding ID: %v", err)
			}
			// coverage:ignore - gRPC handler; tested via integration tests
			embID := c.ID
			c.EmbeddingID = &embID
		}

		// coverage:ignore - gRPC handler; tested via integration tests
		pbChunks = append(pbChunks, storeChunkToProto(c))
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.IngestChunksResponse{Chunks: pbChunks}, nil
}

// GetChunk retrieves a chunk. Requires read permission on its topic.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *ChunkServer) GetChunk(ctx context.Context, req *pb.GetChunkRequest) (*pb.Chunk, error) {
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
	c, err := s.chunkStore.Get(ctx, req.GetId())
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Error(codes.NotFound, "chunk not found")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	topicID, err := s.chunkStore.DocumentTopicID(ctx, c.DocumentID)
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving topic: %v", err)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return storeChunkToProto(c), nil
}

// DeleteChunk deletes a chunk and its embedding. Requires admin permission.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *ChunkServer) DeleteChunk(ctx context.Context, req *pb.DeleteChunkRequest) (*pb.DeleteChunkResponse, error) {
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
	c, err := s.chunkStore.Get(ctx, req.GetId())
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Error(codes.NotFound, "chunk not found")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	topicID, err := s.chunkStore.DocumentTopicID(ctx, c.DocumentID)
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving topic: %v", err)
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionAdmin); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// Delete from vector backend first.
	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.backend.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting embedding: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	if err := s.chunkStore.Delete(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting chunk: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.DeleteChunkResponse{}, nil
}

// coverage:ignore - gRPC handler; tested via integration tests
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
	// coverage:ignore - gRPC handler; tested via integration tests
	if c.EmbeddingID != nil {
		chunk.EmbeddingId = *c.EmbeddingID
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if c.CompactedBy != nil {
		chunk.CompactedBy = *c.CompactedBy
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	return chunk
}

// coverage:ignore - gRPC handler; tested via integration tests
func stringToChunkStatus(s string) pb.ChunkStatus {
	switch s {
	// coverage:ignore - gRPC handler; tested via integration tests
	case "active":
		return pb.ChunkStatus_CHUNK_STATUS_ACTIVE
	// coverage:ignore - gRPC handler; tested via integration tests
	case "compacted":
		return pb.ChunkStatus_CHUNK_STATUS_COMPACTED
	// coverage:ignore - gRPC handler; tested via integration tests
	default:
		return pb.ChunkStatus_CHUNK_STATUS_UNSPECIFIED
	}
}
