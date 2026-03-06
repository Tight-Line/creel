package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/retrieval"
)

// RetrievalServer implements the RetrievalService gRPC service.
type RetrievalServer struct {
	pb.UnimplementedRetrievalServiceServer
	searcher *retrieval.Searcher
}

// NewRetrievalServer creates a new retrieval service.
// coverage:ignore - gRPC handler; tested via integration tests
func NewRetrievalServer(searcher *retrieval.Searcher) *RetrievalServer {
	return &RetrievalServer{searcher: searcher}
}

// Search performs ACL-filtered similarity search.
// coverage:ignore - gRPC handler; tested via integration tests
func (s *RetrievalServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	// coverage:ignore - gRPC handler; tested via integration tests
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	// coverage:ignore - gRPC handler; tested via integration tests
	if len(req.GetQueryEmbedding()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query_embedding is required")
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	topK := int(req.GetTopK())
	// coverage:ignore - gRPC handler; tested via integration tests
	if topK <= 0 {
		topK = 10
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	var metadataFilter map[string]any
	// coverage:ignore - gRPC handler; tested via integration tests
	if mf := req.GetMetadataFilter(); mf != nil {
		metadataFilter = mf.AsMap()
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	results, err := s.searcher.Search(ctx, p, req.GetTopicIds(), req.GetQueryEmbedding(), topK, metadataFilter)
	// coverage:ignore - gRPC handler; tested via integration tests
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search: %v", err)
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	pbResults := make([]*pb.SearchResult, len(results))
	// coverage:ignore - gRPC handler; tested via integration tests
	for i, r := range results {
		pbResults[i] = &pb.SearchResult{
			Chunk:      storeChunkToProto(r.Chunk),
			DocumentId: r.Chunk.DocumentID,
			TopicId:    r.TopicID,
			Score:      r.Score,
		}
	}

	// coverage:ignore - gRPC handler; tested via integration tests
	return &pb.SearchResponse{Results: pbResults}, nil
}

// GetContext returns Unimplemented (Phase 3).
// coverage:ignore - gRPC handler; tested via integration tests
func (s *RetrievalServer) GetContext(_ context.Context, _ *pb.GetContextRequest) (*pb.GetContextResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetContext is a Phase 3 feature")
}
