package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Tight-Line/creel/internal/store"
)

// mockDBTX implements store.DBTX for unit testing the extraction worker.
type mockDBTX struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, errors.New("not configured")
}

func (m *mockDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{err: errors.New("not configured")}
}

func (m *mockDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

type mockRow struct{ err error }

func (r *mockRow) Scan(_ ...any) error { return r.err }

// newTestExtractionWorker creates an extraction worker with the given docStore
// mock and a jobStore mock that succeeds on Create.
func newTestExtractionWorker(db store.DBTX) *ExtractionWorker {
	jobDB := &mockJobDBTX{}
	return NewExtractionWorker(store.NewDocumentStore(db), store.NewJobStore(jobDB))
}

// mockJobDBTX returns a successful job creation response.
type mockJobDBTX struct{}

func (m *mockJobDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 1"), nil
}

func (m *mockJobDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockJobDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockJobRow{}
}

func (m *mockJobDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

// mockJobRow returns a valid ProcessingJob scan.
type mockJobRow struct{}

func (r *mockJobRow) Scan(dest ...any) error {
	// Scans: id, document_id, job_type, status, progress, error, started_at, completed_at, created_at
	if len(dest) >= 9 {
		*dest[0].(*string) = "job-1"
		*dest[1].(*string) = "doc-1"
		*dest[2].(*string) = "chunking"
		*dest[3].(*string) = "queued"
		*dest[4].(*[]byte) = nil
		*dest[5].(**string) = nil
		*dest[6].(**time.Time) = nil
		*dest[7].(**time.Time) = nil
		*dest[8].(*time.Time) = time.Now()
	}
	return nil
}

func TestExtractionWorker_Process_UpdateStatusProcessingError(t *testing.T) {
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	w := newTestExtractionWorker(db)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractionWorker_Process_GetContentError(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("content not found")}
		},
	}
	w := newTestExtractionWorker(db)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractionWorker_Process_GetContentError_FailedStatusAlsoFails(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			if execCount == 1 {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			return pgconn.CommandTag{}, errors.New("status update failed")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("content not found")}
		},
	}
	w := newTestExtractionWorker(db)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractionWorker_Process_ExtractTextError(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "application/octet-stream"}
		},
	}
	w := newTestExtractionWorker(db)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for unsupported content type")
	}
}

func TestExtractionWorker_Process_ExtractTextError_FailedStatusAlsoFails(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			if execCount <= 1 {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			return pgconn.CommandTag{}, errors.New("status update failed")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "application/octet-stream"}
		},
	}
	w := newTestExtractionWorker(db)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractionWorker_Process_SaveExtractedTextError(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			if execCount <= 1 {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			if execCount == 2 {
				return pgconn.CommandTag{}, errors.New("save failed")
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "text/plain", rawContent: []byte("hello")}
		},
	}
	w := newTestExtractionWorker(db)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for save failure")
	}
}

func TestExtractionWorker_Process_SaveExtractedTextError_FailedStatusAlsoFails(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			if execCount <= 1 {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			return pgconn.CommandTag{}, errors.New("db error")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "text/plain", rawContent: []byte("hello")}
		},
	}
	w := newTestExtractionWorker(db)
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractionWorker_Process_CreateChunkingJobError(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "text/plain", rawContent: []byte("hello")}
		},
	}
	// Use a failing jobStore.
	failingJobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("insert failed")}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db), store.NewJobStore(failingJobDB))
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for job creation failure")
	}
}

func TestExtractionWorker_Process_CreateChunkingJobError_FailedStatusAlsoFails(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			if execCount <= 2 {
				// UpdateStatus("processing") and SaveExtractedText succeed.
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			// UpdateStatus("failed") fails.
			return pgconn.CommandTag{}, errors.New("status update failed")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "text/plain", rawContent: []byte("hello")}
		},
	}
	failingJobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("insert failed")}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db), store.NewJobStore(failingJobDB))
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
}

// mockContentRow implements pgx.Row for document_content queries.
type mockContentRow struct {
	contentType string
	rawContent  []byte
}

func (r *mockContentRow) Scan(dest ...any) error {
	// Scans: document_id, raw_content, content_type, extracted_text, created_at, updated_at
	if len(dest) >= 6 {
		*dest[0].(*string) = "doc-1"
		*dest[1].(*[]byte) = r.rawContent
		*dest[2].(*string) = r.contentType
		*dest[3].(*string) = ""
		// dest[4] and dest[5] are time.Time, zero values are fine
	}
	return nil
}
