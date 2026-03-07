package store

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
)

var validSchemaName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// EnsureSchema creates the given schema if it does not already exist.
// It opens a one-shot connection to baseURL (which should not include
// search_path), validates the schema name, runs CREATE SCHEMA IF NOT EXISTS,
// and closes the connection.
func EnsureSchema(ctx context.Context, baseURL, schema string) error {
	if !validSchemaName.MatchString(schema) {
		return fmt.Errorf("invalid schema name: %q", schema)
	}

	conn, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		return fmt.Errorf("connecting for schema creation: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// Schema name is validated against a strict regex above, so direct
	// interpolation is safe here. PostgreSQL does not support parameterized
	// DDL identifiers.
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	// coverage:ignore - requires a connected database that rejects valid DDL
	if err != nil {
		return fmt.Errorf("creating schema %q: %w", schema, err)
	}

	return nil
}
