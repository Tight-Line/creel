package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/config"
)

// setupJobTestDB creates a pool, ensures schema, runs migrations, and
// seeds a topic + document for job tests. Skips if CREEL_POSTGRES_HOST is not set.
func setupJobTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()
	if err := EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if err := RunMigrations(pgCfg.URL(), "../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgCfg.URL())
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Clean processing_jobs table.
	if _, err := pool.Exec(ctx, "DELETE FROM processing_jobs"); err != nil {
		t.Fatalf("cleaning processing_jobs: %v", err)
	}

	// Create a topic and document for FK references.
	topicStore := NewTopicStore(pool)
	topic, err := topicStore.Create(ctx, "job-test-topic", "Job Test Topic", "", "", nil, nil, nil, nil)
	if err != nil {
		// Topic may already exist; find it by listing all topics.
		topics, listErr := topicStore.ListForPrincipals(ctx, nil)
		if listErr != nil {
			t.Fatalf("listing topics: %v", listErr)
		}
		for i := range topics {
			if topics[i].Slug == "job-test-topic" {
				topic = &topics[i]
				break
			}
		}
		if topic == nil {
			t.Fatalf("creating topic: %v", err)
		}
	}

	docStore := NewDocumentStore(pool)
	slug := fmt.Sprintf("job-test-doc-%d", time.Now().UnixNano())
	doc, err := docStore.Create(ctx, topic.ID, slug, "Job Test Doc", "reference", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	return pool, doc.ID
}

func TestJobStore_Integration_CreateAndGet(t *testing.T) {
	pool, docID := setupJobTestDB(t)
	s := NewJobStore(pool)
	ctx := context.Background()

	job, err := s.Create(ctx, docID, "extraction")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if job.DocumentID != docID {
		t.Errorf("DocumentID = %q, want %q", job.DocumentID, docID)
	}
	if job.JobType != "extraction" {
		t.Errorf("JobType = %q, want extraction", job.JobType)
	}
	if job.Status != "queued" {
		t.Errorf("Status = %q, want queued", job.Status)
	}
	if job.StartedAt != nil {
		t.Errorf("StartedAt should be nil for queued job")
	}

	got, err := s.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("Get ID = %q, want %q", got.ID, job.ID)
	}
}

func TestJobStore_Integration_GetNotFound(t *testing.T) {
	pool, _ := setupJobTestDB(t)
	s := NewJobStore(pool)
	ctx := context.Background()

	_, err := s.Get(ctx, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent job")
	}
}

func TestJobStore_Integration_List(t *testing.T) {
	pool, docID := setupJobTestDB(t)
	s := NewJobStore(pool)
	ctx := context.Background()

	// Create a few jobs.
	for _, jt := range []string{"extraction", "chunking", "embedding"} {
		if _, err := s.Create(ctx, docID, jt); err != nil {
			t.Fatalf("Create(%s): %v", jt, err)
		}
	}

	// List all.
	jobs, err := s.List(ctx, ListJobsOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) < 3 {
		t.Errorf("List len = %d, want >= 3", len(jobs))
	}

	// List by document_id.
	jobs, err = s.List(ctx, ListJobsOptions{DocumentID: docID})
	if err != nil {
		t.Fatalf("List by document: %v", err)
	}
	if len(jobs) < 3 {
		t.Errorf("List by document len = %d, want >= 3", len(jobs))
	}

	// List by status.
	jobs, err = s.List(ctx, ListJobsOptions{Status: "queued"})
	if err != nil {
		t.Fatalf("List by status: %v", err)
	}
	if len(jobs) < 3 {
		t.Errorf("List by status len = %d, want >= 3", len(jobs))
	}

	// List with pagination.
	page1, err := s.List(ctx, ListJobsOptions{PageSize: 2})
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(page1) < 2 {
		t.Fatalf("List page 1 len = %d, want >= 2", len(page1))
	}
	// Use the last item as page token.
	page2, err := s.List(ctx, ListJobsOptions{PageSize: 2, PageToken: page1[1].ID})
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(page2) == 0 {
		t.Error("expected page 2 to have results")
	}
}

func TestJobStore_Integration_ListByTopicID(t *testing.T) {
	pool, docID := setupJobTestDB(t)
	s := NewJobStore(pool)
	ctx := context.Background()

	// Find the topic ID for our test document.
	docStore := NewDocumentStore(pool)
	topicID, err := docStore.TopicIDForDocument(ctx, docID)
	if err != nil {
		t.Fatalf("TopicIDForDocument: %v", err)
	}

	// Create a job.
	if _, err := s.Create(ctx, docID, "embedding"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	jobs, err := s.ListByTopicID(ctx, topicID, ListJobsOptions{})
	if err != nil {
		t.Fatalf("ListByTopicID: %v", err)
	}
	if len(jobs) == 0 {
		t.Error("expected at least one job")
	}

	// Filter by status.
	jobs, err = s.ListByTopicID(ctx, topicID, ListJobsOptions{Status: "queued"})
	if err != nil {
		t.Fatalf("ListByTopicID with status: %v", err)
	}
	if len(jobs) == 0 {
		t.Error("expected at least one queued job")
	}

	// Pagination.
	page1, err := s.ListByTopicID(ctx, topicID, ListJobsOptions{PageSize: 1})
	if err != nil {
		t.Fatalf("ListByTopicID page 1: %v", err)
	}
	if len(page1) == 0 {
		t.Fatal("expected at least one job in page 1")
	}
	page2, err := s.ListByTopicID(ctx, topicID, ListJobsOptions{PageSize: 1, PageToken: page1[0].ID})
	if err != nil {
		t.Fatalf("ListByTopicID page 2: %v", err)
	}
	// page2 may be empty if only one job; that's fine.
	_ = page2
}

func TestJobStore_Integration_UpdateStatus(t *testing.T) {
	pool, docID := setupJobTestDB(t)
	s := NewJobStore(pool)
	ctx := context.Background()

	job, err := s.Create(ctx, docID, "chunking")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Transition to running.
	if err := s.UpdateStatus(ctx, job.ID, "running", nil); err != nil {
		t.Fatalf("UpdateStatus running: %v", err)
	}
	got, err := s.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt should be set for running job")
	}

	// Transition to completed.
	if err := s.UpdateStatus(ctx, job.ID, "completed", nil); err != nil {
		t.Fatalf("UpdateStatus completed: %v", err)
	}
	got, err = s.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set for completed job")
	}

	// Test failed with error message.
	job2, err := s.Create(ctx, docID, "extraction")
	if err != nil {
		t.Fatalf("Create job2: %v", err)
	}
	errMsg := "something went wrong"
	if err := s.UpdateStatus(ctx, job2.ID, "failed", &errMsg); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	got2, err := s.Get(ctx, job2.ID)
	if err != nil {
		t.Fatalf("Get job2: %v", err)
	}
	if got2.Status != "failed" {
		t.Errorf("Status = %q, want failed", got2.Status)
	}
	if got2.Error == nil || *got2.Error != errMsg {
		t.Errorf("Error = %v, want %q", got2.Error, errMsg)
	}
	if got2.CompletedAt == nil {
		t.Error("CompletedAt should be set for failed job")
	}

	// Update non-existent job.
	err = s.UpdateStatus(ctx, "00000000-0000-0000-0000-000000000000", "running", nil)
	if err == nil {
		t.Error("expected error for non-existent job")
	}
}

func TestJobStore_Integration_ClaimNext(t *testing.T) {
	pool, docID := setupJobTestDB(t)
	s := NewJobStore(pool)
	ctx := context.Background()

	// Create multiple queued jobs.
	for i := 0; i < 3; i++ {
		if _, err := s.Create(ctx, docID, "extraction"); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	// Claim first.
	claimed, err := s.ClaimNext(ctx, "extraction")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected a claimed job")
	}
	if claimed.Status != "running" {
		t.Errorf("claimed Status = %q, want running", claimed.Status)
	}
	if claimed.StartedAt == nil {
		t.Error("claimed StartedAt should be set")
	}

	// Claim next should get a different job.
	claimed2, err := s.ClaimNext(ctx, "extraction")
	if err != nil {
		t.Fatalf("ClaimNext 2: %v", err)
	}
	if claimed2 == nil {
		t.Fatal("expected a second claimed job")
	}
	if claimed2.ID == claimed.ID {
		t.Error("second claim should be a different job")
	}

	// ClaimNext for non-existent type should return nil.
	none, err := s.ClaimNext(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ClaimNext nonexistent: %v", err)
	}
	if none != nil {
		t.Error("expected nil for non-existent job type")
	}
}

func TestJobStore_Integration_ClaimNextConcurrent(t *testing.T) {
	pool, docID := setupJobTestDB(t)
	s := NewJobStore(pool)
	ctx := context.Background()

	// Create a single job.
	job, err := s.Create(ctx, docID, "chunking")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Race 10 goroutines to claim it.
	const n = 10
	var mu sync.Mutex
	var claimed []string

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			j, err := s.ClaimNext(ctx, "chunking")
			if err != nil {
				return
			}
			if j != nil {
				mu.Lock()
				claimed = append(claimed, j.ID)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(claimed) != 1 {
		t.Errorf("expected exactly 1 claim, got %d", len(claimed))
	}
	if len(claimed) > 0 && claimed[0] != job.ID {
		t.Errorf("claimed wrong job: got %s, want %s", claimed[0], job.ID)
	}
}

func TestJobStore_Integration_ProgressField(t *testing.T) {
	pool, docID := setupJobTestDB(t)
	s := NewJobStore(pool)
	ctx := context.Background()

	job, err := s.Create(ctx, docID, "extraction")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Manually set progress via direct SQL to exercise the progress unmarshaling path.
	_, err = pool.Exec(ctx, `UPDATE processing_jobs SET progress = '{"step": 1, "total": 10}'::jsonb WHERE id = $1`, job.ID)
	if err != nil {
		t.Fatalf("setting progress: %v", err)
	}

	// Get should unmarshal progress.
	got, err := s.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Progress == nil {
		t.Fatal("expected non-nil Progress")
	}
	if got.Progress["step"] != float64(1) {
		t.Errorf("Progress[step] = %v, want 1", got.Progress["step"])
	}

	// List should also unmarshal progress.
	jobs, err := s.List(ctx, ListJobsOptions{DocumentID: docID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, j := range jobs {
		if j.ID == job.ID && j.Progress != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find job with progress in list results")
	}
}

func TestJobStore_Integration_ClaimNext_ScanError(t *testing.T) {
	// This tests the non-ErrNoRows error path in ClaimNext.
	// We test via the mock path since it's hard to trigger a scan error with a real DB.
	db := &mockDBTX{
		beginFn: func(_ context.Context) (pgx.Tx, error) {
			return &mockTx{
				inner: &mockDBTX{
					queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
						return &mockRow{err: errors.New("scan error")}
					},
				},
			}, nil
		},
	}
	s := NewJobStore(db)
	_, err := s.ClaimNext(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Mock-based unit tests for error paths
// ---------------------------------------------------------------------------

func TestJobStore_Create_Error(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	s := NewJobStore(db)
	_, err := s.Create(context.Background(), "doc-id", "extraction")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJobStore_CreateWithProgress_MarshalError(t *testing.T) {
	s := NewJobStore(nil) // DB not needed; error happens before query.
	_, err := s.CreateWithProgress(context.Background(), "doc-id", "memory_maintenance", map[string]any{"bad": math.Inf(1)})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "marshaling progress") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestJobStore_CreateWithProgress_Error(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	s := NewJobStore(db)
	_, err := s.CreateWithProgress(context.Background(), "doc-id", "memory_maintenance", map[string]any{"fact": "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "inserting processing job with progress") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestJobStore_Get_Error(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	s := NewJobStore(db)
	_, err := s.Get(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJobStore_List_QueryError(t *testing.T) {
	db := &mockDBTX{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errMock
		},
	}
	s := NewJobStore(db)
	_, err := s.List(context.Background(), ListJobsOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJobStore_ListByTopicID_QueryError(t *testing.T) {
	db := &mockDBTX{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errMock
		},
	}
	s := NewJobStore(db)
	_, err := s.ListByTopicID(context.Background(), "topic-id", ListJobsOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJobStore_UpdateStatus_ExecError(t *testing.T) {
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errMock
		},
	}
	s := NewJobStore(db)
	err := s.UpdateStatus(context.Background(), "id", "running", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJobStore_List_ScanError(t *testing.T) {
	db := &mockDBTX{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{nextOnce: true, scanErr: errMock}, nil
		},
	}
	s := NewJobStore(db)
	_, err := s.List(context.Background(), ListJobsOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJobStore_List_IterError(t *testing.T) {
	db := &mockDBTX{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{iterErr: errors.New("iter error")}, nil
		},
	}
	s := NewJobStore(db)
	_, err := s.List(context.Background(), ListJobsOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}
