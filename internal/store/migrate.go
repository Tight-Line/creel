// Package store provides PostgreSQL-backed data access for Creel.
package store

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies all pending database migrations.
// coverage:ignore - database infrastructure
func RunMigrations(postgresURL, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, postgresURL)
	// coverage:ignore - database infrastructure
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	// coverage:ignore - database infrastructure
	defer func() { _, _ = m.Close() }()

	// coverage:ignore - database infrastructure
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	// coverage:ignore - database infrastructure
	return nil
}
