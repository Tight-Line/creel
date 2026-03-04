// Package vectortest provides a reusable conformance test suite for vector backends.
package vectortest

import (
	"context"
	"testing"

	"github.com/Tight-Line/creel/internal/vector"
)

// RunConformanceTests runs the standard vector backend conformance test suite.
// Any Backend implementation must pass all these tests.
func RunConformanceTests(t *testing.T, backend vector.Backend) {
	t.Helper()
	ctx := context.Background()

	// Helper to create a simple embedding of length 1536.
	makeVec := func(val float64) []float64 {
		v := make([]float64, 1536)
		v[0] = val
		return v
	}

	t.Run("Ping", func(t *testing.T) {
		if err := backend.Ping(ctx); err != nil {
			t.Fatalf("Ping: %v", err)
		}
	})

	t.Run("StoreAndSearch", func(t *testing.T) {
		if err := backend.Store(ctx, "chunk-1", makeVec(1.0), map[string]any{"source": "test"}); err != nil {
			t.Fatalf("Store: %v", err)
		}
		if err := backend.Store(ctx, "chunk-2", makeVec(0.9), nil); err != nil {
			t.Fatalf("Store: %v", err)
		}
		if err := backend.Store(ctx, "chunk-3", makeVec(0.1), nil); err != nil {
			t.Fatalf("Store: %v", err)
		}

		results, err := backend.Search(ctx, makeVec(1.0), vector.Filter{}, 2)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].ChunkID != "chunk-1" {
			t.Errorf("expected chunk-1 first, got %s", results[0].ChunkID)
		}
	})

	t.Run("SearchWithChunkIDFilter", func(t *testing.T) {
		results, err := backend.Search(ctx, makeVec(1.0), vector.Filter{ChunkIDs: []string{"chunk-2", "chunk-3"}}, 10)
		if err != nil {
			t.Fatalf("Search with filter: %v", err)
		}
		for _, r := range results {
			if r.ChunkID == "chunk-1" {
				t.Error("chunk-1 should be excluded by filter")
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := backend.Delete(ctx, "chunk-3"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		results, err := backend.Search(ctx, makeVec(0.1), vector.Filter{}, 10)
		if err != nil {
			t.Fatalf("Search after delete: %v", err)
		}
		for _, r := range results {
			if r.ChunkID == "chunk-3" {
				t.Error("chunk-3 should be deleted")
			}
		}
	})

	t.Run("StoreBatch", func(t *testing.T) {
		items := []vector.StoreItem{
			{ID: "batch-1", Embedding: makeVec(0.5), Metadata: map[string]any{"batch": true}},
			{ID: "batch-2", Embedding: makeVec(0.6), Metadata: nil},
		}
		if err := backend.StoreBatch(ctx, items); err != nil {
			t.Fatalf("StoreBatch: %v", err)
		}
	})

	t.Run("DeleteBatch", func(t *testing.T) {
		if err := backend.DeleteBatch(ctx, []string{"batch-1", "batch-2"}); err != nil {
			t.Fatalf("DeleteBatch: %v", err)
		}
	})

	// Cleanup.
	_ = backend.DeleteBatch(ctx, []string{"chunk-1", "chunk-2"})
}
