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
)

func TestMemoryMaintenanceWorker_Type(t *testing.T) {
	w := NewMemoryMaintenanceWorker(nil, nil, nil)
	if w.Type() != "memory_maintenance" {
		t.Errorf("expected 'memory_maintenance', got %q", w.Type())
	}
}

func TestMemoryMaintenanceWorker_MissingProgress(t *testing.T) {
	w := NewMemoryMaintenanceWorker(nil, nil, nil)
	job := &store.ProcessingJob{Progress: map[string]any{}}
	err := w.Process(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "missing candidate_fact or principal") {
		t.Fatalf("expected missing progress error, got: %v", err)
	}
}

// mockMemoryDBTX supports memory store operations for testing.
type mockMemoryDBTX struct {
	queryErr  error
	queryRows []*store.Memory
	execErr   error
	createMem *store.Memory
}

func (m *mockMemoryDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockMemoryDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return &mockMemoryQueryRows{memories: m.queryRows}, nil
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

// mockMemoryQueryRows returns full memory rows for GetByScope.
type mockMemoryQueryRows struct {
	memories []*store.Memory
	idx      int
}

func (r *mockMemoryQueryRows) Close()                                       {}
func (r *mockMemoryQueryRows) Err() error                                   { return nil }
func (r *mockMemoryQueryRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockMemoryQueryRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockMemoryQueryRows) RawValues() [][]byte                          { return nil }
func (r *mockMemoryQueryRows) Conn() *pgx.Conn                              { return nil }
func (r *mockMemoryQueryRows) Values() ([]any, error)                       { return nil, nil }

func (r *mockMemoryQueryRows) Next() bool {
	if r.memories != nil && r.idx < len(r.memories) {
		r.idx++
		return true
	}
	return false
}

func (r *mockMemoryQueryRows) Scan(dest ...any) error {
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
	return &mockMemoryQueryRows{}, nil
}

func (m *mockMemoryDBTXWithFailCreate) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{err: errors.New("insert failed")}
}

func (m *mockMemoryDBTXWithFailCreate) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}

// mockMemoryDBTXUpdateFail succeeds on Get (QueryRow SELECT) but fails on Update (QueryRow UPDATE).
type mockMemoryDBTXUpdateFail struct {
	callCount int
}

func (m *mockMemoryDBTXUpdateFail) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockMemoryDBTXUpdateFail) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &mockMemoryQueryRows{}, nil
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

func TestMemoryMaintenanceWorker_GetByScopeError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryErr: errors.New("query failed")}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
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
	if err == nil || !strings.Contains(err.Error(), "fetching existing memories") {
		t.Fatalf("expected fetch error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_LLMError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
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
	combinedDB := &mockMemoryDBTXWithFailCreate{queryErr: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(combinedDB),
		nil,
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
	if err == nil || !strings.Contains(err.Error(), "creating memory") {
		t.Fatalf("expected create error, got: %v", err)
	}
}

func TestMemoryMaintenanceWorker_UPDATE_Success(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
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

func TestMemoryMaintenanceWorker_UPDATE_GetMemoryError(t *testing.T) {
	failGetDB := &mockMemoryDBTXWithFailCreate{queryErr: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(failGetDB),
		nil,
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

func TestMemoryMaintenanceWorker_UPDATE_UpdateError(t *testing.T) {
	updateFailDB := &mockMemoryDBTXUpdateFail{}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(updateFailDB),
		nil,
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

func TestMemoryMaintenanceWorker_DELETE_Success(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
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

func TestMemoryMaintenanceWorker_DELETE_InvalidateError(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil, execErr: errors.New("invalidate failed")}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
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

func TestMemoryMaintenanceWorker_DefaultScope(t *testing.T) {
	memDB := &mockMemoryDBTX{queryRows: nil}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
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
	memDB := &mockMemoryDBTX{
		queryRows: []*store.Memory{
			{ID: "mem-1", Principal: "user1", Scope: "test", Content: "user likes cats", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	w := NewMemoryMaintenanceWorker(
		store.NewMemoryStore(memDB),
		nil,
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
