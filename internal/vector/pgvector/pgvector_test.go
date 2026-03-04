package pgvector

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/vector/vectortest"
)

func TestPgvectorConformance(t *testing.T) {
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL not set; skipping integration test")
	}

	if err := store.RunMigrations(pgURL, "../../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	ids := vectortest.TestIDs
	allIDs := []string{ids.Chunk1, ids.Chunk2, ids.Chunk3, ids.Batch1, ids.Batch2}

	// Seed parent rows required by the chunk_embeddings FK constraint.
	// Clean up stale data first (cascade handles embeddings).
	_, _ = pool.Exec(ctx, `DELETE FROM topics WHERE slug = 'conformance-test'`)

	topicStore := store.NewTopicStore(pool)
	topic, err := topicStore.Create(ctx, "conformance-test", "Conformance", "test", "system:test")
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	docStore := store.NewDocumentStore(pool)
	doc, err := docStore.Create(ctx, topic.ID, "conformance-doc", "Conformance Doc", "text", nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	for i, id := range allIDs {
		// Insert chunk rows with the exact IDs the conformance suite expects.
		_, err := pool.Exec(ctx,
			`INSERT INTO chunks (id, document_id, sequence, content)
			 VALUES ($1, $2, $3, 'conformance test')
			 ON CONFLICT (id) DO NOTHING`,
			id, doc.ID, i,
		)
		if err != nil {
			t.Fatalf("seeding chunk %s: %v", id, err)
		}
	}

	backend := New(pool)
	vectortest.RunConformanceTests(t, backend)
}
