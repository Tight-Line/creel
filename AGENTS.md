# Creel

Creel is a self-hosted memory-as-a-service platform for AI agents. Go backend, gRPC API, PostgreSQL metadata store, pgvector for embeddings.

## Build & Test

```bash
make build                    # builds cmd/creel and cmd/creel-cli
go test ./...                 # unit tests only
go vet ./...                  # static analysis

# Integration tests (require a running PostgreSQL with pgvector):
TEST_POSTGRES_URL="postgres://creel:creel@localhost:5432/creel?sslmode=disable" go test ./...

# Docker environment for integration tests:
docker compose up -d postgres
```

## Code Style

- **Imports**: group as stdlib, external, internal (blank line between groups). Use `goimports`.
- **Linting**: `golangci-lint run`
- **Error wrapping**: use `fmt.Errorf("context: %w", err)`, not bare returns.
- **No em-dashes or en-dashes** as separators in comments or docs; use semicolons or new sentences.

## Database Query Rules

**Never write N+1 query patterns.** Do not loop over a collection making a database call per iteration. Always use batch queries:

- Use `WHERE id = ANY($1)` with a slice parameter for multi-row fetches.
- Collect IDs first, then batch-fetch in one query.
- If you need data from two tables, do two batch queries, not N individual lookups.

Bad (N+1):

```go
for _, id := range ids {
    chunk, _ := store.Get(ctx, id)  // 1 query per iteration
}
```

Good (batch):

```go
chunks, _ := store.GetMultiple(ctx, ids)  // 1 query total
```

The `internal/store/dbtest` package provides a `QueryCounter` wrapper that counts database calls. Integration tests use it to assert bounded query counts and catch N+1 regressions. See `internal/store/dbtest/query_counter_test.go` for examples.

## Testing Conventions

- **Unit tests**: alongside source files (`foo_test.go` next to `foo.go`).
- **Integration tests**: gated on `TEST_POSTGRES_URL` env var; skipped otherwise.
- **Vector backend conformance**: `internal/vector/vectortest/conformance.go` defines a reusable test suite. Every backend implementation must pass it.
- **Query counter tests**: `internal/store/dbtest/` verifies that search and other hot paths use bounded query counts.

### N+1 query counter test fixtures

Query counter tests must seed enough data to actually trigger N+1 regressions. A fixture with one topic, one document, and a few chunks will not catch a loop that fires once per topic or once per document. Fixtures should include:

- **Multiple topics** (>= 3) with grants to the test principal
- **Multiple documents per topic** (>= 2) so document-level loops are visible
- **Multiple chunks per document** (>= 5) so chunk-level loops are visible
- **Embeddings for every chunk** so vector search returns results across the full fixture

The query count bound must hold regardless of how many results come back. If you add a new code path that touches the database during search or hydration, add or update a query counter test that exercises it with this multi-entity fixture.

## Project Structure

```
cmd/creel/          server entrypoint
cmd/creel-cli/      CLI entrypoint
internal/auth/      authentication + authorization (OIDC, API keys, grants)
internal/config/    configuration loading
internal/retrieval/ RAG search logic
internal/server/    gRPC service implementations
internal/store/     PostgreSQL persistence (chunks, documents, topics, grants)
internal/vector/    vector backend interface + implementations
migrations/         SQL migrations (golang-migrate)
proto/              protobuf definitions
```

## Key Interfaces

- `store.DBTX`: database interface (Exec/Query/QueryRow/Begin) accepted by all stores. `*pgxpool.Pool` and `dbtest.QueryCounter` both satisfy it.
- `vector.Backend`: pluggable vector storage (Store/Delete/Search/Ping).
- `auth.Authorizer`: ACL checks (Check/AccessibleTopics).
