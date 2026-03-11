package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Tight-Line/creel/internal/llm"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector"
)

func TestMemoryMaintenanceWorker_Type(t *testing.T) {
	w := NewMemoryMaintenanceWorker(nil, nil, nil, nil, nil)
	if w.Type() != "memory_maintenance" {
		t.Errorf("expected 'memory_maintenance', got %q", w.Type())
	}
}

func TestMemoryMaintenanceWorker_MissingProgress(t *testing.T) {
	w := NewMemoryMaintenanceWorker(nil, nil, nil, nil, nil)
	job := &store.ProcessingJob{Progress: map[string]any{}}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "missing candidate_fact or principal") {
		t.Fatalf("expected missing progress error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_EmbedError(t *testing.T) {
	w := NewMemoryMaintenanceWorker(
		nil, nil, nil,
		&failingEmbeddingProvider{dim: 4},
		nil,
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "embedding candidate fact") {
		t.Fatalf("expected embedding error, got: %v", err)
	}
}

// mockMemoryDBTX supports memory store operations for testing.
// It routes queries based on the SQL content.
type mockMemoryDBTX struct {
	queryErr     error
	embeddingIDs []string
	queryRows    []*store.Memory
	execErr      error
	createMem    *store.Memory
}

func (m *mockMemoryDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockMemoryDBTX) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	// EmbeddingIDsByPrincipalScope returns embedding_id strings.
	if strings.Contains(sql, "SELECT embedding_id FROM memories") {
		return &mockEmbeddingIDRows{ids: m.embeddingIDs}, nil
	}
	// GetByEmbeddingIDs returns full memory rows.
	if strings.Contains(sql, "embedding_id = ANY") {
		return &mockFullMemoryRows{memories: m.queryRows}, nil
	}
	return &mockEmbeddingIDRows{}, nil
}

func (m *mockMemoryDBTX) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if strings.Contains(sql, "INSERT") {
		if m.createMem != nil {
			return &mockMemoryCreateRow{mem: m.createMem}
		}
		return &mockMemoryCreateRow{mem: &store.Memory{ID: "mem-1", Principal: "user1", Scope: "test", Content: "fact", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	}
	if strings.Contains(sql, "UPDATE") && strings.Contains(sql, "content") {
		return &mockMemoryCreateRow{mem: &store.Memory{ID: "mem-1", Principal: "user1", Scope: "test", Content: "updated", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	}
	// Get by ID
	return &mockMemoryCreateRow{mem: &store.Memory{ID: "mem-1", Principal: "user1", Scope: "test", Content: "old fact", Status: "active", Metadata: map[string]any{}, CreatedAt: time.Now(), UpdatedAt: time.Now()}}
}

func (m *mockMemoryDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

// mockEmbeddingIDRows returns embedding_id strings.
type mockEmbeddingIDRows struct {
	ids []string
	idx int
}

func (r *mockEmbeddingIDRows) Close()                                       {}
func (r *mockEmbeddingIDRows) Err() error                                   { return nil }
func (r *mockEmbeddingIDRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockEmbeddingIDRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockEmbeddingIDRows) RawValues() [][]byte                          { return nil }
func (r *mockEmbeddingIDRows) Conn() *pgx.Conn                              { return nil }
func (r *mockEmbeddingIDRows) Values() ([]any, error)                       { return nil, nil }

func (r *mockEmbeddingIDRows) Next() bool {
	if r.ids != nil && r.idx < len(r.ids) {
		r.idx++
		return true
	}
	return false
}

func (r *mockEmbeddingIDRows) Scan(dest ...any) error {
	*dest[0].(*string) = r.ids[r.idx-1]
	return nil
}

// mockFullMemoryRows returns full memory rows for GetByEmbeddingIDs.
type mockFullMemoryRows struct {
	memories []*store.Memory
	idx      int
}

func (r *mockFullMemoryRows) Close()                                       {}
func (r *mockFullMemoryRows) Err() error                                   { return nil }
func (r *mockFullMemoryRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockFullMemoryRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockFullMemoryRows) RawValues() [][]byte                          { return nil }
func (r *mockFullMemoryRows) Conn() *pgx.Conn                              { return nil }
func (r *mockFullMemoryRows) Values() ([]any, error)                       { return nil, nil }

func (r *mockFullMemoryRows) Next() bool {
	if r.memories != nil && r.idx < len(r.memories) {
		r.idx++
		return true
	}
	return false
}

func (r *mockFullMemoryRows) Scan(dest ...any) error {
	m := r.memories[r.idx-1]
	*dest[0].(*string) = m.ID
	*dest[1].(*string) = m.Principal
	*dest[2].(*string) = m.Scope
	*dest[3].(*string) = m.Content
	*dest[4].(**string) = m.EmbeddingID
	*dest[5].(**string) = m.Subject
	*dest[6].(**string) = m.Predicate
	*dest[7].(**string) = m.Object
	*dest[8].(**string) = m.SourceChunkID
	*dest[9].(*string) = m.Status
	*dest[10].(**time.Time) = m.InvalidatedAt
	*dest[11].(*[]byte) = []byte("{}")
	*dest[12].(*time.Time) = m.CreatedAt
	*dest[13].(*time.Time) = m.UpdatedAt
	return nil
}

type mockMemoryCreateRow struct {
	mem *store.Memory
}

func (r *mockMemoryCreateRow) Scan(dest ...any) error {
	if len(dest) >= 14 {
		*dest[0].(*string) = r.mem.ID
		*dest[1].(*string) = r.mem.Principal
		*dest[2].(*string) = r.mem.Scope
		*dest[3].(*string) = r.mem.Content
		*dest[4].(**string) = r.mem.EmbeddingID
		*dest[5].(**string) = r.mem.Subject
		*dest[6].(**string) = r.mem.Predicate
		*dest[7].(**string) = r.mem.Object
		*dest[8].(**string) = r.mem.SourceChunkID
		*dest[9].(*string) = r.mem.Status
		*dest[10].(**time.Time) = r.mem.InvalidatedAt
		*dest[11].(*[]byte) = []byte("{}")
		*dest[12].(*time.Time) = r.mem.CreatedAt
		*dest[13].(*time.Time) = r.mem.UpdatedAt
	}
	return nil
}

// mockVectorBackendWithSearch extends mockVectorBackend with search results.
type mockVectorBackendWithSearch struct {
	storeErr      error
	searchResults []vector.SearchResult
	searchErr     error
	dim           int
}

func (m *mockVectorBackendWithSearch) EmbeddingDimension() int { return m.dim }
func (m *mockVectorBackendWithSearch) Store(_ context.Context, _ string, _ []float64, _ map[string]any) error {
	return m.storeErr
}
func (m *mockVectorBackendWithSearch) Delete(_ context.Context, _ string) error { return nil }
func (m *mockVectorBackendWithSearch) Search(_ context.Context, _ []float64, _ vector.Filter, _ int) ([]vector.SearchResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResults, nil
}
func (m *mockVectorBackendWithSearch) StoreBatch(_ context.Context, _ []vector.StoreItem) error {
	return m.storeErr
}
func (m *mockVectorBackendWithSearch) DeleteBatch(_ context.Context, _ []string) error { return nil }
func (m *mockVectorBackendWithSearch) Ping(_ context.Context) error                    { return nil }

func TestMemoryMaintenanceWorker_EmbeddingIDsQueryError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryErr: errors.New("query failed")}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		nil,
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "fetching memory embedding IDs") {
		t.Fatalf("expected embedding IDs error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_SearchError(t *testing.T) {
	embID := "emb-1"
	mem := &store.Memory{ID: "mem-1", EmbeddingID: &embID, Principal: "user1", Scope: "test", Content: "old", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	memDB := &mockMemoryDBTX{embeddingIDs: []string{"emb-1"}, queryRows: []*store.Memory{mem}}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4, searchErr: errors.New("search failed")},
		NewStubEmbeddingProvider(4),
		nil,
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "searching existing memories") {
		t.Fatalf("expected search error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_LLMError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil} // No existing memories
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		&failingLLMProvider{},
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "calling LLM for maintenance") {
		t.Fatalf("expected LLM error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_BadJSON(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider("not json"),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "parsing maintenance response") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_NOOP(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "NOOP", "memory_id": "", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMaintenanceWorker_UnknownAction(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "INVALID", "memory_id": "", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "unknown maintenance action") {
		t.Fatalf("expected unknown action error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_ADD_Success(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "ADD", "memory_id": "", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMaintenanceWorker_ADD_CreateError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	failMemDB := &mockMemoryDBTX{queryRows: nil}
	// Use a separate store for Create that fails.
	failCreateDB := &mockDBTX{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "INSERT") {
				return &mockRow{err: errors.New("insert failed")}
			}
			return &mockRow{err: pgx.ErrNoRows}
		},
	}
	// Need a memStore that returns empty embedding IDs but fails on Create.
	// The simplest approach: have queryRows be nil (for EmbeddingIDsByPrincipalScope)
	// but the QueryRow for INSERT fail.
	_ = failMemDB
	_ = memDB
	combinedDB := &mockMemoryDBTXWithFailCreate{queryErr: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(combinedDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "ADD", "memory_id": "", "merged_content": ""}`),
	)
	_ = failCreateDB
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "creating memory") {
		t.Fatalf("expected create error, got: %v", err)
	}
}

// mockMemoryDBTXWithFailCreate fails on QueryRow (INSERT) but succeeds on Query.
type mockMemoryDBTXWithFailCreate struct {
	queryErr error
}

func (m *mockMemoryDBTXWithFailCreate) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockMemoryDBTXWithFailCreate) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	// Return empty rows for embedding IDs.
	return &mockEmbeddingIDRows{}, nil
}

func (m *mockMemoryDBTXWithFailCreate) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{err: errors.New("insert failed")}
}

func (m *mockMemoryDBTXWithFailCreate) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

func TestMemoryMaintenanceWorker_ADD_VectorStoreError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4, storeErr: errors.New("vector store failed")},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "ADD", "memory_id": "", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "storing memory embedding") {
		t.Fatalf("expected vector store error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_UPDATE_Success(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "UPDATE", "memory_id": "mem-1", "merged_content": "user loves cats and dogs"}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user loves cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMaintenanceWorker_UPDATE_MissingFields(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "UPDATE", "memory_id": "", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user loves cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "UPDATE action requires memory_id and merged_content") {
		t.Fatalf("expected missing fields error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_DELETE_Success(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "DELETE", "memory_id": "mem-1", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user no longer likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMaintenanceWorker_DELETE_MissingID(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "DELETE", "memory_id": "", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user no longer likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "DELETE action requires memory_id") {
		t.Fatalf("expected missing ID error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_DefaultScope(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "NOOP", "memory_id": "", "merged_content": ""}`),
	)
	// No scope in progress; should default to "default".
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
		},
	}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMaintenanceWorker_WithExistingMemories(t *testing.T) {
	embID := "emb-1"
	mem := &store.Memory{ID: "mem-1", EmbeddingID: &embID, Principal: "user1", Scope: "test", Content: "user likes cats", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	memDB := &mockMemoryDBTX{embeddingIDs: []string{"emb-1"}, queryRows: []*store.Memory{mem}}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{
			dim:           4,
			searchResults: []vector.SearchResult{{ChunkID: "emb-1", Score: 0.95}},
		},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "NOOP", "memory_id": "", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryMaintenanceWorker_FetchMemoriesByEmbIDError(t *testing.T) {
	// Use a DB that succeeds on first query (EmbeddingIDsByPrincipalScope) but fails on second (GetByEmbeddingIDs).
	callCount := 0
	memDB := &mockMemoryDBTXWithCallCount{
		embeddingIDs: []string{"emb-1"},
		callCount:    &callCount,
		failOnCall:   2,
	}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{
			dim:           4,
			searchResults: []vector.SearchResult{{ChunkID: "emb-1", Score: 0.95}},
		},
		NewStubEmbeddingProvider(4),
		nil,
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "fetching memories by embedding IDs") {
		t.Fatalf("expected fetch error, got: %v", err)
	}
}

// mockMemoryDBTXWithCallCount tracks Query calls and fails on a specific one.
type mockMemoryDBTXWithCallCount struct {
	embeddingIDs []string
	callCount    *int
	failOnCall   int
}

func (m *mockMemoryDBTXWithCallCount) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockMemoryDBTXWithCallCount) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	*m.callCount++
	if *m.callCount == m.failOnCall {
		return nil, errors.New("query failed")
	}
	// First call: EmbeddingIDsByPrincipalScope.
	return &mockEmbeddingIDRows{ids: m.embeddingIDs}, nil
}

func (m *mockMemoryDBTXWithCallCount) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{err: errors.New("not configured")}
}

func (m *mockMemoryDBTXWithCallCount) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

func TestMemoryMaintenanceWorker_ADD_SetEmbeddingIDError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil, execErr: errors.New("exec failed")}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "ADD", "memory_id": "", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "setting memory embedding ID") {
		t.Fatalf("expected set embedding ID error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_UPDATE_GetMemoryError(t *testing.T) {
	failGetDB := &mockMemoryDBTXWithFailCreate{queryErr: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(failGetDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "UPDATE", "memory_id": "mem-1", "merged_content": "new content"}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "getting memory for update") {
		t.Fatalf("expected get memory error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_DELETE_InvalidateError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil, execErr: errors.New("invalidate failed")}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "DELETE", "memory_id": "mem-1", "merged_content": ""}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "outdated fact",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "invalidating memory") {
		t.Fatalf("expected invalidate error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_UPDATE_UpdateError(t *testing.T) {
	// Simulate: Get succeeds but Update fails.
	updateFailDB := &mockMemoryDBTXUpdateFail{}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(updateFailDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "UPDATE", "memory_id": "mem-1", "merged_content": "new"}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "updating memory") {
		t.Fatalf("expected update error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_UPDATE_EmbedError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	// First embed succeeds, second fails (for re-embedding after UPDATE).
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		&secondCallFailProvider{dim: 4},
		llm.NewStubProvider(`{"action": "UPDATE", "memory_id": "mem-1", "merged_content": "new"}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "embedding updated memory") {
		t.Fatalf("expected embed error, got: %v", err)
	}
}

// secondCallFailProvider succeeds on first Embed call, fails on second.
type secondCallFailProvider struct {
	dim       int
	callCount int
}

func (p *secondCallFailProvider) Embed(_ context.Context, texts []string) ([][]float64, error) {
	p.callCount++
	if p.callCount > 1 {
		return nil, errors.New("embed failed")
	}
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = make([]float64, p.dim)
	}
	return result, nil
}

func (p *secondCallFailProvider) Dimensions() int { return p.dim }
func (p *secondCallFailProvider) Model() string    { return "" }

func TestMemoryMaintenanceWorker_UPDATE_SetEmbeddingIDError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil, execErr: errors.New("exec failed")}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "UPDATE", "memory_id": "mem-1", "merged_content": "new"}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "setting updated memory embedding ID") {
		t.Fatalf("expected set embedding ID error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_EmptyEmbedding(t *testing.T) {
	w := NewMemoryMaintenanceWorker(
		nil, nil, nil,
		&emptyEmbeddingProvider{},
		nil,
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "no embedding returned for candidate fact") {
		t.Fatalf("expected empty embedding error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_UPDATE_EmptyReEmbed(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	// First embed succeeds, second returns empty (for re-embedding after UPDATE).
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4},
		&secondCallEmptyProvider{dim: 4},
		llm.NewStubProvider(`{"action": "UPDATE", "memory_id": "mem-1", "merged_content": "new"}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "no embedding returned for updated memory") {
		t.Fatalf("expected empty re-embed error, got: %v", err)
	}
}

// secondCallEmptyProvider succeeds on first Embed call, returns empty on second.
type secondCallEmptyProvider struct {
	dim       int
	callCount int
}

func (p *secondCallEmptyProvider) Embed(_ context.Context, texts []string) ([][]float64, error) {
	p.callCount++
	if p.callCount > 1 {
		return nil, nil
	}
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = make([]float64, p.dim)
	}
	return result, nil
}

func (p *secondCallEmptyProvider) Dimensions() int { return p.dim }
func (p *secondCallEmptyProvider) Model() string    { return "" }

// emptyEmbeddingProvider returns empty embeddings.
type emptyEmbeddingProvider struct{}

func (p *emptyEmbeddingProvider) Embed(_ context.Context, _ []string) ([][]float64, error) {
	return nil, nil
}

func (p *emptyEmbeddingProvider) Dimensions() int { return 4 }
func (p *emptyEmbeddingProvider) Model() string    { return "" }

// mockMemoryDBTXUpdateFail succeeds on Get (QueryRow SELECT) but fails on Update (QueryRow UPDATE).
type mockMemoryDBTXUpdateFail struct {
	callCount int
}

func (m *mockMemoryDBTXUpdateFail) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockMemoryDBTXUpdateFail) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &mockEmbeddingIDRows{}, nil
}

func (m *mockMemoryDBTXUpdateFail) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	m.callCount++
	if strings.Contains(sql, "UPDATE") {
		return &mockRow{err: errors.New("update failed")}
	}
	// Get by ID
	return &mockMemoryCreateRow{mem: &store.Memory{ID: "mem-1", Principal: "user1", Scope: "test", Content: "old", Status: "active", Metadata: map[string]any{}, CreatedAt: time.Now(), UpdatedAt: time.Now()}}
}

func (m *mockMemoryDBTXUpdateFail) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

func TestMemoryMaintenanceWorker_UPDATE_VectorStoreError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
		&mockVectorBackendWithSearch{dim: 4, storeErr: errors.New("vector failed")},
		NewStubEmbeddingProvider(4),
		llm.NewStubProvider(`{"action": "UPDATE", "memory_id": "mem-1", "merged_content": "updated content"}`),
	)
	job := &store.ProcessingJob{
		Progress: map[string]any{
			"candidate_fact": "user likes cats",
			"principal":      "user1",
			"scope":          "test",
		},
	}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "storing updated memory embedding") {
		t.Fatalf("expected vector store error, got: %v", err)
	}
}
