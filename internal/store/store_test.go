package store

import (
	"context"
	"errors"
	"math"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	_, err := s.Create(ctx(), "slug", "name", "desc", "owner")
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
	_, err := s.Update(ctx(), "id", "name", "desc")
	expectErr(t, err, "topic not found")
}

func TestTopicStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewTopicStore(db)
	_, err := s.Update(ctx(), "id", "name", "desc")
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
	_, err := s.Create(ctx(), "tid", "slug", "name", "note", nil)
	expectErr(t, err, "inserting document")
}

func TestDocumentStore_Create_BadMetadata(t *testing.T) {
	s := NewDocumentStore(&mockDBTX{})
	_, err := s.Create(ctx(), "tid", "slug", "name", "note", map[string]any{"bad": math.Inf(1)})
	expectErr(t, err, "marshaling metadata")
}

func TestDocumentStore_Update_BadMetadata(t *testing.T) {
	s := NewDocumentStore(&mockDBTX{})
	_, err := s.Update(ctx(), "id", "name", "note", map[string]any{"bad": math.Inf(1)})
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
	_, err := s.Update(ctx(), "id", "name", "note", nil)
	expectErr(t, err, "document not found")
}

func TestDocumentStore_Update_OtherError(t *testing.T) {
	db := &mockDBTX{queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return &mockRow{err: errMock}
	}}
	s := NewDocumentStore(db)
	_, err := s.Update(ctx(), "id", "name", "note", nil)
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
	_, err := s.ChunkIDsByTopics(ctx(), []string{"t1"})
	expectErr(t, err, "querying chunk IDs")
}

func TestChunkStore_ChunkIDsByTopics_ScanError(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{nextOnce: true, scanErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.ChunkIDsByTopics(ctx(), []string{"t1"})
	expectErr(t, err, "scanning chunk ID")
}

func TestChunkStore_ChunkIDsByTopics_RowsErr(t *testing.T) {
	db := &mockDBTX{queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return &mockRows{iterErr: errMock}, nil
	}}
	s := NewChunkStore(db)
	_, err := s.ChunkIDsByTopics(ctx(), []string{"t1"})
	expectErr(t, err, "mock error")
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
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL not set; skipping integration test")
	}

	pool, err := NewPool(ctx(), pgURL)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	pool.Close()
}

// ---------------------------------------------------------------------------
// RunMigrations error path
// ---------------------------------------------------------------------------

func TestRunMigrations_InvalidPath(t *testing.T) {
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		// Without a real DB, we can still test with a bogus URL and path;
		// the migrator creation itself will fail.
		err := RunMigrations("postgres://invalid:5432/db", "/nonexistent/path")
		expectErr(t, err, "creating migrator")
		return
	}

	err := RunMigrations(pgURL, "/nonexistent/path")
	expectErr(t, err, "creating migrator")
}
