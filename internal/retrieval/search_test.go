package retrieval_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/retrieval"
	"github.com/Tight-Line/creel/internal/store"
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

func fakeEmbedding(seed int, dim int) []float64 {
	emb := make([]float64, dim)
	for i := range emb {
		emb[i] = float64((seed+i)%100) / 100.0
	}
	return emb
}

func TestSearcher_EndToEnd(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	pgCfg := config.PostgresConfigFromEnv()
	if err := store.RunMigrations(pgCfg.URL(), "../../migrations"); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	principal := &auth.Principal{ID: "system:search-test", IsSystem: true}
	backend := pgvector.New(pool)
	dim := backend.EmbeddingDimension()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	chunkStore := store.NewChunkStore(pool)
	grantStore := store.NewGrantStore(pool)

	// Clean up stale data.
	for i := 0; i < 3; i++ {
		slug := fmt.Sprintf("search-test-topic-%d", i)
		_, _ = pool.Exec(ctx, `DELETE FROM topics WHERE slug = $1`, slug)
	}

	// Seed topics with grants, documents, chunks, and embeddings.
	var topicIDs []string
	for i := 0; i < 3; i++ {
		slug := fmt.Sprintf("search-test-topic-%d", i)
		topic, err := topicStore.Create(ctx, slug, slug, "test", principal.ID, nil, nil, nil)
		if err != nil {
			t.Fatalf("creating topic %d: %v", i, err)
		}
		topicIDs = append(topicIDs, topic.ID)

		if _, err := topicStore.Grant(ctx, topic.ID, principal.ID, "read", principal.ID); err != nil {
			t.Fatalf("granting access: %v", err)
		}

		doc, err := docStore.Create(ctx, topic.ID, fmt.Sprintf("doc-%d", i), fmt.Sprintf("Doc %d", i), "text", nil)
		if err != nil {
			t.Fatalf("creating document: %v", err)
		}

		for ci := 0; ci < 3; ci++ {
			chunk, err := chunkStore.Create(ctx, doc.ID, fmt.Sprintf("content %d-%d", i, ci), ci, nil)
			if err != nil {
				t.Fatalf("creating chunk: %v", err)
			}
			emb := fakeEmbedding(i*10+ci, dim)
			if err := backend.Store(ctx, chunk.ID, emb, nil); err != nil {
				t.Fatalf("storing embedding: %v", err)
			}
			if err := chunkStore.SetEmbeddingID(ctx, chunk.ID, chunk.ID); err != nil {
				t.Fatalf("setting embedding ID: %v", err)
			}
		}
	}
	t.Cleanup(func() {
		for _, id := range topicIDs {
			_ = topicStore.Delete(ctx, id)
		}
	})

	authorizer := auth.NewGrantAuthorizer(grantStore)
	searcher := retrieval.NewSearcher(chunkStore, authorizer, backend)

	t.Run("search all topics", func(t *testing.T) {
		results, err := searcher.Search(ctx, principal, nil, fakeEmbedding(0, dim), 10, nil, nil)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected results")
		}
		// Verify results have topic IDs.
		for _, r := range results {
			if r.TopicID == "" {
				t.Error("result missing TopicID")
			}
			if r.Chunk == nil {
				t.Error("result missing Chunk")
			}
		}
	})

	t.Run("search specific topics", func(t *testing.T) {
		results, err := searcher.Search(ctx, principal, topicIDs[:1], fakeEmbedding(0, dim), 10, nil, nil)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		for _, r := range results {
			if r.TopicID != topicIDs[0] {
				t.Errorf("result from wrong topic: %s", r.TopicID)
			}
		}
	})

	t.Run("search inaccessible topic returns empty", func(t *testing.T) {
		stranger := &auth.Principal{ID: "user:stranger@example.com"}
		results, err := searcher.Search(ctx, stranger, nil, fakeEmbedding(0, dim), 10, nil, nil)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results for stranger, got %d", len(results))
		}
	})

	t.Run("search with metadata filter no match", func(t *testing.T) {
		results, err := searcher.Search(ctx, principal, nil, fakeEmbedding(0, dim), 10, map[string]any{"nonexistent": "value"}, nil)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results with non-matching metadata, got %d", len(results))
		}
	})

	t.Run("search requested topics filtered to accessible", func(t *testing.T) {
		// Request a topic the principal has access to plus a non-existent one.
		results, err := searcher.Search(ctx, principal, []string{topicIDs[0], "non-existent-id"}, fakeEmbedding(0, dim), 10, nil, nil)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		for _, r := range results {
			if r.TopicID != topicIDs[0] {
				t.Errorf("result from wrong topic: %s", r.TopicID)
			}
		}
	})

	t.Run("search with only non-accessible requested topics", func(t *testing.T) {
		results, err := searcher.Search(ctx, principal, []string{"non-existent-id"}, fakeEmbedding(0, dim), 10, nil, nil)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}
