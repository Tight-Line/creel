package worker

import (
	"context"
	"errors"
	"testing"

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

func TestExtractionWorker_Process_UpdateStatusProcessingError(t *testing.T) {
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db))
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
			// First call: UpdateStatus("processing") succeeds.
			// Second call: UpdateStatus("failed") succeeds.
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("content not found")}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db))
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
				// UpdateStatus("processing") succeeds.
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			// UpdateStatus("failed") fails.
			return pgconn.CommandTag{}, errors.New("status update failed")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("content not found")}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db))
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
			// Return content with unsupported type.
			return &mockContentRow{contentType: "application/pdf"}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db))
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
			return &mockContentRow{contentType: "application/pdf"}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db))
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
				// UpdateStatus("processing") succeeds.
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			if execCount == 2 {
				// SaveExtractedText fails.
				return pgconn.CommandTag{}, errors.New("save failed")
			}
			// UpdateStatus("failed") succeeds.
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "text/plain", rawContent: []byte("hello")}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db))
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
			// Both SaveExtractedText and UpdateStatus("failed") fail.
			return pgconn.CommandTag{}, errors.New("db error")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "text/plain", rawContent: []byte("hello")}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db))
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractionWorker_Process_FinalStatusUpdateError(t *testing.T) {
	execCount := 0
	db := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCount++
			if execCount <= 2 {
				// UpdateStatus("processing") and SaveExtractedText succeed.
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			// UpdateStatus("ready") fails.
			return pgconn.CommandTag{}, errors.New("final status error")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockContentRow{contentType: "text/plain", rawContent: []byte("hello")}
		},
	}
	w := NewExtractionWorker(store.NewDocumentStore(db))
	job := &store.ProcessingJob{DocumentID: "doc-1"}
	err := w.Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for final status update failure")
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
