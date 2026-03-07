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

- **Unit tests**: alongside source files (`foo_test.go` next to `foo.go`). Mock-based unit tests for error paths live in `internal/store/store_test.go`, `internal/server/helpers_test.go`, `internal/retrieval/search_unit_test.go`, and `internal/auth/bootstrap_test.go`.
- **Integration tests**: gated on `TEST_POSTGRES_URL` env var; skipped otherwise. CI provides a PostgreSQL service and sets this automatically.
- **Vector backend conformance**: `internal/vector/vectortest/conformance.go` defines a reusable test suite. Every backend implementation must pass it.
- **Query counter tests**: `internal/store/dbtest/` verifies that search and other hot paths use bounded query counts.
- **Coverage enforcement**: `./scripts/check-coverage.sh` runs tests and verifies every uncovered line has a `// coverage:ignore - <reason>` annotation. Use this annotation only for lines that are genuinely unreachable in tests (e.g., database connection failures, OIDC provider infrastructure). Never use it for testable code.

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

## Documentation Rules

- Any commit that changes behavior, adds features, or modifies the API **must** update relevant docs and add a CHANGELOG.md entry under `[Unreleased]`.
- Changelog entries should describe the change from the user's perspective, not the developer's. Good: "Search results now include metadata scores." Bad: "Added score field to SearchResult proto."
- Do **not** add changelog entries for test-only changes or internal refactoring with no behavior change.
- See `scripts/make-tag` for the release process.

## Release Process

- `scripts/make-tag X.Y.Z` handles the full release cycle: validates preconditions, runs `make all`, updates Chart.yaml and CHANGELOG.md, commits, and creates an annotated tag.
- Must be on `main` with a clean working directory.
- CI must be passing before tagging.
- After tagging: `git push origin main vX.Y.Z` to trigger the release workflow.

## Git Workflow

- **Never merge a PR without CI passing.** Always wait for all CI checks to go green before merging. No exceptions unless the human explicitly says to skip.
- This repo only allows rebase merges (no squash, no merge commits).

## Key Interfaces

- `store.DBTX`: database interface (Exec/Query/QueryRow/Begin) accepted by all stores. `*pgxpool.Pool` and `dbtest.QueryCounter` both satisfy it.
- `vector.Backend`: pluggable vector storage (Store/Delete/Search/Ping).
- `auth.Authorizer`: ACL checks (Check/AccessibleTopics).
