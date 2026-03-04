package pgvector

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/vector/vectortest"
)

func TestPgvectorConformance(t *testing.T) {
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL not set; skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	defer pool.Close()

	backend := New(pool)
	vectortest.RunConformanceTests(t, backend)
}
