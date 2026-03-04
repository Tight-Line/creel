package store

import (
	"os"
	"testing"
)

func TestRunMigrations(t *testing.T) {
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL not set; skipping integration test")
	}

	// Run migrations up.
	if err := RunMigrations(pgURL, "../../migrations"); err != nil {
		t.Fatalf("RunMigrations up: %v", err)
	}

	// Running again should be a no-op (no error).
	if err := RunMigrations(pgURL, "../../migrations"); err != nil {
		t.Fatalf("RunMigrations idempotent: %v", err)
	}
}
