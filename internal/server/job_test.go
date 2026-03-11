package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/store"
)

// ---------------------------------------------------------------------------
// GetJob tests
// ---------------------------------------------------------------------------

func TestJobServer_GetJob_Unauthenticated(t *testing.T) {
	s := NewJobServer(nil, nil, nil)
	_, err := s.GetJob(context.Background(), &pb.GetJobRequest{Id: "x"})
	requireCode(t, err, codes.Unauthenticated)
}

func TestJobServer_GetJob_MissingID(t *testing.T) {
	s := NewJobServer(nil, nil, nil)
	_, err := s.GetJob(systemCtx(), &pb.GetJobRequest{})
	requireCode(t, err, codes.InvalidArgument)
}

func TestJobServer_GetJob_NotFound(t *testing.T) {
	db := failDBTX()
	s := NewJobServer(store.NewJobStore(db), store.NewDocumentStore(db), &mockAuthorizer{})
	_, err := s.GetJob(systemCtx(), &pb.GetJobRequest{Id: "job-1"})
	requireCode(t, err, codes.NotFound)
}

func TestJobServer_GetJob_DocumentNotFound(t *testing.T) {
	// JobStore.Get succeeds (returns job with document_id),
	// then DocumentStore.TopicIDForDocument fails.
	jobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockJobGetRow{}
		},
	}
	docDB := failDBTX()
	s := NewJobServer(store.NewJobStore(jobDB), store.NewDocumentStore(docDB), &mockAuthorizer{})
	_, err := s.GetJob(systemCtx(), &pb.GetJobRequest{Id: "job-1"})
	requireCode(t, err, codes.NotFound)
}

func TestJobServer_GetJob_PermissionDenied(t *testing.T) {
	jobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockJobGetRow{}
		},
	}
	docDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	s := NewJobServer(store.NewJobStore(jobDB), store.NewDocumentStore(docDB), &mockAuthorizer{checkErr: errors.New("denied")})
	_, err := s.GetJob(systemCtx(), &pb.GetJobRequest{Id: "job-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestJobServer_GetJob_DoclessJob_Success(t *testing.T) {
	// Documentless job where progress["principal"] matches the caller.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockDoclessJobRow{principal: "system:test"}
		},
	}
	s := NewJobServer(store.NewJobStore(db), nil, nil)
	resp, err := s.GetJob(systemCtx(), &pb.GetJobRequest{Id: "job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

func TestJobServer_GetJob_DoclessJob_WrongPrincipal(t *testing.T) {
	// Documentless job where progress["principal"] does NOT match the caller.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockDoclessJobRow{principal: "system:other"}
		},
	}
	s := NewJobServer(store.NewJobStore(db), nil, nil)
	_, err := s.GetJob(systemCtx(), &pb.GetJobRequest{Id: "job-1"})
	requireCode(t, err, codes.PermissionDenied)
}

// ---------------------------------------------------------------------------
// ListJobs tests
// ---------------------------------------------------------------------------

func TestJobServer_ListJobs_Unauthenticated(t *testing.T) {
	s := NewJobServer(nil, nil, nil)
	_, err := s.ListJobs(context.Background(), &pb.ListJobsRequest{})
	requireCode(t, err, codes.Unauthenticated)
}

func TestJobServer_ListJobs_ByTopicID_PermissionDenied(t *testing.T) {
	s := NewJobServer(nil, nil, &mockAuthorizer{checkErr: errors.New("denied")})
	_, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{TopicId: "topic-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestJobServer_ListJobs_ByTopicID_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewJobServer(store.NewJobStore(db), nil, &mockAuthorizer{})
	_, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{TopicId: "topic-1"})
	requireCode(t, err, codes.Internal)
}

func TestJobServer_ListJobs_ByDocumentID_DocNotFound(t *testing.T) {
	docDB := failDBTX()
	s := NewJobServer(nil, store.NewDocumentStore(docDB), &mockAuthorizer{})
	_, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{DocumentId: "doc-1"})
	requireCode(t, err, codes.NotFound)
}

func TestJobServer_ListJobs_ByDocumentID_PermissionDenied(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	s := NewJobServer(store.NewJobStore(db), store.NewDocumentStore(db), &mockAuthorizer{checkErr: errors.New("denied")})
	_, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{DocumentId: "doc-1"})
	requireCode(t, err, codes.PermissionDenied)
}

func TestJobServer_ListJobs_ByDocumentID_StoreError(t *testing.T) {
	// DocStore.TopicIDForDocument succeeds, authorizer allows, but jobStore.List fails.
	docDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	jobDB := failDBTX()
	s := NewJobServer(store.NewJobStore(jobDB), store.NewDocumentStore(docDB), &mockAuthorizer{})
	_, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{DocumentId: "doc-1"})
	requireCode(t, err, codes.Internal)
}

func TestJobServer_ListJobs_AllTopics_AccessError(t *testing.T) {
	s := NewJobServer(nil, nil, &mockAuthorizer{accessibleErr: errors.New("access error")})
	_, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{})
	requireCode(t, err, codes.Internal)
}

func TestJobServer_ListJobs_AllTopics_Empty(t *testing.T) {
	s := NewJobServer(nil, nil, &mockAuthorizer{accessibleTopics: []string{}})
	resp, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(resp.Jobs))
	}
}

func TestJobServer_ListJobs_AllTopics_StoreError(t *testing.T) {
	db := failDBTX()
	s := NewJobServer(store.NewJobStore(db), nil, &mockAuthorizer{accessibleTopics: []string{"topic-1"}})
	_, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{})
	requireCode(t, err, codes.Internal)
}

// ---------------------------------------------------------------------------
// Success path tests (cover storeJobToProto, pagination, etc.)
// ---------------------------------------------------------------------------

func TestJobServer_GetJob_Success(t *testing.T) {
	// Mock where both JobStore.Get and DocStore.TopicIDForDocument succeed.
	calls := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			calls++
			if calls == 1 {
				return &mockJobGetRow{} // JobStore.Get
			}
			return &mockRow{err: nil} // DocStore.TopicIDForDocument
		},
	}
	s := NewJobServer(store.NewJobStore(db), store.NewDocumentStore(db), &mockAuthorizer{})
	resp, err := s.GetJob(systemCtx(), &pb.GetJobRequest{Id: "job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

func TestJobServer_ListJobs_ByTopicID_Success(t *testing.T) {
	db := &mockDBTX{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &emptyRows{}, nil
		},
	}
	s := NewJobServer(store.NewJobStore(db), nil, &mockAuthorizer{})
	resp, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{TopicId: "topic-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(resp.Jobs))
	}
}

func TestJobServer_ListJobs_ByDocumentID_Success(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &emptyRows{}, nil
		},
	}
	s := NewJobServer(store.NewJobStore(db), store.NewDocumentStore(db), &mockAuthorizer{})
	resp, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{DocumentId: "doc-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(resp.Jobs))
	}
}

func TestJobServer_ListJobs_AllTopics_Success(t *testing.T) {
	db := &mockDBTX{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &emptyRows{}, nil
		},
	}
	s := NewJobServer(store.NewJobStore(db), nil, &mockAuthorizer{accessibleTopics: []string{"topic-1", "topic-2"}})
	resp, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

func TestJobServer_ListJobs_ByTopicID_WithResults(t *testing.T) {
	// jobRows returns mock rows with n jobs.
	jobRows := func(n int) *mockJobRows {
		return &mockJobRows{count: n}
	}
	db := &mockDBTX{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return jobRows(3), nil // return 3 rows
		},
	}
	s := NewJobServer(store.NewJobStore(db), nil, &mockAuthorizer{})
	// Use PageSize=2 so we get pagination.
	resp, err := s.ListJobs(systemCtx(), &pb.ListJobsRequest{TopicId: "topic-1", PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(resp.Jobs))
	}
	if resp.NextPageToken == "" {
		t.Error("expected non-empty next_page_token")
	}
}

// mockJobGetRow implements pgx.Row that returns a single job with a document_id.
// Used for GetJob tests where the job must have a non-nil document_id.
type mockJobGetRow struct{}

func (r *mockJobGetRow) Scan(dest ...any) error {
	// id, document_id(*string), job_type, status, progress, error, started_at, completed_at, created_at
	*(dest[0].(*string)) = "job-1"
	docID := "doc-1"
	*(dest[1].(**string)) = &docID
	*(dest[2].(*string)) = "extraction"
	*(dest[3].(*string)) = "queued"
	// dest[4] *[]byte (progress) - leave nil
	// dest[5] **string (error) - leave nil
	// dest[6] **time.Time (started_at) - leave nil
	// dest[7] **time.Time (completed_at) - leave nil
	*(dest[8].(*time.Time)) = time.Now()
	return nil
}

// mockDoclessJobRow implements pgx.Row for a documentless job with progress data.
type mockDoclessJobRow struct{ principal string }

func (r *mockDoclessJobRow) Scan(dest ...any) error {
	*(dest[0].(*string)) = "job-1"
	*(dest[1].(**string)) = nil // document_id is NULL
	*(dest[2].(*string)) = "memory_maintenance"
	*(dest[3].(*string)) = "queued"
	*(dest[4].(*[]byte)) = []byte(fmt.Sprintf(`{"principal":%q}`, r.principal))
	// dest[5] **string (error) - leave nil
	// dest[6] **time.Time (started_at) - leave nil
	// dest[7] **time.Time (completed_at) - leave nil
	*(dest[8].(*time.Time)) = time.Now()
	return nil
}

// mockJobRows implements pgx.Rows that returns n mock job rows.
type mockJobRows struct {
	count int
	idx   int
}

func (r *mockJobRows) Close()                                       {}
func (r *mockJobRows) Err() error                                   { return nil }
func (r *mockJobRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockJobRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockJobRows) RawValues() [][]byte                          { return nil }
func (r *mockJobRows) Conn() *pgx.Conn                              { return nil }
func (r *mockJobRows) Values() ([]any, error)                       { return nil, nil }

func (r *mockJobRows) Next() bool {
	if r.idx < r.count {
		r.idx++
		return true
	}
	return false
}

func (r *mockJobRows) Scan(dest ...any) error {
	// Scan into: id, document_id, job_type, status, progress, error, started_at, completed_at, created_at
	if id, ok := dest[0].(*string); ok {
		*id = fmt.Sprintf("job-%d", r.idx)
	}
	if docID, ok := dest[1].(**string); ok {
		v := "doc-1"
		*docID = &v
	}
	if jt, ok := dest[2].(*string); ok {
		*jt = "extraction"
	}
	if st, ok := dest[3].(*string); ok {
		*st = "queued"
	}
	// dest[4] is *[]byte (progress) - leave nil
	// dest[5] is **string (error) - leave nil
	// dest[6] is **time.Time (started_at) - leave nil
	// dest[7] is **time.Time (completed_at) - leave nil
	if ca, ok := dest[8].(*time.Time); ok {
		*ca = time.Now()
	}
	return nil
}

func TestStoreJobToProto_AllFields(t *testing.T) {
	now := time.Now()
	errMsg := "something failed"
	job := &store.ProcessingJob{
		ID:          "job-123",
		DocumentID:  "doc-456",
		JobType:     "extraction",
		Status:      "failed",
		Progress:    map[string]any{"step": float64(3)},
		Error:       &errMsg,
		StartedAt:   &now,
		CompletedAt: &now,
		CreatedAt:   now,
	}

	proto := storeJobToProto(job)
	if proto.Id != "job-123" {
		t.Errorf("Id = %q, want job-123", proto.Id)
	}
	if proto.Error != errMsg {
		t.Errorf("Error = %q, want %q", proto.Error, errMsg)
	}
	if proto.StartedAt == nil {
		t.Error("StartedAt should be set")
	}
	if proto.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
	if proto.Progress == nil {
		t.Error("Progress should be set")
	}
}
