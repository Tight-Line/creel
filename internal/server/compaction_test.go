package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

func compactionAuthedCtx() context.Context {
	return auth.ContextWithPrincipal(context.Background(), &auth.Principal{ID: "user:test"})
}

func compactionServerWithMock(db *mockDBTX, authz *mockAuthorizer, backend *mockBackend) *CompactionServer {
	if backend == nil {
		backend = &mockBackend{}
	}
	return NewCompactionServer(
		store.NewChunkStore(db),
		store.NewLinkStore(db),
		store.NewCompactionStore(db),
		store.NewDocumentStore(db),
		store.NewJobStore(db),
		backend,
		authz,
	)
}

// --- Compact ---

func TestCompactionServer_Compact_Unauthenticated(t *testing.T) {
	srv := compactionServerWithMock(&mockDBTX{}, &mockAuthorizer{}, nil)
	_, err := srv.Compact(context.Background(), &pb.CompactRequest{
		DocumentId: "d1", ChunkIds: []string{"c1"}, SummaryContent: "sum",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestCompactionServer_Compact_MissingFields(t *testing.T) {
	srv := compactionServerWithMock(&mockDBTX{}, &mockAuthorizer{}, nil)
	_, err := srv.Compact(compactionAuthedCtx(), &pb.CompactRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestCompactionServer_Compact_DocumentNotFound(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("document not found")}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, nil)
	_, err := srv.Compact(compactionAuthedCtx(), &pb.CompactRequest{
		DocumentId: "bad", ChunkIds: []string{"c1"}, SummaryContent: "sum",
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestCompactionServer_Compact_PermissionDenied(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &compactTopicRow{topicID: "t1"}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{checkErr: errors.New("denied")}, nil)
	_, err := srv.Compact(compactionAuthedCtx(), &pb.CompactRequest{
		DocumentId: "d1", ChunkIds: []string{"c1"}, SummaryContent: "sum",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

// --- Uncompact ---

func TestCompactionServer_Uncompact_Unauthenticated(t *testing.T) {
	srv := compactionServerWithMock(&mockDBTX{}, &mockAuthorizer{}, nil)
	_, err := srv.Uncompact(context.Background(), &pb.UncompactRequest{SummaryChunkId: "x"})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestCompactionServer_Uncompact_MissingID(t *testing.T) {
	srv := compactionServerWithMock(&mockDBTX{}, &mockAuthorizer{}, nil)
	_, err := srv.Uncompact(compactionAuthedCtx(), &pb.UncompactRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestCompactionServer_Uncompact_RecordNotFound(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("compaction record not found")}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, nil)
	_, err := srv.Uncompact(compactionAuthedCtx(), &pb.UncompactRequest{SummaryChunkId: "bad"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestCompactionServer_Uncompact_PermissionDenied(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				return &compactRecordRow{id: "r1", summaryChunk: "sc1", docID: "d1"}
			}
			return &compactTopicRow{topicID: "t1"}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{checkErr: errors.New("denied")}, nil)
	_, err := srv.Uncompact(compactionAuthedCtx(), &pb.UncompactRequest{SummaryChunkId: "sc1"})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

// --- RequestCompaction ---

func TestCompactionServer_RequestCompaction_Unauthenticated(t *testing.T) {
	srv := compactionServerWithMock(&mockDBTX{}, &mockAuthorizer{}, nil)
	_, err := srv.RequestCompaction(context.Background(), &pb.RequestCompactionRequest{DocumentId: "d1"})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestCompactionServer_RequestCompaction_MissingDocID(t *testing.T) {
	srv := compactionServerWithMock(&mockDBTX{}, &mockAuthorizer{}, nil)
	_, err := srv.RequestCompaction(compactionAuthedCtx(), &pb.RequestCompactionRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestCompactionServer_RequestCompaction_DocumentNotFound(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("document not found")}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, nil)
	_, err := srv.RequestCompaction(compactionAuthedCtx(), &pb.RequestCompactionRequest{DocumentId: "bad"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestCompactionServer_RequestCompaction_PermissionDenied(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &compactTopicRow{topicID: "t1"}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{checkErr: errors.New("denied")}, nil)
	_, err := srv.RequestCompaction(compactionAuthedCtx(), &pb.RequestCompactionRequest{DocumentId: "d1"})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

// --- GetCompactionHistory ---

func TestCompactionServer_GetCompactionHistory_Unauthenticated(t *testing.T) {
	srv := compactionServerWithMock(&mockDBTX{}, &mockAuthorizer{}, nil)
	_, err := srv.GetCompactionHistory(context.Background(), &pb.GetCompactionHistoryRequest{DocumentId: "d1"})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestCompactionServer_GetCompactionHistory_MissingDocID(t *testing.T) {
	srv := compactionServerWithMock(&mockDBTX{}, &mockAuthorizer{}, nil)
	_, err := srv.GetCompactionHistory(compactionAuthedCtx(), &pb.GetCompactionHistoryRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestCompactionServer_GetCompactionHistory_DocumentNotFound(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("document not found")}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, nil)
	_, err := srv.GetCompactionHistory(compactionAuthedCtx(), &pb.GetCompactionHistoryRequest{DocumentId: "bad"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestCompactionServer_GetCompactionHistory_PermissionDenied(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &compactTopicRow{topicID: "t1"}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{checkErr: errors.New("denied")}, nil)
	_, err := srv.GetCompactionHistory(compactionAuthedCtx(), &pb.GetCompactionHistoryRequest{DocumentId: "d1"})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

func TestCompactionServer_GetCompactionHistory_EmptyList(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			return &compactTopicRow{topicID: "t1"}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &compactEmptyRows{}, nil
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, nil)
	resp, err := srv.GetCompactionHistory(compactionAuthedCtx(), &pb.GetCompactionHistoryRequest{DocumentId: "d1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetRecords()) != 0 {
		t.Errorf("expected 0 records, got %d", len(resp.GetRecords()))
	}
}

func TestCompactionServer_GetCompactionHistory_QueryError(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			return &compactTopicRow{topicID: "t1"}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, nil)
	_, err := srv.GetCompactionHistory(compactionAuthedCtx(), &pb.GetCompactionHistoryRequest{DocumentId: "d1"})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

// --- Success path tests ---

func TestCompactionServer_Compact_Success(t *testing.T) {
	queryRowCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			queryRowCount++
			switch queryRowCount {
			case 1:
				// TopicIDForDocument
				return &compactTopicRow{topicID: "t1"}
			case 2:
				// NextSequence (MAX(sequence))
				return &compactIntRow{val: 5}
			case 3:
				// ChunkStore.Create (INSERT INTO chunks)
				return &compactChunkRow{id: "sc1", docID: "d1"}
			case 4:
				// JobStore.Create (embedding job; best-effort, error ignored)
				return &mockRow{err: errors.New("ignored")}
			case 5:
				// CompactionStore.Create (INSERT INTO compaction_records)
				return &compactRecordRow{id: "r1", summaryChunk: "sc1", docID: "d1"}
			case 6:
				// ChunkStore.Get (re-read summary chunk)
				return &compactChunkRow{id: "sc1", docID: "d1"}
			default:
				return &mockRow{err: errors.New("unexpected query")}
			}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 2"), nil
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, &mockBackend{})
	resp, err := srv.Compact(compactionAuthedCtx(), &pb.CompactRequest{
		DocumentId:     "d1",
		ChunkIds:       []string{"c1", "c2"},
		SummaryContent: "compacted summary",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCompactedCount() != 2 {
		t.Errorf("expected compacted_count=2, got %d", resp.GetCompactedCount())
	}
	if resp.GetSummaryChunk().GetId() != "sc1" {
		t.Errorf("expected summary chunk id=sc1, got %s", resp.GetSummaryChunk().GetId())
	}
}

func TestCompactionServer_Uncompact_Success(t *testing.T) {
	queryRowCount := 0
	queryCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			queryRowCount++
			switch queryRowCount {
			case 1:
				// CompactionStore.GetBySummaryChunkID
				return &compactRecordRow{id: "r1", summaryChunk: "sc1", docID: "d1"}
			case 2:
				// TopicIDForDocument
				return &compactTopicRow{topicID: "t1"}
			case 3:
				// JobStore.Create (embedding job; returns job row)
				return &compactJobRow{id: "j1"}
			default:
				return &mockRow{err: errors.New("unexpected query")}
			}
		},
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			queryCount++
			if queryCount == 1 {
				// RestoreCompacted returns IDs
				return &compactIDRows{ids: []string{"c1", "c2"}}, nil
			}
			// GetMultiple returns chunk data
			return &compactRestoredChunkRows{chunks: []compactChunkData{
				{id: "c1", docID: "d1"},
				{id: "c2", docID: "d1"},
			}}, nil
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, &mockBackend{})
	resp, err := srv.Uncompact(compactionAuthedCtx(), &pb.UncompactRequest{SummaryChunkId: "sc1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetRestoredChunks()) != 2 {
		t.Errorf("expected 2 restored chunks, got %d", len(resp.GetRestoredChunks()))
	}
}

func TestCompactionServer_RequestCompaction_Success(t *testing.T) {
	queryRowCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			queryRowCount++
			if queryRowCount == 1 {
				// TopicIDForDocument
				return &compactTopicRow{topicID: "t1"}
			}
			// JobStore.CreateWithProgress
			return &compactJobRow{id: "j1"}
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, nil)
	resp, err := srv.RequestCompaction(compactionAuthedCtx(), &pb.RequestCompactionRequest{
		DocumentId: "d1",
		ChunkIds:   []string{"c1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetJobId() != "j1" {
		t.Errorf("expected job_id=j1, got %s", resp.GetJobId())
	}
}

func TestCompactionServer_GetCompactionHistory_WithRecords(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			return &compactTopicRow{topicID: "t1"}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &compactRecordRows{records: []compactRecordData{
				{id: "r1", summaryChunk: "sc1", docID: "d1"},
			}}, nil
		},
	}
	srv := compactionServerWithMock(db, &mockAuthorizer{}, nil)
	resp, err := srv.GetCompactionHistory(compactionAuthedCtx(), &pb.GetCompactionHistoryRequest{DocumentId: "d1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetRecords()) != 1 {
		t.Errorf("expected 1 record, got %d", len(resp.GetRecords()))
	}
}

// --- Mock helpers specific to compaction tests ---

// compactTopicRow returns a topic ID for TopicIDForDocument queries.
type compactTopicRow struct{ topicID string }

func (r *compactTopicRow) Scan(dest ...any) error {
	*(dest[0].(*string)) = r.topicID
	return nil
}

// compactRecordRow returns a compaction record for GetBySummaryChunkID queries.
type compactRecordRow struct{ id, summaryChunk, docID string }

func (r *compactRecordRow) Scan(dest ...any) error {
	// CompactionStore.GetBySummaryChunkID: id, summary_chunk_id, source_chunk_ids, document_id, created_by, created_at
	*(dest[0].(*string)) = r.id
	*(dest[1].(*string)) = r.summaryChunk
	*(dest[2].(*[]string)) = []string{"c1", "c2"}
	*(dest[3].(*string)) = r.docID
	*(dest[4].(*string)) = "user:test"
	*(dest[5].(*time.Time)) = time.Now()
	return nil
}

// compactIntRow returns an int for NextSequence queries.
type compactIntRow struct{ val int }

func (r *compactIntRow) Scan(dest ...any) error {
	*(dest[0].(*int)) = r.val
	return nil
}

// compactChunkRow returns chunk data for ChunkStore.Create/Get queries.
type compactChunkRow struct{ id, docID string }

func (r *compactChunkRow) Scan(dest ...any) error {
	// id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
	*(dest[0].(*string)) = r.id
	*(dest[1].(*string)) = r.docID
	*(dest[2].(*int)) = 1
	*(dest[3].(*string)) = "summary content"
	// dest[4] *string (embedding_id) - leave nil
	*(dest[5].(*string)) = "active"
	// dest[6] *string (compacted_by) - leave nil
	*(dest[7].(*[]byte)) = []byte(`{}`)
	*(dest[8].(*time.Time)) = time.Now()
	return nil
}

// compactJobRow returns a job for JobStore.Create/CreateWithProgress queries.
type compactJobRow struct{ id string }

func (r *compactJobRow) Scan(dest ...any) error {
	// id, document_id, job_type, status, progress, error, started_at, completed_at, created_at
	*(dest[0].(*string)) = r.id
	*(dest[1].(*string)) = "d1"
	*(dest[2].(*string)) = "compaction"
	*(dest[3].(*string)) = "queued"
	*(dest[4].(*[]byte)) = []byte(`{}`)
	// dest[5] *string (error) - leave nil
	// dest[6] *time.Time (started_at) - leave nil
	// dest[7] *time.Time (completed_at) - leave nil
	*(dest[8].(*time.Time)) = time.Now()
	return nil
}

// compactRecordData holds data for mock compaction record rows.
type compactRecordData struct{ id, summaryChunk, docID string }

// compactRecordRows implements pgx.Rows returning compaction records.
type compactRecordRows struct {
	records []compactRecordData
	idx     int
}

func (r *compactRecordRows) Close()                                       {}
func (r *compactRecordRows) Err() error                                   { return nil }
func (r *compactRecordRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *compactRecordRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *compactRecordRows) RawValues() [][]byte                          { return nil }
func (r *compactRecordRows) Conn() *pgx.Conn                              { return nil }
func (r *compactRecordRows) Values() ([]any, error)                       { return nil, nil }

func (r *compactRecordRows) Next() bool {
	if r.idx < len(r.records) {
		r.idx++
		return true
	}
	return false
}

func (r *compactRecordRows) Scan(dest ...any) error {
	row := r.records[r.idx-1]
	*(dest[0].(*string)) = row.id
	*(dest[1].(*string)) = row.summaryChunk
	*(dest[2].(*[]string)) = []string{"c1", "c2"}
	*(dest[3].(*string)) = row.docID
	*(dest[4].(*string)) = "user:test"
	*(dest[5].(*time.Time)) = time.Now()
	return nil
}

// compactChunkData holds data for mock chunk rows in compaction tests.
type compactChunkData struct{ id, docID string }

// compactIDRows implements pgx.Rows returning string IDs.
type compactIDRows struct {
	ids []string
	idx int
}

func (r *compactIDRows) Close()                                       {}
func (r *compactIDRows) Err() error                                   { return nil }
func (r *compactIDRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *compactIDRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *compactIDRows) RawValues() [][]byte                          { return nil }
func (r *compactIDRows) Conn() *pgx.Conn                              { return nil }
func (r *compactIDRows) Values() ([]any, error)                       { return nil, nil }

func (r *compactIDRows) Next() bool {
	if r.idx < len(r.ids) {
		r.idx++
		return true
	}
	return false
}

func (r *compactIDRows) Scan(dest ...any) error {
	*(dest[0].(*string)) = r.ids[r.idx-1]
	return nil
}

// compactRestoredChunkRows implements pgx.Rows returning full chunk data.
type compactRestoredChunkRows struct {
	chunks []compactChunkData
	idx    int
}

func (r *compactRestoredChunkRows) Close()                                       {}
func (r *compactRestoredChunkRows) Err() error                                   { return nil }
func (r *compactRestoredChunkRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *compactRestoredChunkRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *compactRestoredChunkRows) RawValues() [][]byte                          { return nil }
func (r *compactRestoredChunkRows) Conn() *pgx.Conn                              { return nil }
func (r *compactRestoredChunkRows) Values() ([]any, error)                       { return nil, nil }

func (r *compactRestoredChunkRows) Next() bool {
	if r.idx < len(r.chunks) {
		r.idx++
		return true
	}
	return false
}

func (r *compactRestoredChunkRows) Scan(dest ...any) error {
	row := r.chunks[r.idx-1]
	// id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
	*(dest[0].(*string)) = row.id
	*(dest[1].(*string)) = row.docID
	*(dest[2].(*int)) = r.idx
	*(dest[3].(*string)) = "restored content"
	// dest[4] *string (embedding_id) - leave nil
	*(dest[5].(*string)) = "active"
	// dest[6] *string (compacted_by) - leave nil
	*(dest[7].(*[]byte)) = []byte(`{}`)
	*(dest[8].(*time.Time)) = time.Now()
	return nil
}

// compactEmptyRows implements pgx.Rows with no data.
type compactEmptyRows struct{}

func (r *compactEmptyRows) Close()                                       {}
func (r *compactEmptyRows) Err() error                                   { return nil }
func (r *compactEmptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *compactEmptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *compactEmptyRows) Next() bool                                   { return false }
func (r *compactEmptyRows) Scan(_ ...any) error                          { return nil }
func (r *compactEmptyRows) Values() ([]any, error)                       { return nil, nil }
func (r *compactEmptyRows) RawValues() [][]byte                          { return nil }
func (r *compactEmptyRows) Conn() *pgx.Conn                              { return nil }
