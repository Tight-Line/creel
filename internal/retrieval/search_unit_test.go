package retrieval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockAuthorizer implements auth.Authorizer for unit tests.
type mockAuthorizer struct {
	checkErr     error
	accessTopics []string
	accessErr    error
}

func (m *mockAuthorizer) Check(_ context.Context, _ *auth.Principal, _ string, _ auth.Action) error {
	return m.checkErr
}

func (m *mockAuthorizer) AccessibleTopics(_ context.Context, _ *auth.Principal, _ auth.Action) ([]string, error) {
	return m.accessTopics, m.accessErr
}

// mockBackend implements vector.Backend for unit tests.
type mockBackend struct {
	results []vector.SearchResult
	err     error
	dim     int
}

func (m *mockBackend) Store(_ context.Context, _ string, _ []float64, _ map[string]any) error {
	return nil
}
func (m *mockBackend) Delete(_ context.Context, _ string) error                 { return nil }
func (m *mockBackend) StoreBatch(_ context.Context, _ []vector.StoreItem) error { return nil }
func (m *mockBackend) DeleteBatch(_ context.Context, _ []string) error          { return nil }
func (m *mockBackend) Ping(_ context.Context) error                             { return nil }
func (m *mockBackend) EmbeddingDimension() int                                  { return m.dim }
func (m *mockBackend) Search(_ context.Context, _ []float64, _ vector.Filter, _ int) ([]vector.SearchResult, error) {
	return m.results, m.err
}

// mockRow implements pgx.Row, returning an error on Scan.
type mockRow struct {
	err error
}

func (m *mockRow) Scan(_ ...any) error { return m.err }

// mockRows implements pgx.Rows for controlling query results.
type mockRows struct {
	closed bool
	data   [][]any
	idx    int
	err    error
}

func (m *mockRows) Close()                                       { m.closed = true }
func (m *mockRows) Err() error                                   { return m.err }
func (m *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) RawValues() [][]byte                          { return nil }
func (m *mockRows) Conn() *pgx.Conn                              { return nil }

func (m *mockRows) Next() bool {
	if m.idx < len(m.data) {
		m.idx++
		return true
	}
	return false
}

func (m *mockRows) Scan(dest ...any) error {
	row := m.data[m.idx-1]
	for i, d := range dest {
		switch ptr := d.(type) {
		case *string:
			*ptr = row[i].(string)
		case *int:
			*ptr = row[i].(int)
		case **string:
			if row[i] == nil {
				*ptr = nil
			} else {
				s := row[i].(string)
				*ptr = &s
			}
		}
	}
	return nil
}

func (m *mockRows) Values() ([]any, error) { return nil, nil }

// mockDBTX implements store.DBTX, routing queries by SQL content.
type mockDBTX struct {
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
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
	return &mockRow{err: pgx.ErrNoRows}
}

func (m *mockDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not implemented")
}

func TestSearcher_ResolveTopicsError(t *testing.T) {
	authz := &mockAuthorizer{accessErr: errors.New("authz unavailable")}
	backend := &mockBackend{dim: 3}
	cs := store.NewChunkStore(&mockDBTX{})
	s := NewSearcher(cs, authz, backend)

	_, err := s.Search(context.Background(), &auth.Principal{ID: "user:test"}, nil, []float64{1, 2, 3}, 10, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fetching accessible topics") {
		t.Errorf("expected 'fetching accessible topics' in error, got: %v", err)
	}
}

func TestSearcher_ChunkIDsByTopicsError(t *testing.T) {
	authz := &mockAuthorizer{accessTopics: []string{"topic-1"}}
	backend := &mockBackend{dim: 3}
	db := &mockDBTX{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			// ChunkIDsByTopics query joins chunks and documents.
			if strings.Contains(sql, "chunks") {
				return nil, errors.New("db query failed")
			}
			return &mockRows{}, nil
		},
	}
	cs := store.NewChunkStore(db)
	s := NewSearcher(cs, authz, backend)

	_, err := s.Search(context.Background(), &auth.Principal{ID: "user:test"}, nil, []float64{1, 2, 3}, 10, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fetching chunk IDs") {
		t.Errorf("expected 'fetching chunk IDs' in error, got: %v", err)
	}
}

func TestSearcher_BackendSearchError(t *testing.T) {
	authz := &mockAuthorizer{accessTopics: []string{"topic-1"}}
	backend := &mockBackend{dim: 3, err: errors.New("vector search failed")}

	// ChunkIDsByTopics must return some IDs so we reach backend.Search.
	db := &mockDBTX{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "chunks") && strings.Contains(sql, "documents") {
				return &mockRows{
					data: [][]any{{"chunk-1"}},
				}, nil
			}
			return &mockRows{}, nil
		},
	}
	cs := store.NewChunkStore(db)
	s := NewSearcher(cs, authz, backend)

	_, err := s.Search(context.Background(), &auth.Principal{ID: "user:test"}, nil, []float64{1, 2, 3}, 10, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "searching vector backend") {
		t.Errorf("expected 'searching vector backend' in error, got: %v", err)
	}
}

func TestSearcher_GetMultipleError(t *testing.T) {
	authz := &mockAuthorizer{accessTopics: []string{"topic-1"}}
	backend := &mockBackend{
		dim: 3,
		results: []vector.SearchResult{
			{ChunkID: "chunk-1", Score: 0.9},
		},
	}

	callCount := 0
	db := &mockDBTX{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			callCount++
			// First query: ChunkIDsByTopics (joins chunks and documents).
			if strings.Contains(sql, "JOIN documents") {
				return &mockRows{
					data: [][]any{{"chunk-1"}},
				}, nil
			}
			// Second query: GetMultiple (SELECT FROM chunks WHERE id = ANY).
			if strings.Contains(sql, "FROM chunks") {
				return nil, errors.New("get multiple failed")
			}
			return &mockRows{}, nil
		},
	}
	cs := store.NewChunkStore(db)
	s := NewSearcher(cs, authz, backend)

	_, err := s.Search(context.Background(), &auth.Principal{ID: "user:test"}, nil, []float64{1, 2, 3}, 10, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fetching chunks") {
		t.Errorf("expected 'fetching chunks' in error, got: %v", err)
	}
}

func TestSearcher_DocumentTopicIDsError(t *testing.T) {
	authz := &mockAuthorizer{accessTopics: []string{"topic-1"}}
	backend := &mockBackend{
		dim: 3,
		results: []vector.SearchResult{
			{ChunkID: "chunk-1", Score: 0.9},
		},
	}

	db := &mockDBTX{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			// ChunkIDsByTopics: JOIN documents.
			if strings.Contains(sql, "JOIN documents") {
				return &mockRows{
					data: [][]any{{"chunk-1"}},
				}, nil
			}
			// GetMultiple: SELECT ... FROM chunks WHERE id = ANY.
			// Need 9 columns: id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
			if strings.Contains(sql, "FROM chunks") {
				return &mockRowsGetMultiple{
					chunks: []getMultipleRow{
						{id: "chunk-1", docID: "doc-1"},
					},
				}, nil
			}
			// DocumentTopicIDs: SELECT ... FROM documents WHERE id = ANY.
			if strings.Contains(sql, "FROM documents") {
				return nil, errors.New("doc topics query failed")
			}
			return &mockRows{}, nil
		},
	}
	cs := store.NewChunkStore(db)
	s := NewSearcher(cs, authz, backend)

	_, err := s.Search(context.Background(), &auth.Principal{ID: "user:test"}, nil, []float64{1, 2, 3}, 10, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fetching document topics") {
		t.Errorf("expected 'fetching document topics' in error, got: %v", err)
	}
}

func TestSearcher_ChunkNotFoundSkipped(t *testing.T) {
	authz := &mockAuthorizer{accessTopics: []string{"topic-1"}}
	backend := &mockBackend{
		dim: 3,
		results: []vector.SearchResult{
			{ChunkID: "chunk-missing", Score: 0.9},
			{ChunkID: "chunk-1", Score: 0.8},
		},
	}

	db := &mockDBTX{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "JOIN documents") {
				return &mockRows{
					data: [][]any{{"chunk-missing"}, {"chunk-1"}},
				}, nil
			}
			// GetMultiple returns only chunk-1 (chunk-missing not found).
			if strings.Contains(sql, "FROM chunks") {
				return &mockRowsGetMultiple{
					chunks: []getMultipleRow{
						{id: "chunk-1", docID: "doc-1"},
					},
				}, nil
			}
			// DocumentTopicIDs.
			if strings.Contains(sql, "FROM documents") {
				return &mockRowsDocTopics{
					docs: []docTopicRow{
						{docID: "doc-1", topicID: "topic-1"},
					},
				}, nil
			}
			return &mockRows{}, nil
		},
	}
	cs := store.NewChunkStore(db)
	s := NewSearcher(cs, authz, backend)

	results, err := s.Search(context.Background(), &auth.Principal{ID: "user:test"}, nil, []float64{1, 2, 3}, 10, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// chunk-missing should be skipped; only chunk-1 should appear.
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Chunk.ID != "chunk-1" {
		t.Errorf("expected chunk-1, got %s", results[0].Chunk.ID)
	}
}

func TestSearcher_DocTopicNotFoundSkipped(t *testing.T) {
	authz := &mockAuthorizer{accessTopics: []string{"topic-1"}}
	backend := &mockBackend{
		dim: 3,
		results: []vector.SearchResult{
			{ChunkID: "chunk-1", Score: 0.9},
			{ChunkID: "chunk-2", Score: 0.8},
		},
	}

	db := &mockDBTX{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "JOIN documents") {
				return &mockRows{
					data: [][]any{{"chunk-1"}, {"chunk-2"}},
				}, nil
			}
			if strings.Contains(sql, "FROM chunks") {
				return &mockRowsGetMultiple{
					chunks: []getMultipleRow{
						{id: "chunk-1", docID: "doc-1"},
						{id: "chunk-2", docID: "doc-orphan"},
					},
				}, nil
			}
			// DocumentTopicIDs returns only doc-1; doc-orphan is missing.
			if strings.Contains(sql, "FROM documents") {
				return &mockRowsDocTopics{
					docs: []docTopicRow{
						{docID: "doc-1", topicID: "topic-1"},
					},
				}, nil
			}
			return &mockRows{}, nil
		},
	}
	cs := store.NewChunkStore(db)
	s := NewSearcher(cs, authz, backend)

	results, err := s.Search(context.Background(), &auth.Principal{ID: "user:test"}, nil, []float64{1, 2, 3}, 10, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// chunk-2's document has no topic mapping; it should be skipped.
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Chunk.ID != "chunk-1" {
		t.Errorf("expected chunk-1, got %s", results[0].Chunk.ID)
	}
}

func TestSearcher_EmptyChunkIDs(t *testing.T) {
	authz := &mockAuthorizer{accessTopics: []string{"topic-1"}}
	backend := &mockBackend{dim: 3}
	// ChunkIDsByTopics returns zero rows (no chunks in accessible topics).
	db := &mockDBTX{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			return &mockRows{}, nil // empty rows
		},
	}
	cs := store.NewChunkStore(db)
	s := NewSearcher(cs, authz, backend)

	results, err := s.Search(context.Background(), &auth.Principal{ID: "user:test"}, nil, []float64{1, 2, 3}, 10, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

// mockRowsGetMultiple provides rows that satisfy the GetMultiple scan pattern
// (id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at).
type getMultipleRow struct {
	id    string
	docID string
}

type mockRowsGetMultiple struct {
	chunks []getMultipleRow
	idx    int
	closed bool
}

func (m *mockRowsGetMultiple) Close()                                       { m.closed = true }
func (m *mockRowsGetMultiple) Err() error                                   { return nil }
func (m *mockRowsGetMultiple) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockRowsGetMultiple) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRowsGetMultiple) RawValues() [][]byte                          { return nil }
func (m *mockRowsGetMultiple) Conn() *pgx.Conn                              { return nil }
func (m *mockRowsGetMultiple) Values() ([]any, error)                       { return nil, nil }

func (m *mockRowsGetMultiple) Next() bool {
	if m.idx < len(m.chunks) {
		m.idx++
		return true
	}
	return false
}

func (m *mockRowsGetMultiple) Scan(dest ...any) error {
	row := m.chunks[m.idx-1]
	// GetMultiple scans: id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
	if len(dest) != 9 {
		return errors.New("expected 9 scan destinations")
	}
	*dest[0].(*string) = row.id
	*dest[1].(*string) = row.docID
	*dest[2].(*int) = 0
	*dest[3].(*string) = "test content"
	*dest[4].(**string) = nil
	*dest[5].(*string) = "active"
	*dest[6].(**string) = nil
	*dest[7].(*[]byte) = []byte("{}")
	// created_at is a time.Time; we can set it to zero value.
	return nil
}

// mockRowsDocTopics provides rows for DocumentTopicIDs (id, topic_id).
type docTopicRow struct {
	docID   string
	topicID string
}

type mockRowsDocTopics struct {
	docs   []docTopicRow
	idx    int
	closed bool
}

func (m *mockRowsDocTopics) Close()                                       { m.closed = true }
func (m *mockRowsDocTopics) Err() error                                   { return nil }
func (m *mockRowsDocTopics) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockRowsDocTopics) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRowsDocTopics) RawValues() [][]byte                          { return nil }
func (m *mockRowsDocTopics) Conn() *pgx.Conn                              { return nil }
func (m *mockRowsDocTopics) Values() ([]any, error)                       { return nil, nil }

func (m *mockRowsDocTopics) Next() bool {
	if m.idx < len(m.docs) {
		m.idx++
		return true
	}
	return false
}

func (m *mockRowsDocTopics) Scan(dest ...any) error {
	row := m.docs[m.idx-1]
	*dest[0].(*string) = row.docID
	*dest[1].(*string) = row.topicID
	return nil
}
