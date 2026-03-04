// Package vectortest provides a reusable conformance test suite for vector backends.
package vectortest

import (
	"context"
	"testing"

	"github.com/Tight-Line/creel/internal/vector"
)

// TestIDs are deterministic UUIDs used by the conformance suite.
// Backends with foreign key constraints (e.g. pgvector) must ensure
// parent rows exist for these IDs before calling RunConformanceTests.
var TestIDs = struct {
	Chunk1, Chunk2, Chunk3 string
	Batch1, Batch2         string
}{
	Chunk1: "00000000-0000-0000-0000-000000000001",
	Chunk2: "00000000-0000-0000-0000-000000000002",
	Chunk3: "00000000-0000-0000-0000-000000000003",
	Batch1: "00000000-0000-0000-0000-0000000000b1",
	Batch2: "00000000-0000-0000-0000-0000000000b2",
}

// RunConformanceTests runs the standard vector backend conformance test suite.
// Any Backend implementation must pass all these tests.
func RunConformanceTests(t *testing.T, backend vector.Backend) {
	t.Helper()
	ctx := context.Background()
	ids := TestIDs

	// Helper to create an embedding matching the backend's dimension.
	makeVec := func(val float64) []float64 {
		v := make([]float64, backend.EmbeddingDimension())
		v[0] = val
		return v
	}

	// Clean up any stale data from previous failed runs.
	_ = backend.DeleteBatch(ctx, []string{ids.Chunk1, ids.Chunk2, ids.Chunk3, ids.Batch1, ids.Batch2})

	t.Run("Ping", func(t *testing.T) {
		if err := backend.Ping(ctx); err != nil {
			t.Fatalf("Ping: %v", err)
		}
	})

	t.Run("StoreAndSearch", func(t *testing.T) {
		if err := backend.Store(ctx, ids.Chunk1, makeVec(1.0), map[string]any{"source": "test"}); err != nil {
			t.Fatalf("Store: %v", err)
		}
		if err := backend.Store(ctx, ids.Chunk2, makeVec(0.9), nil); err != nil {
			t.Fatalf("Store: %v", err)
		}
		if err := backend.Store(ctx, ids.Chunk3, makeVec(0.1), nil); err != nil {
			t.Fatalf("Store: %v", err)
		}

		results, err := backend.Search(ctx, makeVec(1.0), vector.Filter{}, 2)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].ChunkID != ids.Chunk1 {
			t.Errorf("expected %s first, got %s", ids.Chunk1, results[0].ChunkID)
		}
	})

	t.Run("SearchWithChunkIDFilter", func(t *testing.T) {
		results, err := backend.Search(ctx, makeVec(1.0), vector.Filter{ChunkIDs: []string{ids.Chunk2, ids.Chunk3}}, 10)
		if err != nil {
			t.Fatalf("Search with filter: %v", err)
		}
		for _, r := range results {
			if r.ChunkID == ids.Chunk1 {
				t.Errorf("%s should be excluded by filter", ids.Chunk1)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := backend.Delete(ctx, ids.Chunk3); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		results, err := backend.Search(ctx, makeVec(0.1), vector.Filter{}, 10)
		if err != nil {
			t.Fatalf("Search after delete: %v", err)
		}
		for _, r := range results {
			if r.ChunkID == ids.Chunk3 {
				t.Errorf("%s should be deleted", ids.Chunk3)
			}
		}
	})

	t.Run("StoreBatch", func(t *testing.T) {
		items := []vector.StoreItem{
			{ID: ids.Batch1, Embedding: makeVec(0.5), Metadata: map[string]any{"batch": true}},
			{ID: ids.Batch2, Embedding: makeVec(0.6), Metadata: nil},
		}
		if err := backend.StoreBatch(ctx, items); err != nil {
			t.Fatalf("StoreBatch: %v", err)
		}
	})

	t.Run("DeleteBatch", func(t *testing.T) {
		if err := backend.DeleteBatch(ctx, []string{ids.Batch1, ids.Batch2}); err != nil {
			t.Fatalf("DeleteBatch: %v", err)
		}
	})

	// Cleanup.
	_ = backend.DeleteBatch(ctx, []string{ids.Chunk1, ids.Chunk2})
}
