package server

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/retrieval"
)

// RetrievalServer implements the RetrievalService gRPC service.
type RetrievalServer struct {
	pb.UnimplementedRetrievalServiceServer
	searcher       *retrieval.Searcher
	contextFetcher *retrieval.ContextFetcher
}

// NewRetrievalServer creates a new retrieval service.
func NewRetrievalServer(searcher *retrieval.Searcher, contextFetcher *retrieval.ContextFetcher) *RetrievalServer {
	return &RetrievalServer{searcher: searcher, contextFetcher: contextFetcher}
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

	var metadataFilter map[string]any
	if mf := req.GetMetadataFilter(); mf != nil {
		metadataFilter = mf.AsMap()
	}

	results, err := s.searcher.Search(ctx, p, req.GetTopicIds(), req.GetQueryEmbedding(), topK, metadataFilter, req.GetExcludeDocumentIds())
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

// GetContext returns chunks from a single document in sequence order.
func (s *RetrievalServer) GetContext(ctx context.Context, req *pb.GetContextRequest) (*pb.GetContextResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetDocumentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "document_id is required")
	}

	lastN := int(req.GetLastN())
	var since time.Time
	if req.GetSince() != nil {
		since = req.GetSince().AsTime()
	}

	chunks, err := s.contextFetcher.GetContext(ctx, p, req.GetDocumentId(), lastN, since)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get context: %v", err)
	}

	pbChunks := make([]*pb.Chunk, len(chunks))
	for i, c := range chunks {
		pbChunks[i] = storeChunkToProto(c)
	}

	return &pb.GetContextResponse{Chunks: pbChunks}, nil
}
