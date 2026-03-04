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
func NewRetrievalServer(searcher *retrieval.Searcher) *RetrievalServer {
	return &RetrievalServer{searcher: searcher}
}

// Search performs ACL-filtered similarity search.
func (s *RetrievalServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if len(req.GetQueryEmbedding()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query_embedding is required")
	}

	topK := int(req.GetTopK())
	if topK <= 0 {
		topK = 10
	}

	results, err := s.searcher.Search(ctx, p, req.GetTopicIds(), req.GetQueryEmbedding(), topK)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search: %v", err)
	}

	pbResults := make([]*pb.SearchResult, len(results))
	for i, r := range results {
		pbResults[i] = &pb.SearchResult{
			Chunk:      storeChunkToProto(r.Chunk),
			DocumentId: r.Chunk.DocumentID,
			TopicId:    r.TopicID,
			Score:      r.Score,
		}
	}

	return &pb.SearchResponse{Results: pbResults}, nil
}

// GetContext returns Unimplemented (Phase 3).
func (s *RetrievalServer) GetContext(_ context.Context, _ *pb.GetContextRequest) (*pb.GetContextResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetContext is a Phase 3 feature")
}
