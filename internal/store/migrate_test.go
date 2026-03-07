package store

import (
	"context"
	"testing"

	"github.com/Tight-Line/creel/internal/config"
)

func TestRunMigrations(t *testing.T) {
	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()
	if err := EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	pgURL := pgCfg.URL()

	// Run migrations up.
	if err := RunMigrations(pgURL, "../../migrations"); err != nil {
		t.Fatalf("RunMigrations up: %v", err)
	}

	// Running again should be a no-op (no error).
	if err := RunMigrations(pgURL, "../../migrations"); err != nil {
		t.Fatalf("RunMigrations idempotent: %v", err)
	}
}
