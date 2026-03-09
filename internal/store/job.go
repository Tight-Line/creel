package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ProcessingJob represents a background processing job.
type ProcessingJob struct {
	ID          string
	DocumentID  string
	JobType     string
	Status      string
	Progress    map[string]any
	Error       *string
	StartedAt   *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
}

// ListJobsOptions controls filtering and pagination for job listing.
type ListJobsOptions struct {
	DocumentID string
	Status     string
	PageSize   int
	PageToken  string // job ID for keyset pagination
}

// JobStore handles processing job persistence.
type JobStore struct {
	pool DBTX
}

// NewJobStore creates a new job store.
func NewJobStore(pool DBTX) *JobStore {
	return &JobStore{pool: pool}
}

// jobColumns is the column list shared across all scans.
const jobColumns = `id, document_id, job_type, status, progress, error, started_at, completed_at, created_at`

// scanJob scans a single job row into a ProcessingJob.
func scanJob(row pgx.Row) (*ProcessingJob, error) {
	var j ProcessingJob
	var progressBytes []byte
	err := row.Scan(&j.ID, &j.DocumentID, &j.JobType, &j.Status, &progressBytes, &j.Error, &j.StartedAt, &j.CompletedAt, &j.CreatedAt)
	if err != nil {
		return nil, err
	}
	if progressBytes != nil {
		_ = json.Unmarshal(progressBytes, &j.Progress)
	}
	return &j, nil
}

// scanJobRows scans multiple job rows into a slice.
func scanJobRows(rows pgx.Rows) ([]*ProcessingJob, error) {
	defer rows.Close()
	var jobs []*ProcessingJob
	for rows.Next() {
		var j ProcessingJob
		var progressBytes []byte
		if err := rows.Scan(&j.ID, &j.DocumentID, &j.JobType, &j.Status, &progressBytes, &j.Error, &j.StartedAt, &j.CompletedAt, &j.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning job row: %w", err)
		}
		if progressBytes != nil {
			_ = json.Unmarshal(progressBytes, &j.Progress)
		}
		jobs = append(jobs, &j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating job rows: %w", err)
	}
	return jobs, nil
}

// Create inserts a new queued processing job.
func (s *JobStore) Create(ctx context.Context, documentID, jobType string) (*ProcessingJob, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO processing_jobs (document_id, job_type) VALUES ($1, $2) RETURNING `+jobColumns,
		documentID, jobType,
	)
	j, err := scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("inserting processing job: %w", err)
	}
	return j, nil
}

// CreateWithProgress inserts a new queued processing job with initial progress data.
func (s *JobStore) CreateWithProgress(ctx context.Context, documentID, jobType string, progress map[string]any) (*ProcessingJob, error) {
	progressJSON, err := json.Marshal(progress)
	if err != nil {
		return nil, fmt.Errorf("marshaling progress: %w", err)
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO processing_jobs (document_id, job_type, progress) VALUES ($1, $2, $3) RETURNING `+jobColumns,
		documentID, jobType, progressJSON,
	)
	j, err := scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("inserting processing job with progress: %w", err)
	}
	return j, nil
}

// Get retrieves a processing job by ID.
func (s *JobStore) Get(ctx context.Context, id string) (*ProcessingJob, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+jobColumns+` FROM processing_jobs WHERE id = $1`, id,
	)
	j, err := scanJob(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying processing job: %w", err)
	}
	return j, nil
}

// List returns processing jobs matching the given filters.
func (s *JobStore) List(ctx context.Context, opts ListJobsOptions) ([]*ProcessingJob, error) {
	query := `SELECT ` + jobColumns + ` FROM processing_jobs WHERE 1=1`
	args := []any{}
	argIdx := 1

	if opts.DocumentID != "" {
		query += fmt.Sprintf(` AND document_id = $%d`, argIdx)
		args = append(args, opts.DocumentID)
		argIdx++
	}
	if opts.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, opts.Status)
		argIdx++
	}
	if opts.PageToken != "" {
		query += fmt.Sprintf(` AND created_at <= (SELECT created_at FROM processing_jobs WHERE id = $%d)`, argIdx)
		args = append(args, opts.PageToken)
		argIdx++
		query += fmt.Sprintf(` AND id != $%d`, argIdx)
		args = append(args, opts.PageToken)
		argIdx++
	}

	query += ` ORDER BY created_at DESC`

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, pageSize+1) // fetch one extra for next_page_token
	// argIdx++ (not needed; last parameter)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing processing jobs: %w", err)
	}
	return scanJobRows(rows)
}

// ListByTopicID returns processing jobs for documents in the given topic.
func (s *JobStore) ListByTopicID(ctx context.Context, topicID string, opts ListJobsOptions) ([]*ProcessingJob, error) {
	query := `SELECT j.` + `id, j.document_id, j.job_type, j.status, j.progress, j.error, j.started_at, j.completed_at, j.created_at` +
		` FROM processing_jobs j JOIN documents d ON j.document_id = d.id WHERE d.topic_id = $1`
	args := []any{topicID}
	argIdx := 2

	if opts.Status != "" {
		query += fmt.Sprintf(` AND j.status = $%d`, argIdx)
		args = append(args, opts.Status)
		argIdx++
	}
	if opts.PageToken != "" {
		query += fmt.Sprintf(` AND j.created_at <= (SELECT created_at FROM processing_jobs WHERE id = $%d)`, argIdx)
		args = append(args, opts.PageToken)
		argIdx++
		query += fmt.Sprintf(` AND j.id != $%d`, argIdx)
		args = append(args, opts.PageToken)
		argIdx++
	}

	query += ` ORDER BY j.created_at DESC`

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, pageSize+1)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing processing jobs by topic: %w", err)
	}
	return scanJobRows(rows)
}

// UpdateStatus sets the status of a job. When transitioning to "running",
// started_at is set. When transitioning to "completed" or "failed",
// completed_at is set. If errorMsg is non-nil, the error field is set.
func (s *JobStore) UpdateStatus(ctx context.Context, id, status string, errorMsg *string) error {
	query := `UPDATE processing_jobs SET status = $2`
	args := []any{id, status}
	argIdx := 3

	if status == "running" {
		query += `, started_at = now()`
	}
	if status == "completed" || status == "failed" {
		query += `, completed_at = now()`
	}
	if errorMsg != nil {
		query += fmt.Sprintf(`, error = $%d`, argIdx)
		args = append(args, *errorMsg)
		argIdx++ //nolint:ineffassign // clarity for future additions
	}

	query += ` WHERE id = $1`

	tag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating job status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job not found")
	}
	return nil
}

// ClaimNext atomically finds and claims the next queued job of the given type.
// Uses FOR UPDATE SKIP LOCKED for safe concurrent polling. Returns nil, nil
// when no jobs are available.
func (s *JobStore) ClaimNext(ctx context.Context, jobType string) (*ProcessingJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err) // coverage:ignore - requires connection failure mid-operation
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`UPDATE processing_jobs
		 SET status = 'running', started_at = now()
		 WHERE id = (
		     SELECT id FROM processing_jobs
		     WHERE status = 'queued' AND job_type = $1
		     ORDER BY created_at
		     LIMIT 1
		     FOR UPDATE SKIP LOCKED
		 )
		 RETURNING `+jobColumns,
		jobType,
	)

	j, err := scanJob(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claiming next job: %w", err)
	}

	// coverage:ignore - requires connection failure mid-commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing claim: %w", err)
	}
	return j, nil
}
