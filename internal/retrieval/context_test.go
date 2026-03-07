package retrieval

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

func TestContextFetcher_DocumentNotFound(t *testing.T) {
	// DocumentTopicID returns "document not found" when the doc doesn't exist.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: pgx.ErrNoRows}
		},
	}
	cs := store.NewChunkStore(db)
	authz := &mockAuthorizer{}
	f := NewContextFetcher(cs, authz)

	_, err := f.GetContext(context.Background(), &auth.Principal{ID: "user:test"}, "doc-missing", 0, time.Time{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "resolving document topic") {
		t.Errorf("expected 'resolving document topic' in error, got: %v", err)
	}
}

func TestContextFetcher_AccessDenied(t *testing.T) {
	// DocumentTopicID succeeds, then authorizer denies.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil} // Scan succeeds with zero values
		},
	}
	cs := store.NewChunkStore(db)
	authz := &mockAuthorizer{checkErr: errors.New("permission denied")}
	f := NewContextFetcher(cs, authz)

	_, err := f.GetContext(context.Background(), &auth.Principal{ID: "user:test"}, "doc-1", 0, time.Time{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected 'access denied' in error, got: %v", err)
	}
}

func TestContextFetcher_ListByDocumentError(t *testing.T) {
	// DocumentTopicID succeeds, authorizer passes, ListByDocument fails.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db query failed")
		},
	}
	cs := store.NewChunkStore(db)
	authz := &mockAuthorizer{}
	f := NewContextFetcher(cs, authz)

	_, err := f.GetContext(context.Background(), &auth.Principal{ID: "user:test"}, "doc-1", 0, time.Time{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listing chunks") {
		t.Errorf("expected 'listing chunks' in error, got: %v", err)
	}
}

func TestContextFetcher_Success(t *testing.T) {
	// DocumentTopicID succeeds, authorizer passes, ListByDocument returns empty.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{}, nil // empty result set
		},
	}
	cs := store.NewChunkStore(db)
	authz := &mockAuthorizer{}
	f := NewContextFetcher(cs, authz)

	chunks, err := f.GetContext(context.Background(), &auth.Principal{ID: "user:test"}, "doc-1", 0, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestContextFetcher_SuccessWithLastN(t *testing.T) {
	// Verify lastN parameter is passed through without error.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}
	cs := store.NewChunkStore(db)
	authz := &mockAuthorizer{}
	f := NewContextFetcher(cs, authz)

	chunks, err := f.GetContext(context.Background(), &auth.Principal{ID: "user:test"}, "doc-1", 10, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestContextFetcher_SuccessWithSince(t *testing.T) {
	// Verify since parameter is passed through without error.
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}
	cs := store.NewChunkStore(db)
	authz := &mockAuthorizer{}
	f := NewContextFetcher(cs, authz)

	chunks, err := f.GetContext(context.Background(), &auth.Principal{ID: "user:test"}, "doc-1", 0, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(chunks))
	}
}
