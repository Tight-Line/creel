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
	embedder       EmbeddingProvider
}

// NewRetrievalServer creates a new retrieval service.
// The embedder is optional; if nil, query_text is not supported and clients
// must provide query_embedding directly.
func NewRetrievalServer(searcher *retrieval.Searcher, contextFetcher *retrieval.ContextFetcher, embedder EmbeddingProvider) *RetrievalServer {
	return &RetrievalServer{searcher: searcher, contextFetcher: contextFetcher, embedder: embedder}
}

// Search performs ACL-filtered similarity search.
func (s *RetrievalServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	queryEmbedding := req.GetQueryEmbedding()
	if len(queryEmbedding) == 0 && req.GetQueryText() != "" {
		if s.embedder == nil {
			return nil, status.Error(codes.FailedPrecondition, "embedding provider not configured; provide query_embedding directly")
		}
		var err error
		queryEmbedding, err = s.embedder.Embed(ctx, req.GetQueryText())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "computing embedding: %v", err)
		}
	}
	if len(queryEmbedding) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query_embedding or query_text is required")
	}

	topK := int(req.GetTopK())
	if topK <= 0 {
		topK = 10
	}

	var metadataFilter map[string]any
	if mf := req.GetMetadataFilter(); mf != nil {
		metadataFilter = mf.AsMap()
	}

	results, err := s.searcher.Search(ctx, p, req.GetTopicIds(), queryEmbedding, topK, metadataFilter, req.GetExcludeDocumentIds())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search: %v", err)
	}

	pbResults := make([]*pb.SearchResult, len(results))
	for i, r := range results {
		pbResults[i] = &pb.SearchResult{
			Chunk:            storeChunkToProto(r.Chunk),
			DocumentId:       r.Chunk.DocumentID,
			TopicId:          r.TopicID,
			Score:            r.Score,
			DocumentCitation: storeDocToCitation(r.Document),
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
