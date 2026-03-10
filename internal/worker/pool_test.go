package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/store"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testWorker is a mock Worker for testing.
type testWorker struct {
	jobType   string
	processFn func(ctx context.Context, job *store.ProcessingJob) error
}

func (w *testWorker) Type() string { return w.jobType }
func (w *testWorker) Process(ctx context.Context, job *store.ProcessingJob) error {
	if w.processFn != nil {
		return w.processFn(ctx, job)
	}
	return nil
}

// setupWorkerTestDB creates a pool with a test document for creating jobs.
func setupWorkerTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()
	if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if err := store.RunMigrations(pgCfg.URL(), "../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgCfg.URL())
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Clean processing_jobs.
	if _, err := pool.Exec(ctx, "DELETE FROM processing_jobs"); err != nil {
		t.Fatalf("cleaning processing_jobs: %v", err)
	}

	// Create a topic and document.
	topicStore := store.NewTopicStore(pool)
	topic, err := topicStore.Create(ctx, "worker-test-topic", "Worker Test Topic", "", "", nil, nil, nil, false, nil)
	if err != nil {
		// Topic may already exist; find it by listing all topics.
		topics, listErr := topicStore.ListForPrincipals(ctx, nil)
		if listErr != nil {
			t.Fatalf("listing topics: %v", listErr)
		}
		for i := range topics {
			if topics[i].Slug == "worker-test-topic" {
				topic = &topics[i]
				break
			}
		}
		if topic == nil {
			t.Fatalf("creating topic: %v", err)
		}
	}

	docStore := store.NewDocumentStore(pool)
	slug := fmt.Sprintf("worker-test-doc-%d", time.Now().UnixNano())
	doc, err := docStore.Create(ctx, topic.ID, slug, "Worker Test Doc", "reference", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	return pool, doc.ID
}

// ---------------------------------------------------------------------------
// Unit tests (no database needed)
// ---------------------------------------------------------------------------

func TestNewPool_Defaults(t *testing.T) {
	p := NewPool(nil, 0, 0, nil)
	if p.concurrency != 4 {
		t.Errorf("concurrency = %d, want 4", p.concurrency)
	}
	if p.pollInterval != 5*time.Second {
		t.Errorf("pollInterval = %v, want 5s", p.pollInterval)
	}
}

func TestPool_RegisterAndStartStop(t *testing.T) {
	p := NewPool(nil, 1, 100*time.Millisecond, slog.Default())
	w := &testWorker{jobType: "test"}
	p.Register(w)

	ctx := context.Background()
	p.Start(ctx)

	// Give it a moment to start.
	time.Sleep(50 * time.Millisecond)

	p.Stop()
}

func TestPool_StartStopWithoutWorkers(t *testing.T) {
	p := NewPool(nil, 2, 100*time.Millisecond, slog.Default())
	ctx := context.Background()
	p.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	p.Stop()
}

func TestPool_StopWithoutStart(t *testing.T) {
	// Stop without Start should not panic.
	p := NewPool(nil, 1, time.Second, slog.Default())
	p.Stop()
}

// ---------------------------------------------------------------------------
// Integration test: processes jobs end-to-end
// ---------------------------------------------------------------------------

func TestPool_Integration_ProcessesJob(t *testing.T) {
	pool, docID := setupWorkerTestDB(t)
	jobStore := store.NewJobStore(pool)
	ctx := context.Background()

	// Create a job.
	job, err := jobStore.Create(ctx, docID, "test-worker")
	if err != nil {
		t.Fatalf("Create job: %v", err)
	}

	var processed atomic.Bool
	w := &testWorker{
		jobType: "test-worker",
		processFn: func(_ context.Context, j *store.ProcessingJob) error {
			if j.ID == job.ID {
				processed.Store(true)
			}
			return nil
		},
	}

	p := NewPool(jobStore, 1, 100*time.Millisecond, slog.Default())
	p.Register(w)
	p.Start(ctx)

	// Wait for job to be processed.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if processed.Load() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	p.Stop()

	if !processed.Load() {
		t.Error("job was not processed")
	}

	// Verify job is completed.
	got, err := jobStore.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("Status = %q, want completed", got.Status)
	}
}

func TestPool_Integration_HandlesWorkerError(t *testing.T) {
	pool, docID := setupWorkerTestDB(t)
	jobStore := store.NewJobStore(pool)
	ctx := context.Background()

	job, err := jobStore.Create(ctx, docID, "failing-worker")
	if err != nil {
		t.Fatalf("Create job: %v", err)
	}

	var processed atomic.Bool
	w := &testWorker{
		jobType: "failing-worker",
		processFn: func(_ context.Context, _ *store.ProcessingJob) error {
			processed.Store(true)
			return errors.New("worker failed")
		},
	}

	p := NewPool(jobStore, 1, 100*time.Millisecond, slog.Default())
	p.Register(w)
	p.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if processed.Load() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	p.Stop()

	if !processed.Load() {
		t.Error("job was not processed")
	}

	got, err := jobStore.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "failed" {
		t.Errorf("Status = %q, want failed", got.Status)
	}
	if got.Error == nil || *got.Error != "worker failed" {
		t.Errorf("Error = %v, want 'worker failed'", got.Error)
	}
}

func TestPool_Integration_RespectsContextCancellation(t *testing.T) {
	pool, docID := setupWorkerTestDB(t)
	jobStore := store.NewJobStore(pool)

	// Create a job but cancel before it can be processed.
	ctx, cancel := context.WithCancel(context.Background())

	_, err := jobStore.Create(ctx, docID, "slow-worker")
	if err != nil {
		t.Fatalf("Create job: %v", err)
	}

	var started atomic.Bool
	w := &testWorker{
		jobType: "slow-worker",
		processFn: func(ctx context.Context, _ *store.ProcessingJob) error {
			started.Store(true)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	p := NewPool(jobStore, 1, 100*time.Millisecond, slog.Default())
	p.Register(w)
	p.Start(ctx)

	// Wait for the job to start processing.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if started.Load() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Cancel context; pool should shut down.
	cancel()
	p.Stop()
}

// ---------------------------------------------------------------------------
// Unit test: tryProcess with mock that returns claim error
// ---------------------------------------------------------------------------

func TestPool_TryProcess_ClaimError(t *testing.T) {
	// Use a mock DBTX that returns a begin error to trigger ClaimNext failure.
	db := &mockDBTXForWorker{
		beginErr: errors.New("begin failed"),
	}
	jobStore := store.NewJobStore(db)

	p := NewPool(jobStore, 1, time.Second, slog.Default())
	w := &testWorker{jobType: "test"}
	p.Register(w)

	// This should log the error but not panic.
	p.tryProcess(context.Background(), "test")
}

func TestPool_TryProcess_ClaimError_CancelledContext(t *testing.T) {
	// When ClaimNext fails and ctx is cancelled, should return silently.
	db := &mockDBTXForWorker{
		beginErr: errors.New("begin failed"),
	}
	jobStore := store.NewJobStore(db)

	p := NewPool(jobStore, 1, time.Second, slog.Default())
	w := &testWorker{jobType: "test"}
	p.Register(w)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p.tryProcess(ctx, "test")
}

func TestPool_PollOnce_CancelledContext(t *testing.T) {
	p := NewPool(nil, 1, time.Second, slog.Default())
	w := &testWorker{jobType: "test"}
	p.Register(w)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should return early without panicking.
	p.pollOnce(ctx)
}

// mockDBTXForWorker is a minimal mock for worker tests that need a JobStore
// but no real database.
type mockDBTXForWorker struct {
	beginErr error
}

func (m *mockDBTXForWorker) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockDBTXForWorker) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDBTXForWorker) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRowForWorker{err: pgx.ErrNoRows}
}

func (m *mockDBTXForWorker) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, m.beginErr
}

type mockRowForWorker struct{ err error }

func (r *mockRowForWorker) Scan(_ ...any) error { return r.err }

// ---------------------------------------------------------------------------
// Integration test: concurrent pool goroutines
// ---------------------------------------------------------------------------

func TestPool_Integration_ConcurrentProcessing(t *testing.T) {
	pool, docID := setupWorkerTestDB(t)
	jobStore := store.NewJobStore(pool)
	ctx := context.Background()

	// Create multiple jobs.
	const numJobs = 5
	for i := 0; i < numJobs; i++ {
		if _, err := jobStore.Create(ctx, docID, "concurrent-worker"); err != nil {
			t.Fatalf("Create job %d: %v", i, err)
		}
	}

	var count atomic.Int32
	var mu sync.Mutex
	processedIDs := make(map[string]bool)

	w := &testWorker{
		jobType: "concurrent-worker",
		processFn: func(_ context.Context, j *store.ProcessingJob) error {
			count.Add(1)
			mu.Lock()
			processedIDs[j.ID] = true
			mu.Unlock()
			time.Sleep(10 * time.Millisecond) // simulate work
			return nil
		},
	}

	p := NewPool(jobStore, 3, 100*time.Millisecond, slog.Default())
	p.Register(w)
	p.Start(ctx)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if count.Load() >= numJobs {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	p.Stop()

	if got := count.Load(); got < int32(numJobs) {
		t.Errorf("processed %d jobs, want %d", got, numJobs)
	}

	mu.Lock()
	uniqueCount := len(processedIDs)
	mu.Unlock()
	if uniqueCount != numJobs {
		t.Errorf("unique processed = %d, want %d (duplicate processing detected)", uniqueCount, numJobs)
	}
}
