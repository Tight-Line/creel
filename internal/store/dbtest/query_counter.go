// Package dbtest provides test helpers for database stores.
package dbtest

import (
	"context"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Tight-Line/creel/internal/store"
)

// QueryCounter wraps a store.DBTX and counts every query-level call
// (Exec, Query, QueryRow). Begin is counted once (for the BEGIN itself);
// calls on the returned transaction are also counted via a wrapped Tx.
type QueryCounter struct {
	db    store.DBTX
	count int64
}

// NewQueryCounter wraps the given DBTX with call counting.
func NewQueryCounter(db store.DBTX) *QueryCounter {
	return &QueryCounter{db: db}
}

// Count returns the number of database calls observed so far.
func (qc *QueryCounter) Count() int {
	return int(atomic.LoadInt64(&qc.count))
}

// Reset sets the counter back to zero.
func (qc *QueryCounter) Reset() {
	atomic.StoreInt64(&qc.count, 0)
}

func (qc *QueryCounter) inc() {
	atomic.AddInt64(&qc.count, 1)
}

// Exec delegates to the underlying DBTX and increments the counter.
func (qc *QueryCounter) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	qc.inc()
	return qc.db.Exec(ctx, sql, arguments...)
}

// Query delegates to the underlying DBTX and increments the counter.
func (qc *QueryCounter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	qc.inc()
	return qc.db.Query(ctx, sql, args...)
}

// QueryRow delegates to the underlying DBTX and increments the counter.
func (qc *QueryCounter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	qc.inc()
	return qc.db.QueryRow(ctx, sql, args...)
}

// Begin delegates to the underlying DBTX and returns a counted transaction.
func (qc *QueryCounter) Begin(ctx context.Context) (pgx.Tx, error) {
	qc.inc()
	tx, err := qc.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &countedTx{Tx: tx, qc: qc}, nil
}

// countedTx wraps a pgx.Tx so that query calls on the transaction are also counted.
type countedTx struct {
	pgx.Tx
	qc *QueryCounter
}

func (ct *countedTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	ct.qc.inc()
	return ct.Tx.Exec(ctx, sql, arguments...)
}

func (ct *countedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	ct.qc.inc()
	return ct.Tx.Query(ctx, sql, args...)
}

func (ct *countedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	ct.qc.inc()
	return ct.Tx.QueryRow(ctx, sql, args...)
}
