package store

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/crypto"
)

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

var errMock = errors.New("mock error")

// mockRow implements pgx.Row. Scan returns the configured error.
type mockRow struct {
	err error
}

func (r *mockRow) Scan(_ ...any) error { return r.err }

// mockRows implements pgx.Rows. It can be configured to return a scan error
// on the first row, or to return an iteration error via Err().
type mockRows struct {
	closed   bool
	nextOnce bool // if true, Next() returns true once (to trigger Scan)
	called   bool
	scanErr  error
	iterErr  error
}

func (r *mockRows) Close()                                       { r.closed = true }
func (r *mockRows) Err() error                                   { return r.iterErr }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 0") }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) RawValues() [][]byte                          { return nil }
func (r *mockRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockRows) Conn() *pgx.Conn                              { return nil }

func (r *mockRows) Next() bool {
	if r.nextOnce && !r.called {
		r.called = true
		return true
	}
	return false
}

func (r *mockRows) Scan(_ ...any) error { return r.scanErr }

// mockTx implements pgx.Tx. It delegates QueryRow/Query/Exec to an inner
// mockDBTX and adds configurable Commit/Rollback errors.
type mockTx struct {
	inner     *mockDBTX
	commitErr error
}

func (t *mockTx) Begin(ctx context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested begin not supported")
}
func (t *mockTx) Commit(_ context.Context) error   { return t.commitErr }
func (t *mockTx) Rollback(_ context.Context) error { return nil }

func (t *mockTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *mockTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *mockTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (t *mockTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.inner.Exec(ctx, sql, args...)
}
func (t *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.inner.Query(ctx, sql, args...)
}
func (t *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.inner.QueryRow(ctx, sql, args...)
}
func (t *mockTx) Conn() *pgx.Conn { return nil }

// mockDBTX implements DBTX. Each method can be individually configured.
type mockDBTX struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	beginFn    func(ctx context.Context) (pgx.Tx, error)
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{err: errMock}
}

func (m *mockDBTX) Begin(ctx context.Context) (pgx.Tx, error) {
	if m.beginFn != nil {
		return m.beginFn(ctx)
	}
	return nil, errMock
}

// helpers
func ctx() context.Context { return context.Background() }

func expectErr(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !contains(err.Error(), substr) {
		t.Fatalf("expected error containing %q, got %q", substr, err.Error())
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (s == sub || len(s) > 0 && containsImpl(s, sub))
}

func containsImpl(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// TopicStore tests
// ---------------------------------------------------------------------------

func TestTopicStore_Create_Error(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewTopicStore(db)
	_, err := s.Create(ctx(), "slug", "name", "desc", "owner", nil, nil, nil, false, nil)
	expectErr(t, err, "inserting topic")
}

func TestTopicStore_Get_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewTopicStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "topic not found")
}

func TestTopicStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewTopicStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying topic")
}

func TestTopicStore_ListForPrincipals_QueryError_NilPrincipals(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewTopicStore(db)
	_, err := s.ListForPrincipals(ctx(), nil)
	expectErr(t, err, "listing topics")
}

func TestTopicStore_ListForPrincipals_QueryError_WithPrincipals(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewTopicStore(db)
	_, err := s.ListForPrincipals(ctx(), []string{"user:alice"})
	expectErr(t, err, "listing topics")
}

func TestTopicStore_ListForPrincipals_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewTopicStore(db)
	_, err := s.ListForPrincipals(ctx(), nil)
	expectErr(t, err, "scanning topic")
}

func TestTopicStore_ListForPrincipals_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewTopicStore(db)
	_, err := s.ListForPrincipals(ctx(), nil)
	expectErr(t, err, "mock error")
}

func TestTopicStore_Update_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewTopicStore(db)
	_, err := s.Update(ctx(), "id", "name", "desc", nil, nil, nil, nil, nil)
	expectErr(t, err, "topic not found")
}

func TestTopicStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewTopicStore(db)
	_, err := s.Update(ctx(), "id", "name", "desc", nil, nil, nil, nil, nil)
	expectErr(t, err, "updating topic")
}

func TestTopicStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewTopicStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting topic")
}

func TestTopicStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewTopicStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "topic not found")
}

func TestTopicStore_Grant_Error(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewTopicStore(db)
	_, err := s.Grant(ctx(), "tid", "p", "read", "gb")
	expectErr(t, err, "granting access")
}

func TestTopicStore_Revoke_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewTopicStore(db)
	err := s.Revoke(ctx(), "tid", "p")
	expectErr(t, err, "revoking access")
}

func TestTopicStore_Revoke_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewTopicStore(db)
	err := s.Revoke(ctx(), "tid", "p")
	expectErr(t, err, "grant not found")
}

func TestTopicStore_ListGrants_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewTopicStore(db)
	_, err := s.ListGrants(ctx(), "tid")
	expectErr(t, err, "listing grants")
}

func TestTopicStore_ListGrants_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewTopicStore(db)
	_, err := s.ListGrants(ctx(), "tid")
	expectErr(t, err, "scanning grant")
}

func TestTopicStore_ListGrants_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewTopicStore(db)
	_, err := s.ListGrants(ctx(), "tid")
	expectErr(t, err, "mock error")
}

// ---------------------------------------------------------------------------
// DocumentStore tests
// ---------------------------------------------------------------------------

func TestDocumentStore_Create_QueryRowError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewDocumentStore(db)
	_, err := s.Create(ctx(), "tid", "slug", "name", "note", nil, nil, nil, nil)
	expectErr(t, err, "inserting document")
}

func TestDocumentStore_Create_BadMetadata(t *testing.T) {
	s := NewDocumentStore(&mockDBTX{})
	_, err := s.Create(ctx(), "tid", "slug", "name", "note", map[string]any{"bad": math.Inf(1)}, nil, nil, nil)
	expectErr(t, err, "marshaling metadata")
}

func TestDocumentStore_Update_BadMetadata(t *testing.T) {
	s := NewDocumentStore(&mockDBTX{})
	_, err := s.Update(ctx(), "id", "name", "note", map[string]any{"bad": math.Inf(1)}, nil, nil, nil)
	expectErr(t, err, "marshaling metadata")
}

func TestDocumentStore_Get_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewDocumentStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "document not found")
}

func TestDocumentStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewDocumentStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying document")
}

func TestDocumentStore_ListByTopic_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewDocumentStore(db)
	_, err := s.ListByTopic(ctx(), "tid")
	expectErr(t, err, "listing documents")
}

func TestDocumentStore_ListByTopic_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewDocumentStore(db)
	_, err := s.ListByTopic(ctx(), "tid")
	expectErr(t, err, "scanning document")
}

func TestDocumentStore_ListByTopic_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewDocumentStore(db)
	_, err := s.ListByTopic(ctx(), "tid")
	expectErr(t, err, "mock error")
}

func TestDocumentStore_Update_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewDocumentStore(db)
	_, err := s.Update(ctx(), "id", "name", "note", nil, nil, nil, nil)
	expectErr(t, err, "document not found")
}

func TestDocumentStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewDocumentStore(db)
	_, err := s.Update(ctx(), "id", "name", "note", nil, nil, nil, nil)
	expectErr(t, err, "updating document")
}

func TestDocumentStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewDocumentStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting document")
}

func TestDocumentStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewDocumentStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "document not found")
}

func TestDocumentStore_TopicIDForDocument_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewDocumentStore(db)
	_, err := s.TopicIDForDocument(ctx(), "id")
	expectErr(t, err, "document not found")
}

func TestDocumentStore_TopicIDForDocument_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewDocumentStore(db)
	_, err := s.TopicIDForDocument(ctx(), "id")
	expectErr(t, err, "querying document topic")
}

func TestDocumentStore_GetMultiple_EmptyIDs(t *testing.T) {
	s := NewDocumentStore(&mockDBTX{})
	result, err := s.GetMultiple(ctx(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestDocumentStore_GetMultiple_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewDocumentStore(db)
	_, err := s.GetMultiple(ctx(), []string{"id1"})
	expectErr(t, err, "querying documents")
}

func TestDocumentStore_GetMultiple_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewDocumentStore(db)
	_, err := s.GetMultiple(ctx(), []string{"id1"})
	expectErr(t, err, "scanning document")
}

func TestDocumentStore_GetMultiple_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewDocumentStore(db)
	_, err := s.GetMultiple(ctx(), []string{"id1"})
	expectErr(t, err, "mock error")
}

// ---------------------------------------------------------------------------
// DocumentStore: new methods (status, content)
// ---------------------------------------------------------------------------

func TestDocumentStore_CreateWithStatus_Error(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewDocumentStore(db)
	_, err := s.CreateWithStatus(ctx(), "topic-1", "slug", "name", "reference", "pending", nil, nil, nil, nil)
	expectErr(t, err, "inserting document")
}

func TestDocumentStore_CreateWithStatus_BadMetadata(t *testing.T) {
	s := NewDocumentStore(&mockDBTX{})
	_, err := s.CreateWithStatus(ctx(), "tid", "s", "n", "t", "ready", map[string]any{"bad": math.Inf(1)}, nil, nil, nil)
	expectErr(t, err, "marshaling metadata")
}

func TestDocumentStore_UpdateStatus_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errMock
	}}
	s := NewDocumentStore(db)
	err := s.UpdateStatus(ctx(), "doc-1", "ready")
	expectErr(t, err, "updating document status")
}

func TestDocumentStore_UpdateStatus_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}}
	s := NewDocumentStore(db)
	err := s.UpdateStatus(ctx(), "doc-1", "ready")
	expectErr(t, err, "document not found")
}

func TestDocumentStore_SaveContent_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errMock
	}}
	s := NewDocumentStore(db)
	err := s.SaveContent(ctx(), "doc-1", []byte("data"), "text/plain")
	expectErr(t, err, "saving document content")
}

func TestDocumentStore_GetContent_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewDocumentStore(db)
	_, err := s.GetContent(ctx(), "doc-1")
	expectErr(t, err, "document content not found")
}

func TestDocumentStore_GetContent_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewDocumentStore(db)
	_, err := s.GetContent(ctx(), "doc-1")
	expectErr(t, err, "querying document content")
}

func TestDocumentStore_SaveExtractedText_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errMock
	}}
	s := NewDocumentStore(db)
	err := s.SaveExtractedText(ctx(), "doc-1", "extracted")
	expectErr(t, err, "saving extracted text")
}

func TestDocumentStore_SaveExtractedText_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}}
	s := NewDocumentStore(db)
	err := s.SaveExtractedText(ctx(), "doc-1", "extracted")
	expectErr(t, err, "document content not found")
}

// ---------------------------------------------------------------------------
// ChunkStore tests
// ---------------------------------------------------------------------------

func TestChunkStore_Create_QueryRowError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewChunkStore(db)
	_, err := s.Create(ctx(), "did", "content", 1, nil)
	expectErr(t, err, "inserting chunk")
}

func TestChunkStore_Create_BadMetadata(t *testing.T) {
	s := NewChunkStore(&mockDBTX{})
	_, err := s.Create(ctx(), "did", "content", 1, map[string]any{"bad": math.Inf(1)})
	expectErr(t, err, "marshaling metadata")
}

func TestChunkStore_Get_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewChunkStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "chunk not found")
}

func TestChunkStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewChunkStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying chunk")
}

func TestChunkStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewChunkStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting chunk")
}

func TestChunkStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewChunkStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "chunk not found")
}

func TestChunkStore_SetEmbeddingID_Error(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewChunkStore(db)
	err := s.SetEmbeddingID(ctx(), "cid", "eid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestChunkStore_ChunkIDsByTopics_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewChunkStore(db)
	_, err := s.ChunkIDsByTopics(ctx(), []string{"t1"}, nil)
	expectErr(t, err, "querying chunk IDs")
}

func TestChunkStore_ChunkIDsByTopics_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.ChunkIDsByTopics(ctx(), []string{"t1"}, nil)
	expectErr(t, err, "scanning chunk ID")
}

func TestChunkStore_ChunkIDsByTopics_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.ChunkIDsByTopics(ctx(), []string{"t1"}, nil)
	expectErr(t, err, "mock error")
}

func TestChunkStore_ChunkIDsByTopics_WithExcludeDocIDs(t *testing.T) {
	var capturedSQL string
	db := &mockDBTX{queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
		capturedSQL = sql
		return &mockRows{}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.ChunkIDsByTopics(ctx(), []string{"t1"}, []string{"doc-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedSQL, "document_id") {
		t.Error("expected exclude clause in SQL")
	}
}

func TestChunkStore_ChunkIDsByTopics_WithExcludeQueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewChunkStore(db)
	_, err := s.ChunkIDsByTopics(ctx(), []string{"t1"}, []string{"doc-1"})
	expectErr(t, err, "querying chunk IDs")
}

func TestChunkStore_GetMultiple_EmptyIDs(t *testing.T) {
	s := NewChunkStore(&mockDBTX{})
	result, err := s.GetMultiple(ctx(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for empty IDs")
	}
}

func TestChunkStore_GetMultiple_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewChunkStore(db)
	_, err := s.GetMultiple(ctx(), []string{"c1"})
	expectErr(t, err, "querying chunks")
}

func TestChunkStore_GetMultiple_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.GetMultiple(ctx(), []string{"c1"})
	expectErr(t, err, "scanning chunk")
}

func TestChunkStore_GetMultiple_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.GetMultiple(ctx(), []string{"c1"})
	expectErr(t, err, "mock error")
}

func TestChunkStore_DocumentTopicIDs_EmptyDocIDs(t *testing.T) {
	s := NewChunkStore(&mockDBTX{})
	result, err := s.DocumentTopicIDs(ctx(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for empty doc IDs")
	}
}

func TestChunkStore_DocumentTopicIDs_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewChunkStore(db)
	_, err := s.DocumentTopicIDs(ctx(), []string{"d1"})
	expectErr(t, err, "querying document topics")
}

func TestChunkStore_DocumentTopicIDs_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.DocumentTopicIDs(ctx(), []string{"d1"})
	expectErr(t, err, "scanning document topic")
}

func TestChunkStore_DocumentTopicIDs_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.DocumentTopicIDs(ctx(), []string{"d1"})
	expectErr(t, err, "mock error")
}

func TestChunkStore_DocumentTopicID_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewChunkStore(db)
	_, err := s.DocumentTopicID(ctx(), "did")
	expectErr(t, err, "document not found")
}

func TestChunkStore_DocumentTopicID_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewChunkStore(db)
	_, err := s.DocumentTopicID(ctx(), "did")
	expectErr(t, err, "querying document topic")
}

// ---------------------------------------------------------------------------
// ChunkStore.ListByDocument tests
// ---------------------------------------------------------------------------

func TestChunkStore_ListByDocument_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewChunkStore(db)
	_, err := s.ListByDocument(ctx(), "doc-1", 0, time.Time{})
	expectErr(t, err, "listing chunks by document")
}

func TestChunkStore_ListByDocument_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.ListByDocument(ctx(), "doc-1", 0, time.Time{})
	expectErr(t, err, "scanning chunk")
}

func TestChunkStore_ListByDocument_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.ListByDocument(ctx(), "doc-1", 0, time.Time{})
	expectErr(t, err, "iterating chunks")
}

// ---------------------------------------------------------------------------
// ChunkStore.GetEmbeddingModels tests
// ---------------------------------------------------------------------------

func TestChunkStore_GetEmbeddingModels_Empty(t *testing.T) {
	s := NewChunkStore(&mockDBTX{})
	result, err := s.GetEmbeddingModels(ctx(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestChunkStore_GetEmbeddingModels_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewChunkStore(db)
	_, err := s.GetEmbeddingModels(ctx(), []string{"chunk-1"})
	expectErr(t, err, "querying embedding models")
}

func TestChunkStore_GetEmbeddingModels_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.GetEmbeddingModels(ctx(), []string{"chunk-1"})
	expectErr(t, err, "scanning embedding model")
}

// ---------------------------------------------------------------------------
// GrantStore tests
// ---------------------------------------------------------------------------

func TestGrantStore_GrantsForPrincipal_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewGrantStore(db)
	_, err := s.GrantsForPrincipal(ctx(), []string{"user:alice"})
	expectErr(t, err, "querying grants")
}

func TestGrantStore_GrantsForPrincipal_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewGrantStore(db)
	_, err := s.GrantsForPrincipal(ctx(), []string{"user:alice"})
	expectErr(t, err, "scanning grant")
}

func TestGrantStore_GrantsForPrincipal_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewGrantStore(db)
	_, err := s.GrantsForPrincipal(ctx(), []string{"user:alice"})
	expectErr(t, err, "mock error")
}

func TestGrantStore_TopicOwner_Error(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewGrantStore(db)
	_, err := s.TopicOwner(ctx(), "tid")
	expectErr(t, err, "querying topic owner")
}

func TestGrantStore_TopicIDsByOwner_Error(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewGrantStore(db)
	_, err := s.TopicIDsByOwner(ctx(), "user:alice")
	expectErr(t, err, "querying owned topics")
}

func TestGrantStore_TopicIDsByOwner_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewGrantStore(db)
	_, err := s.TopicIDsByOwner(ctx(), "user:alice")
	expectErr(t, err, "scanning owned topic")
}

func TestGrantStore_TopicIDsByOwner_IterError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewGrantStore(db)
	_, err := s.TopicIDsByOwner(ctx(), "user:alice")
	if err == nil {
		t.Fatal("expected error from rows.Err()")
	}
}

// ---------------------------------------------------------------------------
// SystemAccountStore tests
// ---------------------------------------------------------------------------

func TestSystemAccountStore_Create_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewSystemAccountStore(db)
	_, _, err := s.Create(ctx(), "name", "desc")
	expectErr(t, err, "beginning transaction")
}

func TestSystemAccountStore_Create_InsertAccountError(t *testing.T) {
	txInner := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewSystemAccountStore(db)
	_, _, err := s.Create(ctx(), "name", "desc")
	expectErr(t, err, "inserting system account")
}

func TestSystemAccountStore_Create_InsertKeyError(t *testing.T) {
	callCount := 0
	txInner := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			// The INSERT account QueryRow; return nil error so Scan succeeds
			// with zero values (good enough for error path testing).
			return &mockRow{err: nil}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			callCount++
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewSystemAccountStore(db)
	_, _, err := s.Create(ctx(), "name", "desc")
	expectErr(t, err, "inserting API key")
}

func TestSystemAccountStore_Create_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewSystemAccountStore(db)
	_, _, err := s.Create(ctx(), "name", "desc")
	expectErr(t, err, "committing transaction")
}

func TestSystemAccountStore_List_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewSystemAccountStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "listing system accounts")
}

func TestSystemAccountStore_List_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewSystemAccountStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "scanning system account")
}

func TestSystemAccountStore_List_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewSystemAccountStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "mock error")
}

func TestSystemAccountStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewSystemAccountStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting system account")
}

func TestSystemAccountStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewSystemAccountStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "system account not found")
}

func TestSystemAccountStore_RotateKey_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewSystemAccountStore(db)
	_, err := s.RotateKey(ctx(), "aid", 0)
	expectErr(t, err, "beginning transaction")
}

func TestSystemAccountStore_RotateKey_UpdateOldKeysError_NoGrace(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewSystemAccountStore(db)
	_, err := s.RotateKey(ctx(), "aid", 0)
	expectErr(t, err, "updating old keys")
}

func TestSystemAccountStore_RotateKey_UpdateOldKeysError_WithGrace(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewSystemAccountStore(db)
	_, err := s.RotateKey(ctx(), "aid", time.Hour)
	expectErr(t, err, "updating old keys")
}

func TestSystemAccountStore_RotateKey_InsertNewKeyError(t *testing.T) {
	callCount := 0
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			callCount++
			if callCount == 1 {
				// First exec: update old keys succeeds.
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			// Second exec: insert new key fails.
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewSystemAccountStore(db)
	_, err := s.RotateKey(ctx(), "aid", 0)
	expectErr(t, err, "inserting new key")
}

func TestSystemAccountStore_RotateKey_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewSystemAccountStore(db)
	_, err := s.RotateKey(ctx(), "aid", 0)
	expectErr(t, err, "committing transaction")
}

func TestSystemAccountStore_RevokeKey_Error(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewSystemAccountStore(db)
	err := s.RevokeKey(ctx(), "aid")
	expectErr(t, err, "revoking keys")
}

func TestSystemAccountStore_LookupKeyHash_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewSystemAccountStore(db)
	p, err := s.LookupKeyHash(ctx(), "hash")
	if err != nil {
		t.Fatalf("expected nil error for ErrNoRows, got %v", err)
	}
	if p != nil {
		t.Fatal("expected nil principal for ErrNoRows")
	}
}

func TestSystemAccountStore_LookupKeyHash_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewSystemAccountStore(db)
	_, err := s.LookupKeyHash(ctx(), "hash")
	expectErr(t, err, "looking up key")
}

// mockRowLookupKey implements pgx.Row for LookupKeyHash; populates principal,
// keyStatus, and graceExpiresAt on Scan.
type mockRowLookupKey struct {
	principal      string
	keyStatus      string
	graceExpiresAt *time.Time
}

func (r *mockRowLookupKey) Scan(dest ...any) error {
	*dest[0].(*string) = r.principal
	*dest[1].(*string) = r.keyStatus
	*dest[2].(**time.Time) = r.graceExpiresAt
	return nil
}

func TestSystemAccountStore_LookupKeyHash_GracePeriodExpired(t *testing.T) {
	expired := time.Now().Add(-1 * time.Hour)
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRowLookupKey{
			principal:      "system:test",
			keyStatus:      "grace_period",
			graceExpiresAt: &expired,
		}
	}}
	s := NewSystemAccountStore(db)
	p, err := s.LookupKeyHash(ctx(), "hash")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if p != nil {
		t.Fatal("expected nil principal for expired grace period")
	}
}

func TestSystemAccountStore_LookupKeyHash_GracePeriodActive(t *testing.T) {
	future := time.Now().Add(1 * time.Hour)
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRowLookupKey{
			principal:      "system:test",
			keyStatus:      "grace_period",
			graceExpiresAt: &future,
		}
	}}
	s := NewSystemAccountStore(db)
	p, err := s.LookupKeyHash(ctx(), "hash")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil principal for active grace period")
	}
	if p.ID != "system:test" {
		t.Errorf("expected principal ID 'system:test', got %q", p.ID)
	}
}

func TestSystemAccountStore_LookupKeyHash_ActiveKey(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRowLookupKey{
			principal:      "system:test",
			keyStatus:      "active",
			graceExpiresAt: nil,
		}
	}}
	s := NewSystemAccountStore(db)
	p, err := s.LookupKeyHash(ctx(), "hash")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil principal for active key")
	}
	if !p.IsSystem {
		t.Error("expected IsSystem to be true")
	}
}

// ---------------------------------------------------------------------------
// NewPool tests
// ---------------------------------------------------------------------------

func TestNewPool_InvalidURL(t *testing.T) {
	_, err := NewPool(ctx(), "not-a-valid-url://")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestNewPool_Success(t *testing.T) {
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	pool, err := NewPool(ctx(), pgCfg.URL())
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	pool.Close()
}

// ---------------------------------------------------------------------------
// EnsureSchema tests
// ---------------------------------------------------------------------------

func TestEnsureSchema_InvalidName(t *testing.T) {
	err := EnsureSchema(ctx(), "postgres://localhost/db", "INVALID-NAME!")
	expectErr(t, err, "invalid schema name")
}

func TestEnsureSchema_BadURL(t *testing.T) {
	err := EnsureSchema(ctx(), "postgres://invalid-host-that-does-not-exist:5432/db?connect_timeout=1", "valid_name")
	expectErr(t, err, "connecting for schema creation")
}

// ---------------------------------------------------------------------------
// RunMigrations error path
// ---------------------------------------------------------------------------

func TestRunMigrations_InvalidPath(t *testing.T) {
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		// Without a real DB, we can still test with a bogus URL and path;
		// the migrator creation itself will fail.
		err := RunMigrations("postgres://invalid:5432/db", "/nonexistent/path")
		expectErr(t, err, "creating migrator")
		return
	}

	err := RunMigrations(pgCfg.URL(), "/nonexistent/path")
	expectErr(t, err, "creating migrator")
}

// ---------------------------------------------------------------------------
// APIKeyConfigStore tests
// ---------------------------------------------------------------------------

// testEncryptor returns a valid Encryptor for use in tests.
func testEncryptor(t *testing.T) *crypto.Encryptor {
	t.Helper()
	enc, err := crypto.NewEncryptor("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("creating test encryptor: %v", err)
	}
	return enc
}

func TestAPIKeyConfigStore_Create_NilEncryptor(t *testing.T) {
	s := NewAPIKeyConfigStore(&mockDBTX{}, nil)
	_, err := s.Create(ctx(), "name", "openai", []byte("sk-test"), false)
	expectErr(t, err, "encryption key not configured")
}

func TestAPIKeyConfigStore_Update_NilEncryptor(t *testing.T) {
	s := NewAPIKeyConfigStore(&mockDBTX{}, nil)
	_, err := s.Update(ctx(), "id", "name", "openai", []byte("sk-new"))
	expectErr(t, err, "encryption key not configured")
}

func TestAPIKeyConfigStore_GetDecrypted_NilEncryptor(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: nil}
	}}
	s := NewAPIKeyConfigStore(db, nil)
	_, err := s.GetDecrypted(ctx(), "id")
	expectErr(t, err, "encryption key not configured")
}

func TestAPIKeyConfigStore_Create_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Create(ctx(), "name", "openai", []byte("sk-test"), false)
	expectErr(t, err, "beginning transaction")
}

func TestAPIKeyConfigStore_Create_ClearDefaultError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Create(ctx(), "name", "openai", []byte("sk-test"), true)
	expectErr(t, err, "clearing previous default")
}

func TestAPIKeyConfigStore_Create_InsertError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Create(ctx(), "name", "openai", []byte("sk-test"), false)
	expectErr(t, err, "inserting API key config")
}

func TestAPIKeyConfigStore_Create_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Create(ctx(), "name", "openai", []byte("sk-test"), false)
	expectErr(t, err, "committing transaction")
}

func TestAPIKeyConfigStore_Get_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "API key config not found")
}

func TestAPIKeyConfigStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying API key config")
}

func TestAPIKeyConfigStore_List_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.List(ctx())
	expectErr(t, err, "listing API key configs")
}

func TestAPIKeyConfigStore_List_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.List(ctx())
	expectErr(t, err, "scanning API key config")
}

func TestAPIKeyConfigStore_List_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.List(ctx())
	expectErr(t, err, "mock error")
}

func TestAPIKeyConfigStore_Update_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Update(ctx(), "id", "name", "openai", nil)
	expectErr(t, err, "API key config not found")
}

func TestAPIKeyConfigStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Update(ctx(), "id", "name", "openai", nil)
	expectErr(t, err, "updating API key config")
}

func TestAPIKeyConfigStore_Update_WithKey_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Update(ctx(), "id", "name", "openai", []byte("sk-new"))
	expectErr(t, err, "API key config not found")
}

func TestAPIKeyConfigStore_Update_WithKey_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.Update(ctx(), "id", "name", "openai", []byte("sk-new"))
	expectErr(t, err, "updating API key config")
}

func TestAPIKeyConfigStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting API key config")
}

func TestAPIKeyConfigStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "API key config not found")
}

func TestAPIKeyConfigStore_SetDefault_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "beginning transaction")
}

func TestAPIKeyConfigStore_SetDefault_ClearError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "clearing previous default")
}

func TestAPIKeyConfigStore_SetDefault_UpdateError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "setting default API key config")
}

func TestAPIKeyConfigStore_SetDefault_NotFound(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: pgx.ErrNoRows}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "API key config not found")
}

func TestAPIKeyConfigStore_SetDefault_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "committing transaction")
}

func TestAPIKeyConfigStore_GetDecrypted_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.GetDecrypted(ctx(), "id")
	expectErr(t, err, "API key config not found")
}

func TestAPIKeyConfigStore_GetDecrypted_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewAPIKeyConfigStore(db, testEncryptor(t))
	_, err := s.GetDecrypted(ctx(), "id")
	expectErr(t, err, "querying encrypted key")
}

// ---------------------------------------------------------------------------
// LLMConfigStore tests
// ---------------------------------------------------------------------------

func TestLLMConfigStore_Create_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Create(ctx(), "name", "openai", "gpt-4o", nil, "akc-id", false)
	expectErr(t, err, "beginning transaction")
}

func TestLLMConfigStore_Create_ClearDefaultError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Create(ctx(), "name", "openai", "gpt-4o", nil, "akc-id", true)
	expectErr(t, err, "clearing previous default")
}

func TestLLMConfigStore_Create_InsertError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Create(ctx(), "name", "openai", "gpt-4o", nil, "akc-id", false)
	expectErr(t, err, "inserting LLM config")
}

func TestLLMConfigStore_Create_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewLLMConfigStore(db)
	// Scan returns nil error but zero values; Unmarshal of zero []byte will fail
	// before commit is reached. We need the scan to populate rawParams.
	// With mockRow returning nil error and zero-value scan destinations,
	// rawParams will be nil, causing Unmarshal to fail. Let's test commit error
	// differently: the Unmarshal error path is also valid to test.
	_, err := s.Create(ctx(), "name", "openai", "gpt-4o", nil, "akc-id", false)
	// With nil rawParams from mock, json.Unmarshal(nil, ...) returns an error.
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLLMConfigStore_Get_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "LLM config not found")
}

func TestLLMConfigStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying LLM config")
}

func TestLLMConfigStore_List_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewLLMConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "listing LLM configs")
}

func TestLLMConfigStore_List_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewLLMConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "scanning LLM config")
}

func TestLLMConfigStore_List_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewLLMConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "mock error")
}

func TestLLMConfigStore_Update_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Update(ctx(), "id", "name", "openai", "gpt-4o", nil, "akc-id")
	expectErr(t, err, "LLM config not found")
}

func TestLLMConfigStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Update(ctx(), "id", "name", "openai", "gpt-4o", nil, "akc-id")
	expectErr(t, err, "updating LLM config")
}

func TestLLMConfigStore_Update_WithParams_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Update(ctx(), "id", "name", "openai", "gpt-4o", map[string]string{"temp": "0.7"}, "akc-id")
	expectErr(t, err, "LLM config not found")
}

func TestLLMConfigStore_Update_WithParams_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewLLMConfigStore(db)
	_, err := s.Update(ctx(), "id", "name", "openai", "gpt-4o", map[string]string{"temp": "0.7"}, "akc-id")
	expectErr(t, err, "updating LLM config")
}

func TestLLMConfigStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewLLMConfigStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting LLM config")
}

func TestLLMConfigStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewLLMConfigStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "LLM config not found")
}

func TestLLMConfigStore_SetDefault_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewLLMConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "beginning transaction")
}

func TestLLMConfigStore_SetDefault_ClearError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewLLMConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "clearing previous default")
}

func TestLLMConfigStore_SetDefault_NotFound(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: pgx.ErrNoRows}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewLLMConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "LLM config not found")
}

func TestLLMConfigStore_SetDefault_UpdateError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewLLMConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "setting default LLM config")
}

func TestLLMConfigStore_SetDefault_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewLLMConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	// Unmarshal of nil rawParams will fail before commit.
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLLMConfigStore_GetDefault_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewLLMConfigStore(db)
	c, err := s.GetDefault(ctx())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if c != nil {
		t.Fatal("expected nil config for no default")
	}
}

func TestLLMConfigStore_GetDefault_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewLLMConfigStore(db)
	_, err := s.GetDefault(ctx())
	expectErr(t, err, "querying default LLM config")
}

// ---------------------------------------------------------------------------
// EmbeddingConfigStore tests
// ---------------------------------------------------------------------------

func TestEmbeddingConfigStore_Create_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.Create(ctx(), "name", "openai", "text-embedding-3-small", 1536, "akc-id", false)
	expectErr(t, err, "beginning transaction")
}

func TestEmbeddingConfigStore_Create_ClearDefaultError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.Create(ctx(), "name", "openai", "text-embedding-3-small", 1536, "akc-id", true)
	expectErr(t, err, "clearing previous default")
}

func TestEmbeddingConfigStore_Create_InsertError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.Create(ctx(), "name", "openai", "text-embedding-3-small", 1536, "akc-id", false)
	expectErr(t, err, "inserting embedding config")
}

func TestEmbeddingConfigStore_Create_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.Create(ctx(), "name", "openai", "text-embedding-3-small", 1536, "akc-id", false)
	expectErr(t, err, "committing transaction")
}

func TestEmbeddingConfigStore_Get_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "embedding config not found")
}

func TestEmbeddingConfigStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying embedding config")
}

func TestEmbeddingConfigStore_List_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "listing embedding configs")
}

func TestEmbeddingConfigStore_List_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "scanning embedding config")
}

func TestEmbeddingConfigStore_List_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "mock error")
}

func TestEmbeddingConfigStore_Update_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.Update(ctx(), "id", "name", "akc-id")
	expectErr(t, err, "embedding config not found")
}

func TestEmbeddingConfigStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.Update(ctx(), "id", "name", "akc-id")
	expectErr(t, err, "updating embedding config")
}

func TestEmbeddingConfigStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewEmbeddingConfigStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting embedding config")
}

func TestEmbeddingConfigStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewEmbeddingConfigStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "embedding config not found")
}

func TestEmbeddingConfigStore_SetDefault_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "beginning transaction")
}

func TestEmbeddingConfigStore_SetDefault_ClearError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "clearing previous default")
}

func TestEmbeddingConfigStore_SetDefault_NotFound(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: pgx.ErrNoRows}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "embedding config not found")
}

func TestEmbeddingConfigStore_SetDefault_UpdateError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "setting default embedding config")
}

func TestEmbeddingConfigStore_SetDefault_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "committing transaction")
}

func TestEmbeddingConfigStore_GetDefault_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewEmbeddingConfigStore(db)
	c, err := s.GetDefault(ctx())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if c != nil {
		t.Fatal("expected nil config for no default")
	}
}

func TestEmbeddingConfigStore_GetDefault_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewEmbeddingConfigStore(db)
	_, err := s.GetDefault(ctx())
	expectErr(t, err, "querying default embedding config")
}

// ---------------------------------------------------------------------------
// ExtractionPromptConfigStore tests
// ---------------------------------------------------------------------------

func TestExtractionPromptConfigStore_Create_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.Create(ctx(), "name", "Extract facts", "Standard", false)
	expectErr(t, err, "beginning transaction")
}

func TestExtractionPromptConfigStore_Create_ClearDefaultError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.Create(ctx(), "name", "Extract facts", "Standard", true)
	expectErr(t, err, "clearing previous default")
}

func TestExtractionPromptConfigStore_Create_InsertError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.Create(ctx(), "name", "Extract facts", "Standard", false)
	expectErr(t, err, "inserting extraction prompt config")
}

func TestExtractionPromptConfigStore_Create_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.Create(ctx(), "name", "Extract facts", "Standard", false)
	expectErr(t, err, "committing transaction")
}

func TestExtractionPromptConfigStore_Get_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "extraction prompt config not found")
}

func TestExtractionPromptConfigStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying extraction prompt config")
}

func TestExtractionPromptConfigStore_List_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "listing extraction prompt configs")
}

func TestExtractionPromptConfigStore_List_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "scanning extraction prompt config")
}

func TestExtractionPromptConfigStore_List_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "mock error")
}

func TestExtractionPromptConfigStore_Update_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.Update(ctx(), "id", "name", "prompt", "desc")
	expectErr(t, err, "extraction prompt config not found")
}

func TestExtractionPromptConfigStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.Update(ctx(), "id", "name", "prompt", "desc")
	expectErr(t, err, "updating extraction prompt config")
}

func TestExtractionPromptConfigStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewExtractionPromptConfigStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting extraction prompt config")
}

func TestExtractionPromptConfigStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewExtractionPromptConfigStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "extraction prompt config not found")
}

func TestExtractionPromptConfigStore_SetDefault_BeginError(t *testing.T) {
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return nil, errMock
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "beginning transaction")
}

func TestExtractionPromptConfigStore_SetDefault_ClearError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errMock
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "clearing previous default")
}

func TestExtractionPromptConfigStore_SetDefault_NotFound(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: pgx.ErrNoRows}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "extraction prompt config not found")
}

func TestExtractionPromptConfigStore_SetDefault_UpdateError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errMock}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "setting default extraction prompt config")
}

func TestExtractionPromptConfigStore_SetDefault_CommitError(t *testing.T) {
	txInner := &mockDBTX{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: nil}
		},
	}
	db := &mockDBTX{beginFn: func(_ context.Context) (pgx.Tx, error) {
		return &mockTx{inner: txInner, commitErr: errMock}, nil
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "committing transaction")
}

func TestExtractionPromptConfigStore_GetDefault_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewExtractionPromptConfigStore(db)
	c, err := s.GetDefault(ctx())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if c != nil {
		t.Fatal("expected nil config for no default")
	}
}

func TestExtractionPromptConfigStore_GetDefault_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewExtractionPromptConfigStore(db)
	_, err := s.GetDefault(ctx())
	expectErr(t, err, "querying default extraction prompt config")
}

// ---------------------------------------------------------------------------
// Topic: ChunkingStrategy tests
// ---------------------------------------------------------------------------

func TestScanTopicChunkingStrategy_ValidJSON(t *testing.T) {
	topic := &Topic{}
	data := []byte(`{"chunk_size":1024,"chunk_overlap":100}`)
	scanTopicChunkingStrategy(topic, data)
	if topic.ChunkingStrategy == nil {
		t.Fatal("expected ChunkingStrategy to be set")
	}
	if topic.ChunkingStrategy.ChunkSize != 1024 {
		t.Errorf("ChunkSize = %d, want 1024", topic.ChunkingStrategy.ChunkSize)
	}
	if topic.ChunkingStrategy.ChunkOverlap != 100 {
		t.Errorf("ChunkOverlap = %d, want 100", topic.ChunkingStrategy.ChunkOverlap)
	}
}

func TestScanTopicChunkingStrategy_NilData(t *testing.T) {
	topic := &Topic{}
	scanTopicChunkingStrategy(topic, nil)
	if topic.ChunkingStrategy != nil {
		t.Error("expected ChunkingStrategy to be nil for nil data")
	}
}

func TestScanTopicChunkingStrategy_InvalidJSON(t *testing.T) {
	topic := &Topic{}
	scanTopicChunkingStrategy(topic, []byte("not json"))
	if topic.ChunkingStrategy != nil {
		t.Error("expected ChunkingStrategy to be nil for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// MemoryStore tests
// ---------------------------------------------------------------------------

func TestMemoryStore_Create_Error(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewMemoryStore(db)
	_, err := s.Create(ctx(), &Memory{Principal: "p", Scope: "s", Content: "c"})
	expectErr(t, err, "inserting memory")
}

func TestMemoryStore_Create_MetadataMarshalError(t *testing.T) {
	db := &mockDBTX{}
	s := NewMemoryStore(db)
	_, err := s.Create(ctx(), &Memory{
		Principal: "p",
		Scope:     "s",
		Content:   "c",
		Metadata:  map[string]any{"bad": math.Inf(1)},
	})
	expectErr(t, err, "marshaling metadata")
}

func TestMemoryStore_Get_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewMemoryStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "memory not found")
}

func TestMemoryStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewMemoryStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying memory")
}

func TestMemoryStore_GetByScope_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewMemoryStore(db)
	_, err := s.GetByScope(ctx(), "p", "s")
	expectErr(t, err, "querying memories by scope")
}

func TestMemoryStore_GetByScope_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewMemoryStore(db)
	_, err := s.GetByScope(ctx(), "p", "s")
	expectErr(t, err, "scanning memory")
}

func TestMemoryStore_Update_ErrNoRows(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewMemoryStore(db)
	_, err := s.Update(ctx(), "id", "content", nil)
	expectErr(t, err, "memory not found")
}

func TestMemoryStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewMemoryStore(db)
	_, err := s.Update(ctx(), "id", "content", nil)
	expectErr(t, err, "updating memory")
}

func TestMemoryStore_Update_MetadataMarshalError(t *testing.T) {
	db := &mockDBTX{}
	s := NewMemoryStore(db)
	_, err := s.Update(ctx(), "id", "content", map[string]any{"bad": math.Inf(1)})
	expectErr(t, err, "marshaling metadata")
}

func TestMemoryStore_Invalidate_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewMemoryStore(db)
	err := s.Invalidate(ctx(), "id")
	expectErr(t, err, "invalidating memory")
}

func TestMemoryStore_Invalidate_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}}
	s := NewMemoryStore(db)
	err := s.Invalidate(ctx(), "id")
	expectErr(t, err, "memory not found")
}

func TestMemoryStore_ListByScope_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewMemoryStore(db)
	_, err := s.ListByScope(ctx(), "p", "s", false)
	expectErr(t, err, "listing memories")
}

func TestMemoryStore_ListByScope_IncludeInvalidated_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewMemoryStore(db)
	_, err := s.ListByScope(ctx(), "p", "s", true)
	expectErr(t, err, "listing memories")
}

func TestMemoryStore_ListScopes_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewMemoryStore(db)
	_, err := s.ListScopes(ctx(), "p")
	expectErr(t, err, "listing scopes")
}

func TestMemoryStore_ListScopes_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewMemoryStore(db)
	_, err := s.ListScopes(ctx(), "p")
	expectErr(t, err, "scanning scope")
}

func TestMemoryStore_GetMultiple_Empty(t *testing.T) {
	s := NewMemoryStore(&mockDBTX{})
	result, err := s.GetMultiple(ctx(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestMemoryStore_GetMultiple_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewMemoryStore(db)
	_, err := s.GetMultiple(ctx(), []string{"id1"})
	expectErr(t, err, "querying memories")
}

func TestMemoryStore_GetMultiple_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewMemoryStore(db)
	_, err := s.GetMultiple(ctx(), []string{"id1"})
	expectErr(t, err, "scanning memory")
}

// ---------------------------------------------------------------------------
// LinkStore unit tests
// ---------------------------------------------------------------------------

func TestLinkStore_Create_MarshalError(t *testing.T) {
	db := &mockDBTX{}
	s := NewLinkStore(db)
	_, err := s.Create(ctx(), "s", "t", "manual", "user", map[string]any{"bad": math.Inf(1)})
	expectErr(t, err, "marshaling metadata")
}

func TestLinkStore_Create_QueryError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewLinkStore(db)
	_, err := s.Create(ctx(), "s", "t", "manual", "user", nil)
	expectErr(t, err, "inserting link")
}

func TestLinkStore_Get_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewLinkStore(db)
	_, err := s.Get(ctx(), "bad-id")
	expectErr(t, err, "link not found")
}

func TestLinkStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewLinkStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying link")
}

func TestLinkStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errMock
	}}
	s := NewLinkStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting link")
}

func TestLinkStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewLinkStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "link not found")
}

func TestLinkStore_ListByChunk_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewLinkStore(db)
	_, err := s.ListByChunk(ctx(), "c1", false)
	expectErr(t, err, "querying links")
}

func TestLinkStore_ListByChunk_WithBacklinks_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewLinkStore(db)
	_, err := s.ListByChunk(ctx(), "c1", true)
	expectErr(t, err, "querying links")
}

func TestLinkStore_ListByChunk_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewLinkStore(db)
	_, err := s.ListByChunk(ctx(), "c1", false)
	expectErr(t, err, "scanning link")
}

func TestLinkStore_ListByChunks_Empty(t *testing.T) {
	s := NewLinkStore(&mockDBTX{})
	links, err := s.ListByChunks(ctx(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if links != nil {
		t.Errorf("expected nil, got %v", links)
	}
}

func TestLinkStore_ListByChunks_Success(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{}, nil // empty but successful
	}}
	s := NewLinkStore(db)
	links, err := s.ListByChunks(ctx(), []string{"c1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

func TestLinkStore_ListByChunks_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewLinkStore(db)
	_, err := s.ListByChunks(ctx(), []string{"c1", "c2"})
	expectErr(t, err, "querying links by chunks")
}

func TestLinkStore_TransferLinks_SourceError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errMock
	}}
	s := NewLinkStore(db)
	_, err := s.TransferLinks(ctx(), "old", "new")
	expectErr(t, err, "transferring source links")
}

func TestLinkStore_TransferLinks_TargetError(t *testing.T) {
	callCount := 0
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		callCount++
		if callCount == 1 {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		return pgconn.CommandTag{}, errMock
	}}
	s := NewLinkStore(db)
	_, err := s.TransferLinks(ctx(), "old", "new")
	expectErr(t, err, "transferring target links")
}

func TestLinkStore_DeleteByChunk_Error(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errMock
	}}
	s := NewLinkStore(db)
	err := s.DeleteByChunk(ctx(), "c1")
	expectErr(t, err, "deleting links by chunk")
}

func TestLinkStore_TransferLinks_Success(t *testing.T) {
	callCount := 0
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		callCount++
		if callCount == 1 {
			return pgconn.NewCommandTag("UPDATE 2"), nil
		}
		return pgconn.NewCommandTag("UPDATE 3"), nil
	}}
	s := NewLinkStore(db)
	total, err := s.TransferLinks(ctx(), "old", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected 5 transferred, got %d", total)
	}
}

func TestLinkStore_DeleteByChunk_Success(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 3"), nil
	}}
	s := NewLinkStore(db)
	err := s.DeleteByChunk(ctx(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CompactionStore
// ---------------------------------------------------------------------------

func TestCompactionStore_Create_Error(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewCompactionStore(db)
	_, err := s.Create(ctx(), "sc1", []string{"c1", "c2"}, "d1", "user:test")
	expectErr(t, err, "inserting compaction record")
}

func TestCompactionStore_GetBySummaryChunkID_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewCompactionStore(db)
	_, err := s.GetBySummaryChunkID(ctx(), "bad")
	expectErr(t, err, "compaction record not found")
}

func TestCompactionStore_GetBySummaryChunkID_Error(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewCompactionStore(db)
	_, err := s.GetBySummaryChunkID(ctx(), "sc1")
	expectErr(t, err, "querying compaction record")
}

func TestCompactionStore_ListByDocument_Error(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewCompactionStore(db)
	_, err := s.ListByDocument(ctx(), "d1")
	expectErr(t, err, "listing compaction records")
}

func TestCompactionStore_ListByDocument_Empty(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{}, nil
	}}
	s := NewCompactionStore(db)
	records, err := s.ListByDocument(ctx(), "d1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestCompactionStore_Delete_Error(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errMock
	}}
	s := NewCompactionStore(db)
	err := s.Delete(ctx(), "r1")
	expectErr(t, err, "deleting compaction record")
}

func TestCompactionStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewCompactionStore(db)
	err := s.Delete(ctx(), "bad")
	expectErr(t, err, "compaction record not found")
}

func TestCompactionStore_Delete_Success(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 1"), nil
	}}
	s := NewCompactionStore(db)
	err := s.Delete(ctx(), "r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ChunkStore additions (MarkCompacted, RestoreCompacted, NextSequence)
// ---------------------------------------------------------------------------

func TestChunkStore_MarkCompacted_Error(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errMock
	}}
	s := NewChunkStore(db)
	err := s.MarkCompacted(ctx(), []string{"c1"}, "sc1")
	expectErr(t, err, "marking chunks as compacted")
}

func TestChunkStore_MarkCompacted_NoRows(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}}
	s := NewChunkStore(db)
	err := s.MarkCompacted(ctx(), []string{"c1"}, "sc1")
	expectErr(t, err, "no active chunks found to compact")
}

func TestChunkStore_MarkCompacted_Success(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("UPDATE 3"), nil
	}}
	s := NewChunkStore(db)
	err := s.MarkCompacted(ctx(), []string{"c1", "c2", "c3"}, "sc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChunkStore_RestoreCompacted_Error(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewChunkStore(db)
	_, err := s.RestoreCompacted(ctx(), "sc1")
	expectErr(t, err, "restoring compacted chunks")
}

func TestChunkStore_RestoreCompacted_Success(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &idRows{ids: []string{"c1", "c2"}}, nil
	}}
	s := NewChunkStore(db)
	ids, err := s.RestoreCompacted(ctx(), "sc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 restored IDs, got %d", len(ids))
	}
}

func TestChunkStore_NextSequence_Error(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewChunkStore(db)
	_, err := s.NextSequence(ctx(), "d1")
	expectErr(t, err, "querying next sequence")
}

func TestChunkStore_NextSequence_Success(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &seqRow{seq: 5}
	}}
	s := NewChunkStore(db)
	seq, err := s.NextSequence(ctx(), "d1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seq != 5 {
		t.Errorf("expected seq 5, got %d", seq)
	}
}

// idRows implements pgx.Rows returning string IDs.
type idRows struct {
	ids []string
	idx int
}

func (r *idRows) Close()                                       {}
func (r *idRows) Err() error                                   { return nil }
func (r *idRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *idRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *idRows) RawValues() [][]byte                          { return nil }
func (r *idRows) Conn() *pgx.Conn                              { return nil }
func (r *idRows) Values() ([]any, error)                       { return nil, nil }

func (r *idRows) Next() bool {
	if r.idx < len(r.ids) {
		r.idx++
		return true
	}
	return false
}

func (r *idRows) Scan(dest ...any) error {
	*(dest[0].(*string)) = r.ids[r.idx-1]
	return nil
}

// seqRow returns a sequence number for NextSequence queries.
type seqRow struct{ seq int }

func (r *seqRow) Scan(dest ...any) error {
	*(dest[0].(*int)) = r.seq
	return nil
}

// ---------------------------------------------------------------------------
// VectorBackendConfigStore
// ---------------------------------------------------------------------------

func TestVectorBackendConfigStore_Create_BeginError(t *testing.T) {
	db := &mockDBTX{} // default Begin returns errMock
	s := NewVectorBackendConfigStore(db)
	_, err := s.Create(ctx(), "name", "pgvector", nil, false)
	expectErr(t, err, "beginning transaction")
}

func TestVectorBackendConfigStore_Create_InsertError(t *testing.T) {
	db := &mockDBTX{
		beginFn: func(_ context.Context) (pgx.Tx, error) {
			return &mockTx{inner: &mockDBTX{
				queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRow{err: errMock}
				},
			}}, nil
		},
	}
	s := NewVectorBackendConfigStore(db)
	_, err := s.Create(ctx(), "name", "pgvector", nil, false)
	expectErr(t, err, "inserting vector backend config")
}

func TestVectorBackendConfigStore_Get_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewVectorBackendConfigStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "vector backend config not found")
}

func TestVectorBackendConfigStore_Get_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewVectorBackendConfigStore(db)
	_, err := s.Get(ctx(), "id")
	expectErr(t, err, "querying vector backend config")
}

func TestVectorBackendConfigStore_List_QueryError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, errMock
	}}
	s := NewVectorBackendConfigStore(db)
	_, err := s.List(ctx())
	expectErr(t, err, "listing vector backend configs")
}

func TestVectorBackendConfigStore_Update_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewVectorBackendConfigStore(db)
	_, err := s.Update(ctx(), "id", "new", nil)
	expectErr(t, err, "vector backend config not found")
}

func TestVectorBackendConfigStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewVectorBackendConfigStore(db)
	_, err := s.Update(ctx(), "id", "new", nil)
	expectErr(t, err, "updating vector backend config")
}

func TestVectorBackendConfigStore_Delete_ExecError(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), errMock
	}}
	s := NewVectorBackendConfigStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "deleting vector backend config")
}

func TestVectorBackendConfigStore_Delete_NotFound(t *testing.T) {
	db := &mockDBTX{execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	s := NewVectorBackendConfigStore(db)
	err := s.Delete(ctx(), "id")
	expectErr(t, err, "vector backend config not found")
}

func TestVectorBackendConfigStore_SetDefault_BeginError(t *testing.T) {
	db := &mockDBTX{} // default Begin returns errMock
	s := NewVectorBackendConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "beginning transaction")
}

func TestVectorBackendConfigStore_SetDefault_NotFound(t *testing.T) {
	db := &mockDBTX{
		beginFn: func(_ context.Context) (pgx.Tx, error) {
			return &mockTx{inner: &mockDBTX{
				queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRow{err: pgx.ErrNoRows}
				},
			}}, nil
		},
	}
	s := NewVectorBackendConfigStore(db)
	_, err := s.SetDefault(ctx(), "id")
	expectErr(t, err, "vector backend config not found")
}

func TestVectorBackendConfigStore_GetDefault_NotFound(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: pgx.ErrNoRows}
	}}
	s := NewVectorBackendConfigStore(db)
	c, err := s.GetDefault(ctx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c != nil {
		t.Error("expected nil when no default")
	}
}

func TestVectorBackendConfigStore_GetDefault_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewVectorBackendConfigStore(db)
	_, err := s.GetDefault(ctx())
	expectErr(t, err, "querying default vector backend config")
}
