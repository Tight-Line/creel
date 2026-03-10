package vector

import (
	"context"
	"fmt"
	"testing"
)

// stubBackend is a minimal Backend implementation for registry tests.
type stubBackend struct {
	dim int
}

func (s *stubBackend) EmbeddingDimension() int { return s.dim }
func (s *stubBackend) Store(_ context.Context, _ string, _ []float64, _ map[string]any) error {
	return nil
}
func (s *stubBackend) Delete(_ context.Context, _ string) error { return nil }
func (s *stubBackend) Search(_ context.Context, _ []float64, _ Filter, _ int) ([]SearchResult, error) {
	return nil, nil
}
func (s *stubBackend) StoreBatch(_ context.Context, _ []StoreItem) error { return nil }
func (s *stubBackend) DeleteBatch(_ context.Context, _ []string) error   { return nil }
func (s *stubBackend) Ping(_ context.Context) error                      { return nil }

func TestRegistry_FallbackWhenNilConfigID(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	b, err := r.Get(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != fb {
		t.Error("expected fallback backend")
	}
}

func TestRegistry_FallbackWhenEmptyConfigID(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	empty := ""
	b, err := r.Get(&empty)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != fb {
		t.Error("expected fallback backend")
	}
}

func TestRegistry_PutAndGet(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	custom := &stubBackend{dim: 256}
	r.Put("cfg-1", custom)

	id := "cfg-1"
	b, err := r.Get(&id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != custom {
		t.Error("expected custom backend")
	}
	if b.EmbeddingDimension() != 256 {
		t.Errorf("expected dim 256, got %d", b.EmbeddingDimension())
	}
}

func TestRegistry_GetMissingReturnsError(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	id := "nonexistent"
	_, err := r.Get(&id)
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestRegistry_GetOrCreate(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	r.RegisterFactory("test", func(config map[string]any) (Backend, error) {
		dim := 512
		if v, ok := config["dim"]; ok {
			if d, ok := v.(int); ok {
				dim = d
			}
		}
		return &stubBackend{dim: dim}, nil
	})

	b, err := r.GetOrCreate("cfg-2", "test", map[string]any{"dim": 512})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.EmbeddingDimension() != 512 {
		t.Errorf("expected dim 512, got %d", b.EmbeddingDimension())
	}

	// Second call should return cached backend.
	b2, err := r.GetOrCreate("cfg-2", "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != b2 {
		t.Error("expected same cached backend instance")
	}
}

func TestRegistry_GetOrCreate_EmptyConfigID(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	b, err := r.GetOrCreate("", "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != fb {
		t.Error("expected fallback backend")
	}
}

func TestRegistry_GetOrCreate_NoFactory(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	_, err := r.GetOrCreate("cfg-3", "unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown backend type")
	}
}

func TestRegistry_GetOrCreate_FactoryError(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	r.RegisterFactory("failing", func(_ map[string]any) (Backend, error) {
		return nil, fmt.Errorf("connection refused")
	})

	_, err := r.GetOrCreate("cfg-4", "failing", nil)
	if err == nil {
		t.Fatal("expected error from factory")
	}
}

func TestRegistry_Remove(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	r.Put("cfg-5", &stubBackend{dim: 64})
	r.Remove("cfg-5")

	id := "cfg-5"
	_, err := r.Get(&id)
	if err == nil {
		t.Fatal("expected error after removal")
	}
}

func TestRegistry_Fallback(t *testing.T) {
	fb := &stubBackend{dim: 128}
	r := NewRegistry(fb)

	if r.Fallback() != fb {
		t.Error("expected fallback backend")
	}
}
