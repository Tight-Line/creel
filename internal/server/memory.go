package server

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

// EmbeddingProvider computes an embedding vector for a text string.
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

// MemoryServer implements the MemoryService gRPC service.
type MemoryServer struct {
	pb.UnimplementedMemoryServiceServer
	memStore *store.MemoryStore
	backend  vector.Backend
	embedder EmbeddingProvider
	jobStore *store.JobStore
}

// NewMemoryServer creates a new memory service.
// The embedder may be nil if no embedding provider is configured.
// The jobStore may be nil; when set, AddMemory routes through the maintenance worker.
func NewMemoryServer(memStore *store.MemoryStore, backend vector.Backend, embedder EmbeddingProvider, jobStore *store.JobStore) *MemoryServer {
	return &MemoryServer{
		memStore: memStore,
		backend:  backend,
		embedder: embedder,
		jobStore: jobStore,
	}
}

// GetMemory returns all active memories for the calling principal in a scope.
func (s *MemoryServer) GetMemory(ctx context.Context, req *pb.GetMemoryRequest) (*pb.GetMemoryResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetScope() == "" {
		return nil, status.Error(codes.InvalidArgument, "scope is required")
	}

	memories, err := s.memStore.GetByScope(ctx, p.ID, req.GetScope())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting memories: %v", err)
	}

	return &pb.GetMemoryResponse{Memories: storeMemoriesToProto(memories)}, nil
}

// SearchMemories performs semantic search within a principal's memories.
func (s *MemoryServer) SearchMemories(ctx context.Context, req *pb.SearchMemoriesRequest) (*pb.SearchMemoriesResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetScope() == "" {
		return nil, status.Error(codes.InvalidArgument, "scope is required")
	}

	topK := int(req.GetTopK())
	if topK <= 0 {
		topK = 10
	}

	// Determine query embedding: use provided embedding, or compute from text.
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

	// If we have a query embedding, search via vector backend.
	if len(queryEmbedding) > 0 {
		embeddingIDs, err := s.memStore.EmbeddingIDsByPrincipalScope(ctx, p.ID, req.GetScope())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "fetching embedding IDs: %v", err)
		}

		if len(embeddingIDs) == 0 {
			return &pb.SearchMemoriesResponse{}, nil
		}

		filter := vector.Filter{ChunkIDs: embeddingIDs}
		searchResults, err := s.backend.Search(ctx, queryEmbedding, filter, topK)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "searching: %v", err)
		}

		if len(searchResults) == 0 {
			return &pb.SearchMemoriesResponse{}, nil
		}

		// Batch-fetch memories by embedding IDs.
		resultEmbIDs := make([]string, len(searchResults))
		for i, sr := range searchResults {
			resultEmbIDs[i] = sr.ChunkID
		}

		memsByEmbID, err := s.memStore.GetByEmbeddingIDs(ctx, resultEmbIDs)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "fetching memories: %v", err)
		}

		var results []*pb.MemorySearchResult
		for _, sr := range searchResults {
			mem, ok := memsByEmbID[sr.ChunkID]
			if !ok {
				continue
			}
			results = append(results, &pb.MemorySearchResult{
				Memory: storeMemoryToProto(mem),
				Score:  sr.Score,
			})
		}

		return &pb.SearchMemoriesResponse{Results: results}, nil
	}

	// No embedding available; fall back to returning all active memories.
	memories, err := s.memStore.GetByScope(ctx, p.ID, req.GetScope())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing memories: %v", err)
	}

	results := make([]*pb.MemorySearchResult, len(memories))
	for i, m := range memories {
		results[i] = &pb.MemorySearchResult{
			Memory: storeMemoryToProto(m),
			Score:  0,
		}
	}

	return &pb.SearchMemoriesResponse{Results: results}, nil
}

// AddMemory queues a memory for creation via the maintenance worker, which
// handles LLM-based deduplication (ADD/UPDATE/DELETE/NOOP). Returns a job ID.
func (s *MemoryServer) AddMemory(ctx context.Context, req *pb.AddMemoryRequest) (*pb.AddMemoryResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetContent() == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	if s.jobStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "job store not configured")
	}

	scope := req.GetScope()
	if scope == "" {
		scope = "default"
	}

	progress := map[string]any{
		"candidate_fact": req.GetContent(),
		"principal":      p.ID,
		"scope":          scope,
	}
	if req.GetSubject() != "" {
		progress["subject"] = req.GetSubject()
	}
	if req.GetPredicate() != "" {
		progress["predicate"] = req.GetPredicate()
	}
	if req.GetObject() != "" {
		progress["object"] = req.GetObject()
	}

	job, err := s.jobStore.CreateDocless(ctx, "memory_maintenance", progress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating maintenance job: %v", err)
	}

	return &pb.AddMemoryResponse{JobId: job.ID}, nil
}

// UpdateMemory updates a memory's content and metadata.
func (s *MemoryServer) UpdateMemory(ctx context.Context, req *pb.UpdateMemoryRequest) (*pb.Memory, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Verify ownership.
	existing, err := s.memStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "memory not found")
	}
	if existing.Principal != p.ID {
		return nil, status.Error(codes.PermissionDenied, "not owner of this memory")
	}

	content := req.GetContent()
	if content == "" {
		content = existing.Content
	}
	meta := structToMap(req.GetMetadata())
	if meta == nil {
		meta = existing.Metadata
	}

	updated, err := s.memStore.Update(ctx, req.GetId(), content, meta)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating memory: %v", err)
	}

	// Re-compute embedding if provider is available and content changed.
	if s.embedder != nil && s.backend != nil && content != existing.Content {
		embedding, embErr := s.embedder.Embed(ctx, updated.Content)
		if embErr == nil {
			embeddingID := fmt.Sprintf("mem_%s", updated.ID)
			storeErr := s.backend.Store(ctx, embeddingID, embedding, nil)
			if storeErr == nil {
				_ = s.memStore.SetEmbeddingID(ctx, updated.ID, embeddingID)
				updated.EmbeddingID = &embeddingID
			}
		}
	}

	return storeMemoryToProto(updated), nil
}

// DeleteMemory soft-deletes a memory by marking it as invalidated.
func (s *MemoryServer) DeleteMemory(ctx context.Context, req *pb.DeleteMemoryRequest) (*pb.DeleteMemoryResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Verify ownership.
	existing, err := s.memStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "memory not found")
	}
	if existing.Principal != p.ID {
		return nil, status.Error(codes.PermissionDenied, "not owner of this memory")
	}

	if err := s.memStore.Invalidate(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "invalidating memory: %v", err)
	}

	return &pb.DeleteMemoryResponse{}, nil
}

// ListMemories returns memories for a scope, optionally including invalidated ones.
func (s *MemoryServer) ListMemories(ctx context.Context, req *pb.ListMemoriesRequest) (*pb.ListMemoriesResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetScope() == "" {
		return nil, status.Error(codes.InvalidArgument, "scope is required")
	}

	memories, err := s.memStore.ListByScope(ctx, p.ID, req.GetScope(), req.GetIncludeInvalidated())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing memories: %v", err)
	}

	return &pb.ListMemoriesResponse{Memories: storeMemoriesToProto(memories)}, nil
}

// ListScopes returns all scopes the calling principal has memories in.
func (s *MemoryServer) ListScopes(ctx context.Context, _ *pb.ListScopesRequest) (*pb.ListScopesResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	scopes, err := s.memStore.ListScopes(ctx, p.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing scopes: %v", err)
	}

	return &pb.ListScopesResponse{Scopes: scopes}, nil
}

func storeMemoryToProto(m *store.Memory) *pb.Memory {
	mem := &pb.Memory{
		Id:        m.ID,
		Principal: m.Principal,
		Scope:     m.Scope,
		Content:   m.Content,
		Status:    m.Status,
		Metadata:  mapToStruct(m.Metadata),
		CreatedAt: timestamppb.New(m.CreatedAt),
		UpdatedAt: timestamppb.New(m.UpdatedAt),
	}
	if m.Subject != nil {
		mem.Subject = *m.Subject
	}
	if m.Predicate != nil {
		mem.Predicate = *m.Predicate
	}
	if m.Object != nil {
		mem.Object = *m.Object
	}
	if m.SourceChunkID != nil {
		mem.SourceChunkId = *m.SourceChunkID
	}
	if m.InvalidatedAt != nil {
		mem.InvalidatedAt = timestamppb.New(*m.InvalidatedAt)
	}
	return mem
}

func storeMemoriesToProto(memories []*store.Memory) []*pb.Memory {
	result := make([]*pb.Memory, len(memories))
	for i, m := range memories {
		result[i] = storeMemoryToProto(m)
	}
	return result
}
