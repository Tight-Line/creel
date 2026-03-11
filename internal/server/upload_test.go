package server

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/fetch"
	"github.com/Tight-Line/creel/internal/store"
)

// ---------------------------------------------------------------------------
// UploadDocument tests
// ---------------------------------------------------------------------------

func TestDocumentServer_UploadDocument_Unauthenticated(t *testing.T) {
	s := NewDocumentServer(nil, nil, nil, nil)
	_, err := s.UploadDocument(context.Background(), &pb.UploadDocumentRequest{TopicId: "t", Name: "n", File: []byte("x")})
	requireCode(t, err, codes.Unauthenticated)
}

func TestDocumentServer_UploadDocument_MissingTopicID(t *testing.T) {
	s := NewDocumentServer(nil, nil, nil, nil)
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{Name: "n", File: []byte("x")})
	requireCode(t, err, codes.InvalidArgument)
}

func TestDocumentServer_UploadDocument_MissingName(t *testing.T) {
	s := NewDocumentServer(nil, nil, nil, nil)
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{TopicId: "t", File: []byte("x")})
	requireCode(t, err, codes.InvalidArgument)
}

func TestDocumentServer_UploadDocument_NeitherFileNorURL(t *testing.T) {
	s := NewDocumentServer(nil, nil, nil, nil)
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{TopicId: "t", Name: "n"})
	requireCode(t, err, codes.InvalidArgument)
}

func TestDocumentServer_UploadDocument_BothFileAndURL(t *testing.T) {
	s := NewDocumentServer(nil, nil, nil, nil)
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId:   "t",
		Name:      "n",
		File:      []byte("x"),
		SourceUrl: "http://example.com",
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestDocumentServer_UploadDocument_PermissionDenied(t *testing.T) {
	s := NewDocumentServer(nil, nil, nil, &mockAuthorizer{checkErr: errors.New("denied")})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId: "t", Name: "n", File: []byte("x"),
	})
	requireCode(t, err, codes.PermissionDenied)
}

func TestDocumentServer_UploadDocument_FetcherError(t *testing.T) {
	f := &mockFetcher{err: errors.New("fetch failed")}
	s := NewDocumentServer(nil, nil, f, &mockAuthorizer{})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId: "t", Name: "n", SourceUrl: "http://example.com",
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestDocumentServer_UploadDocument_NilFetcher(t *testing.T) {
	s := NewDocumentServer(nil, nil, nil, &mockAuthorizer{})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId: "t", Name: "n", SourceUrl: "http://example.com",
	})
	requireCode(t, err, codes.Internal)
}

func TestDocumentServer_UploadDocument_CreateDocError(t *testing.T) {
	db := failDBTX()
	f := &mockFetcher{result: &fetch.Result{Data: []byte("data"), ContentType: "text/plain"}}
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(db), f, &mockAuthorizer{})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId: "t", Name: "n", File: []byte("x"), ContentType: "text/plain",
	})
	requireCode(t, err, codes.Internal)
}

func TestDocumentServer_UploadDocument_SaveContentError(t *testing.T) {
	// CreateWithStatus succeeds but SaveContent fails.
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				// CreateWithStatus succeeds
				return &mockDocRow{}
			}
			return &mockRow{err: errors.New("db error")}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("save content error")
		},
	}
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(db), nil, &mockAuthorizer{})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId: "t", Name: "n", File: []byte("x"),
	})
	requireCode(t, err, codes.Internal)
}

func TestDocumentServer_UploadDocument_CreateJobError(t *testing.T) {
	// CreateWithStatus succeeds, SaveContent succeeds, but Create job fails.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			return &mockDocRow{}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	jobDB := failDBTX()
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(jobDB), nil, &mockAuthorizer{})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId: "t", Name: "n", File: []byte("x"),
	})
	requireCode(t, err, codes.Internal)
}

func TestDocumentServer_UploadDocument_SuccessWithFile(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockDocRow{}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	// Job store also needs a working queryRow for Create.
	jobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockJobRow{}
		},
	}
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(jobDB), nil, &mockAuthorizer{})
	resp, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId:     "t",
		Name:        "n",
		File:        []byte("hello"),
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Document == nil {
		t.Fatal("expected document in response")
	}
	if resp.JobId == "" {
		t.Error("expected non-empty job_id")
	}
}

func TestDocumentServer_UploadDocument_SuccessWithSourceURL(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockDocRow{}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	jobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockJobRow{}
		},
	}
	f := &mockFetcher{result: &fetch.Result{Data: []byte("fetched"), ContentType: "text/html"}}
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(jobDB), f, &mockAuthorizer{})
	resp, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId:   "t",
		Name:      "n",
		SourceUrl: "http://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Document == nil {
		t.Fatal("expected document in response")
	}
}

func TestDocumentServer_UploadDocument_WithCitationFields(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockDocRow{}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	jobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockJobRow{}
		},
	}
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(jobDB), nil, &mockAuthorizer{})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId:     "t",
		Name:        "n",
		File:        []byte("data"),
		Url:         "https://example.com/paper",
		Author:      "Dr. Test",
		PublishedAt: timestamppb.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentServer_UploadDocument_SourceURLDefaultsCitationURL(t *testing.T) {
	var capturedURL any
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			// CreateWithStatus args: topicID, slug, name, docType, status, metaJSON, url, author, publishedAt
			if len(args) >= 7 {
				capturedURL = args[6]
			}
			return &mockDocRow{}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	jobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockJobRow{}
		},
	}
	f := &mockFetcher{result: &fetch.Result{Data: []byte("fetched"), ContentType: "text/html"}}
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(jobDB), f, &mockAuthorizer{})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId:   "t",
		Name:      "n",
		SourceUrl: "http://example.com/doc.pdf",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	urlPtr, ok := capturedURL.(*string)
	if !ok || urlPtr == nil {
		t.Fatal("expected URL to be set from source_url")
	}
	if *urlPtr != "http://example.com/doc.pdf" {
		t.Fatalf("expected URL %q, got %q", "http://example.com/doc.pdf", *urlPtr)
	}
}

func TestDocumentServer_UploadDocument_ExplicitURLOverridesSourceURL(t *testing.T) {
	var capturedURL any
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			if len(args) >= 7 {
				capturedURL = args[6]
			}
			return &mockDocRow{}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	jobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockJobRow{}
		},
	}
	f := &mockFetcher{result: &fetch.Result{Data: []byte("fetched"), ContentType: "text/html"}}
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(jobDB), f, &mockAuthorizer{})
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId:   "t",
		Name:      "n",
		SourceUrl: "http://example.com/doc.pdf",
		Url:       "https://canonical.example.com/paper",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	urlPtr, ok := capturedURL.(*string)
	if !ok || urlPtr == nil {
		t.Fatal("expected URL to be set")
	}
	if *urlPtr != "https://canonical.example.com/paper" {
		t.Fatalf("expected explicit URL %q, got %q", "https://canonical.example.com/paper", *urlPtr)
	}
}

func TestDocumentServer_UploadDocument_ContentTypeOverride(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockDocRow{}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	jobDB := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockJobRow{}
		},
	}
	f := &mockFetcher{result: &fetch.Result{Data: []byte("fetched"), ContentType: "text/html"}}
	s := NewDocumentServer(store.NewDocumentStore(db), store.NewJobStore(jobDB), f, &mockAuthorizer{})
	// Override content type from fetcher result.
	_, err := s.UploadDocument(systemCtx(), &pb.UploadDocumentRequest{
		TopicId:     "t",
		Name:        "n",
		SourceUrl:   "http://example.com",
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Mock helpers for upload tests
// ---------------------------------------------------------------------------

type mockFetcher struct {
	result *fetch.Result
	err    error
}

func (m *mockFetcher) Fetch(_ context.Context, _ string) (*fetch.Result, error) {
	return m.result, m.err
}

// mockDocRow implements pgx.Row for document scans (12 fields with status).
type mockDocRow struct{}

func (r *mockDocRow) Scan(dest ...any) error {
	// Scans: id, topic_id, slug, name, doc_type, status, metadata, created_at, updated_at, url, author, published_at
	if id, ok := dest[0].(*string); ok {
		*id = "doc-1"
	}
	if topicID, ok := dest[1].(*string); ok {
		*topicID = "topic-1"
	}
	if s, ok := dest[2].(*string); ok {
		*s = "test-slug"
	}
	if n, ok := dest[3].(*string); ok {
		*n = "test name"
	}
	if dt, ok := dest[4].(*string); ok {
		*dt = "reference"
	}
	if st, ok := dest[5].(*string); ok {
		*st = "pending"
	}
	if meta, ok := dest[6].(*[]byte); ok {
		*meta = []byte("{}")
	}
	// dest[7] (created_at), dest[8] (updated_at): zero values fine
	// dest[9] (url), dest[10] (author), dest[11] (published_at): nil pointers fine
	return nil
}

// mockJobRow implements pgx.Row for job scans (9 fields).
type mockJobRow struct{}

func (r *mockJobRow) Scan(dest ...any) error {
	// Scans: id, document_id, job_type, status, progress, error, started_at, completed_at, created_at
	if id, ok := dest[0].(*string); ok {
		*id = "job-1"
	}
	if docID, ok := dest[1].(*string); ok {
		*docID = "doc-1"
	}
	if jt, ok := dest[2].(*string); ok {
		*jt = "extraction"
	}
	if st, ok := dest[3].(*string); ok {
		*st = "queued"
	}
	// rest are nil/zero
	return nil
}
