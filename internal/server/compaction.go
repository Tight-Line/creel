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

// CompactionServer implements the CompactionService gRPC service.
type CompactionServer struct {
	pb.UnimplementedCompactionServiceServer
	chunkStore      *store.ChunkStore
	linkStore       *store.LinkStore
	compactionStore *store.CompactionStore
	docStore        *store.DocumentStore
	jobStore        *store.JobStore
	vectorBackend   vector.Backend
	authorizer      auth.Authorizer
}

// NewCompactionServer creates a new compaction service.
func NewCompactionServer(
	chunkStore *store.ChunkStore,
	linkStore *store.LinkStore,
	compactionStore *store.CompactionStore,
	docStore *store.DocumentStore,
	jobStore *store.JobStore,
	vectorBackend vector.Backend,
	authorizer auth.Authorizer,
) *CompactionServer {
	return &CompactionServer{
		chunkStore:      chunkStore,
		linkStore:       linkStore,
		compactionStore: compactionStore,
		docStore:        docStore,
		jobStore:        jobStore,
		vectorBackend:   vectorBackend,
		authorizer:      authorizer,
	}
}

// Compact performs synchronous, manual compaction. The caller supplies the summary content
// and optionally a pre-computed embedding.
func (s *CompactionServer) Compact(ctx context.Context, req *pb.CompactRequest) (*pb.CompactResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetDocumentId() == "" || len(req.GetChunkIds()) == 0 || req.GetSummaryContent() == "" {
		return nil, status.Error(codes.InvalidArgument, "document_id, chunk_ids, and summary_content are required")
	}

	// Auth: require write on the document's topic.
	topicID, err := s.docStore.TopicIDForDocument(ctx, req.GetDocumentId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// Get next sequence number for the summary chunk.
	nextSeq, err := s.chunkStore.NextSequence(ctx, req.GetDocumentId())
	// coverage:ignore - requires DB failure after successful auth check
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting next sequence: %v", err)
	}

	meta := structToMap(req.GetSummaryMetadata())
	if meta == nil {
		meta = map[string]any{}
	}
	meta["compaction_source_count"] = len(req.GetChunkIds())

	// Create the summary chunk.
	summaryChunk, err := s.chunkStore.Create(ctx, req.GetDocumentId(), req.GetSummaryContent(), nextSeq, meta)
	// coverage:ignore - requires DB failure after successful operations
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating summary chunk: %v", err)
	}

	// coverage:ignore - embedding branch requires caller to provide pre-computed embedding
	if len(req.GetSummaryEmbedding()) > 0 {
		// coverage:ignore - requires vector backend failure
		if err := s.vectorBackend.Store(ctx, summaryChunk.ID, req.GetSummaryEmbedding(), meta); err != nil {
			return nil, status.Errorf(codes.Internal, "storing summary embedding: %v", err)
		}
		// coverage:ignore - requires DB failure after vector store
		if err := s.chunkStore.SetEmbeddingID(ctx, summaryChunk.ID, summaryChunk.ID); err != nil {
			return nil, status.Errorf(codes.Internal, "setting embedding ID: %v", err)
		}
	}

	// Transfer links from source chunks to the summary chunk.
	for _, chunkID := range req.GetChunkIds() {
		// coverage:ignore - requires DB failure mid-transfer
		if _, err := s.linkStore.TransferLinks(ctx, chunkID, summaryChunk.ID); err != nil {
			return nil, status.Errorf(codes.Internal, "transferring links: %v", err)
		}
	}

	// coverage:ignore - requires vector backend failure
	if err := s.vectorBackend.DeleteBatch(ctx, req.GetChunkIds()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting source embeddings: %v", err)
	}

	// coverage:ignore - requires DB failure after successful operations
	if err := s.chunkStore.MarkCompacted(ctx, req.GetChunkIds(), summaryChunk.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "marking chunks as compacted: %v", err)
	}

	// coverage:ignore - requires DB failure after successful operations
	if _, err := s.compactionStore.Create(ctx, summaryChunk.ID, req.GetChunkIds(), req.GetDocumentId(), p.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "creating compaction record: %v", err)
	}

	// Re-read the summary chunk to get the final state.
	summaryChunk, err = s.chunkStore.Get(ctx, summaryChunk.ID)
	// coverage:ignore - requires DB failure after successful create
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reading summary chunk: %v", err)
	}

	return &pb.CompactResponse{
		SummaryChunk:   storeChunkToProto(summaryChunk),
		CompactedCount: int32(len(req.GetChunkIds())),
	}, nil
}

// Uncompact reverses a compaction by restoring source chunks and removing the summary chunk.
func (s *CompactionServer) Uncompact(ctx context.Context, req *pb.UncompactRequest) (*pb.UncompactResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetSummaryChunkId() == "" {
		return nil, status.Error(codes.InvalidArgument, "summary_chunk_id is required")
	}

	// Look up the compaction record.
	record, err := s.compactionStore.GetBySummaryChunkID(ctx, req.GetSummaryChunkId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "compaction record not found")
	}

	// Auth: require write on the document's topic.
	topicID, err := s.docStore.TopicIDForDocument(ctx, record.DocumentID)
	// coverage:ignore - requires DB inconsistency (record exists but document deleted)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolving document topic: %v", err)
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// Restore compacted chunks.
	restoredIDs, err := s.chunkStore.RestoreCompacted(ctx, req.GetSummaryChunkId())
	// coverage:ignore - requires DB failure after successful auth check
	if err != nil {
		return nil, status.Errorf(codes.Internal, "restoring chunks: %v", err)
	}

	// coverage:ignore - requires vector backend failure
	if err := s.vectorBackend.Delete(ctx, req.GetSummaryChunkId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting summary embedding: %v", err)
	}

	// coverage:ignore - requires DB failure after vector delete
	if err := s.chunkStore.Delete(ctx, req.GetSummaryChunkId()); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting summary chunk: %v", err)
	}

	// coverage:ignore - requires DB failure after chunk delete
	if err := s.compactionStore.Delete(ctx, record.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "deleting compaction record: %v", err)
	}

	// coverage:ignore - best-effort; chunks are already restored
	if _, err := s.jobStore.Create(ctx, record.DocumentID, "embedding"); err != nil {
		fmt.Printf("warning: failed to enqueue embedding job after uncompact: %v\n", err)
	}

	// Read back the restored chunks.
	restoredChunks := make(map[string]*store.Chunk, len(restoredIDs))
	if len(restoredIDs) > 0 {
		restoredChunks, err = s.chunkStore.GetMultiple(ctx, restoredIDs)
		// coverage:ignore - requires DB failure after successful restore
		if err != nil {
			return nil, status.Errorf(codes.Internal, "reading restored chunks: %v", err)
		}
	}

	var pbChunks []*pb.Chunk
	for _, id := range restoredIDs {
		if c, ok := restoredChunks[id]; ok {
			pbChunks = append(pbChunks, storeChunkToProto(c))
		}
	}

	return &pb.UncompactResponse{RestoredChunks: pbChunks}, nil
}

// RequestCompaction enqueues a background compaction job.
func (s *CompactionServer) RequestCompaction(ctx context.Context, req *pb.RequestCompactionRequest) (*pb.RequestCompactionResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetDocumentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "document_id is required")
	}

	// Auth: require write on the document's topic.
	topicID, err := s.docStore.TopicIDForDocument(ctx, req.GetDocumentId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionWrite); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	progress := map[string]any{
		"requested_by": p.ID,
	}
	if len(req.GetChunkIds()) > 0 {
		progress["chunk_ids"] = req.GetChunkIds()
	}

	job, err := s.jobStore.CreateWithProgress(ctx, req.GetDocumentId(), "compaction", progress)
	// coverage:ignore - requires DB failure after successful auth check
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating compaction job: %v", err)
	}

	return &pb.RequestCompactionResponse{JobId: job.ID}, nil
}

// GetCompactionHistory returns compaction records for a document.
func (s *CompactionServer) GetCompactionHistory(ctx context.Context, req *pb.GetCompactionHistoryRequest) (*pb.GetCompactionHistoryResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetDocumentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "document_id is required")
	}

	// Auth: require read on the document's topic.
	topicID, err := s.docStore.TopicIDForDocument(ctx, req.GetDocumentId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}
	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	records, err := s.compactionStore.ListByDocument(ctx, req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing compaction records: %v", err)
	}

	var pbRecords []*pb.CompactionRecord
	for _, r := range records {
		pbRecords = append(pbRecords, &pb.CompactionRecord{
			Id:             r.ID,
			SummaryChunkId: r.SummaryChunkID,
			SourceChunkIds: r.SourceChunkIDs,
			DocumentId:     r.DocumentID,
			CreatedBy:      r.CreatedBy,
			CreatedAt:      timestamppb.New(r.CreatedAt),
		})
	}

	return &pb.GetCompactionHistoryResponse{Records: pbRecords}, nil
}
