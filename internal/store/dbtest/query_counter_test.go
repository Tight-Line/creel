package dbtest_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/retrieval"
	"github.com/Tight-Line/creel/internal/store"
	"github.com/Tight-Line/creel/internal/store/dbtest"
	"github.com/Tight-Line/creel/internal/vector"
	"github.com/Tight-Line/creel/internal/vector/pgvector"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}
	ctx := context.Background()
	if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	pool, err := pgxpool.New(ctx, pgCfg.URL())
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// fakeEmbedding returns a deterministic embedding of the given dimension.
func fakeEmbedding(seed int, dim int) []float64 {
	emb := make([]float64, dim)
	for i := range emb {
		emb[i] = float64((seed+i)%100) / 100.0
	}
	return emb
}

// seedFixture creates a multi-topic, multi-document, multi-chunk fixture
// designed to surface N+1 regressions at every level (topic, document, chunk).
// Returns the topic IDs created. Embedding dimension is read from the backend.
//
// Layout:
//   - 3 topics, each owned by principal
//   - 2 documents per topic
//   - 5 chunks per document (30 chunks total)
//   - every chunk has an embedding in the vector backend
func seedFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, backend vector.Backend, principal *auth.Principal) []string {
	t.Helper()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	embDim := backend.EmbeddingDimension()

	const (
		numTopics    = 3
		docsPerTopic = 2
		chunksPerDoc = 5
	)

	var topicIDs []string
	chunkSeq := 0

	// Clean up any stale data from previous failed runs before seeding.
	// Topic deletion cascades to documents, chunks, and grants.
	for ti := 0; ti < numTopics; ti++ {
		slug := fmt.Sprintf("qc-topic-%d", ti)
		_, _ = pool.Exec(ctx, `DELETE FROM topics WHERE slug = $1`, slug)
	}

	for ti := 0; ti < numTopics; ti++ {
		slug := fmt.Sprintf("qc-topic-%d", ti)
		topic, err := topicStore.Create(ctx, slug, slug, "test topic", principal.ID)
		if err != nil {
			t.Fatalf("creating topic %d: %v", ti, err)
		}
		topicIDs = append(topicIDs, topic.ID)

		// AccessibleTopics only checks grants, not ownership, so we need
		// an explicit grant for the principal to find its own topics.
		if _, err := topicStore.Grant(ctx, topic.ID, principal.ID, "admin", principal.ID); err != nil {
			t.Fatalf("granting access on topic %d: %v", ti, err)
		}

		for di := 0; di < docsPerTopic; di++ {
			docSlug := fmt.Sprintf("doc-%d-%d", ti, di)
			doc, err := docStore.Create(ctx, topic.ID, docSlug, docSlug, "text", nil)
			if err != nil {
				t.Fatalf("creating document %s: %v", docSlug, err)
			}

			for ci := 0; ci < chunksPerDoc; ci++ {
				chunk, err := chunkStore.Create(ctx, doc.ID, fmt.Sprintf("content %d", chunkSeq), ci, nil)
				if err != nil {
					t.Fatalf("creating chunk %d: %v", chunkSeq, err)
				}

				emb := fakeEmbedding(chunkSeq, embDim)
				if err := backend.Store(ctx, chunk.ID, emb, nil); err != nil {
					t.Fatalf("storing embedding %d: %v", chunkSeq, err)
				}
				if err := chunkStore.SetEmbeddingID(ctx, chunk.ID, chunk.ID); err != nil {
					t.Fatalf("setting embedding ID %d: %v", chunkSeq, err)
				}
				chunkSeq++
			}
		}
	}

	t.Cleanup(func() {
		for _, id := range topicIDs {
			_ = topicStore.Delete(ctx, id)
		}
	})

	return topicIDs
}

// TestSearchQueryCount verifies that Search uses a bounded number of queries
// regardless of how many topics, documents, or chunks are involved.
func TestSearchQueryCount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	pgCfg := config.PostgresConfigFromEnv()
	if err := store.RunMigrations(pgCfg.URL(), "../../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	principal := &auth.Principal{ID: "system:test-counter", IsSystem: true}
	vectorBackend := pgvector.New(pool)
	topicIDs := seedFixture(t, ctx, pool, vectorBackend, principal)
	embDim := vectorBackend.EmbeddingDimension()

	// Set up the searcher with a query counter.
	counter := dbtest.NewQueryCounter(pool)
	countedChunkStore := store.NewChunkStore(counter)
	countedGrantStore := store.NewGrantStore(counter)
	authorizer := auth.NewGrantAuthorizer(countedGrantStore)
	searcher := retrieval.NewSearcher(countedChunkStore, authorizer, vectorBackend)

	// Search across all topics (no topic filter).
	counter.Reset()
	results, err := searcher.Search(ctx, principal, nil, fakeEmbedding(0, embDim), 15, nil)
	if err != nil {
		t.Fatalf("search (all topics) failed: %v", err)
	}
	allCount := counter.Count()
	t.Logf("search (all topics): %d results, %d queries", len(results), allCount)

	// The batched path should use:
	// 1 for AccessibleTopics (grants), 1 for ChunkIDsByTopics,
	// 1 for GetMultiple, 1 for DocumentTopicIDs = 4 queries.
	// Allow headroom to 6 but NOT anything proportional to result count.
	const maxQueries = 6
	if allCount > maxQueries {
		t.Errorf("search (all topics) used %d queries (max %d); likely N+1 regression", allCount, maxQueries)
	}

	// Search with explicit topic filter (subset).
	counter.Reset()
	results, err = searcher.Search(ctx, principal, topicIDs[:2], fakeEmbedding(0, embDim), 15, nil)
	if err != nil {
		t.Fatalf("search (filtered topics) failed: %v", err)
	}
	filteredCount := counter.Count()
	t.Logf("search (filtered topics): %d results, %d queries", len(results), filteredCount)

	if filteredCount > maxQueries {
		t.Errorf("search (filtered topics) used %d queries (max %d); likely N+1 regression", filteredCount, maxQueries)
	}

	// Search with a metadata filter; should not add extra queries.
	counter.Reset()
	results, err = searcher.Search(ctx, principal, nil, fakeEmbedding(0, embDim), 15, map[string]any{"key": "value"})
	if err != nil {
		t.Fatalf("search (metadata filter) failed: %v", err)
	}
	metaCount := counter.Count()
	t.Logf("search (metadata filter): %d results, %d queries", len(results), metaCount)

	if metaCount > maxQueries {
		t.Errorf("search (metadata filter) used %d queries (max %d); likely N+1 regression", metaCount, maxQueries)
	}
}

// TestNPlusOneDetection verifies the counter catches an N+1 pattern by
// simulating individual Get calls in a loop, confirming the counter reflects N queries.
func TestNPlusOneDetection(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	pgCfg2 := config.PostgresConfigFromEnv()
	if err := store.RunMigrations(pgCfg2.URL(), "../../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	principal := &auth.Principal{ID: "system:test-n1", IsSystem: true}
	topicStore := store.NewTopicStore(pool)

	// Clean up stale data from previous failed runs.
	_, _ = pool.Exec(ctx, `DELETE FROM topics WHERE slug = 'n1-test'`)

	topic, err := topicStore.Create(ctx, "n1-test", "N1 Test", "test topic", principal.ID)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() {
		_ = topicStore.Delete(ctx, topic.ID)
	})

	docStore := store.NewDocumentStore(pool)
	doc, err := docStore.Create(ctx, topic.ID, "n1-doc", "N1 Doc", "text", nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	// Create chunks.
	const n = 10
	chunkIDs := make([]string, n)
	chunkStorePlain := store.NewChunkStore(pool)
	for i := 0; i < n; i++ {
		chunk, err := chunkStorePlain.Create(ctx, doc.ID, "content", i, nil)
		if err != nil {
			t.Fatalf("creating chunk %d: %v", i, err)
		}
		chunkIDs[i] = chunk.ID
	}

	// Simulate N+1: loop with individual Get calls through the counter.
	counter := dbtest.NewQueryCounter(pool)
	countedStore := store.NewChunkStore(counter)

	counter.Reset()
	for _, id := range chunkIDs {
		_, _ = countedStore.Get(ctx, id)
	}
	got := counter.Count()

	if got != n {
		t.Errorf("expected %d queries for N+1 pattern, got %d", n, got)
	}

	// Now verify the batch path uses 1 query.
	counter.Reset()
	_, err = countedStore.GetMultiple(ctx, chunkIDs)
	if err != nil {
		t.Fatalf("GetMultiple failed: %v", err)
	}
	gotBatch := counter.Count()
	if gotBatch != 1 {
		t.Errorf("expected 1 query for batch Get, got %d", gotBatch)
	}
}
