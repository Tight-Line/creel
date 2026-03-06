package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is the interface satisfied by *pgxpool.Pool and *pgx.Tx.
// All stores accept this instead of a concrete pool so that a
// QueryCounter (or transaction) can be substituted in tests.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// NewPool creates a new pgxpool connection pool.
// coverage:ignore - database infrastructure
func NewPool(ctx context.Context, postgresURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, postgresURL)
	// coverage:ignore - database infrastructure
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// coverage:ignore - database infrastructure
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// coverage:ignore - database infrastructure
	return pool, nil
}
