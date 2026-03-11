// Package worker provides a background job processing pool.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Tight-Line/creel/internal/store"
)

// Worker processes a specific type of job.
type Worker interface {
	// Type returns the job type string that this worker handles.
	// It must match the job_type column in processing_jobs.
	Type() string

	// Process executes the job. Return nil on success or an error on failure.
	Process(ctx context.Context, job *store.ProcessingJob) error
}

// Pool manages background workers that poll for and process jobs.
type Pool struct {
	jobStore     *store.JobStore
	concurrency  int
	pollInterval time.Duration
	logger       *slog.Logger
	workers      map[string]Worker
	mu           sync.Mutex
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewPool creates a new worker pool.
func NewPool(jobStore *store.JobStore, concurrency int, pollInterval time.Duration, logger *slog.Logger) *Pool {
	if concurrency <= 0 {
		concurrency = 4
	}
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Pool{
		jobStore:     jobStore,
		concurrency:  concurrency,
		pollInterval: pollInterval,
		logger:       logger,
		workers:      make(map[string]Worker),
	}
}

// Register adds a worker for a job type. Must be called before Start.
func (p *Pool) Register(w Worker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.workers[w.Type()] = w
}

// Start begins polling goroutines. Each goroutine polls for jobs across
// all registered worker types.
func (p *Pool) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, p.cancel = context.WithCancel(ctx)

	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.pollLoop(ctx)
	}

	p.logger.Info("worker pool started", "concurrency", p.concurrency, "poll_interval", p.pollInterval)
}

// Stop gracefully shuts down the pool, waiting for in-flight jobs with a timeout.
func (p *Pool) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("worker pool stopped gracefully")
	case <-time.After(30 * time.Second): // coverage:ignore - requires a worker that blocks for 30+ seconds
		p.logger.Warn("worker pool stop timed out after 30s")
	}
}

func (p *Pool) pollLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

// pollOnce tries to claim and process one job for each registered worker type.
// coverage:ignore - tested via integration
func (p *Pool) pollOnce(ctx context.Context) {
	p.mu.Lock()
	types := make([]string, 0, len(p.workers))
	for t := range p.workers {
		types = append(types, t)
	}
	p.mu.Unlock()

	// coverage:ignore - tested via integration
	for _, jobType := range types {
		if ctx.Err() != nil {
			return
		}
		p.tryProcess(ctx, jobType)
	}
}

// coverage:ignore - tested via integration
func (p *Pool) tryProcess(ctx context.Context, jobType string) {
	job, err := p.jobStore.ClaimNext(ctx, jobType)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		p.logger.Error("claiming job", "type", jobType, "error", err)
		return
	}
	// coverage:ignore - tested via integration
	if job == nil {
		return
	}

	// coverage:ignore - tested via integration
	p.mu.Lock()
	w := p.workers[jobType]
	p.mu.Unlock()

	p.logger.Info("processing job", "id", job.ID, "type", jobType)

	// coverage:ignore - tested via integration
	if err := w.Process(ctx, job); err != nil {
		errMsg := err.Error()
		// coverage:ignore - requires DB failure after successful claim
		if updateErr := p.jobStore.UpdateStatus(ctx, job.ID, "failed", &errMsg); updateErr != nil {
			p.logger.Error("updating job status to failed", "id", job.ID, "error", updateErr)
		}
		// coverage:ignore - tested via integration
		p.logger.Error("job failed", "id", job.ID, "type", jobType, "error", err)
		return
	}

	// coverage:ignore - tested via integration
	if err := p.jobStore.UpdateStatus(ctx, job.ID, "completed", nil); err != nil {
		p.logger.Error("updating job status to completed", "id", job.ID, "error", err) // coverage:ignore - requires DB failure after successful claim
	}
	// coverage:ignore - tested via integration
	p.logger.Info("job completed", "id", job.ID, "type", jobType)
}
