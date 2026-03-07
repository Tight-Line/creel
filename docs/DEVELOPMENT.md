# Development Guide

## Prerequisites

- Go 1.26+
- Docker (for PostgreSQL/pgvector)
- [buf](https://buf.build/docs/installation) (protobuf codegen)
- [golangci-lint](https://golangci-lint.run/welcome/install/) (linting)

## Clone and build

```bash
git clone https://github.com/Tight-Line/creel.git
cd creel
make build
```

This builds three binaries into `bin/`:

- `creel` ; the server
- `creel-cli` ; the CLI client
- `creel-chat` ; the interactive demo agent

## Running tests

### Unit tests

```bash
go test ./...
```

Unit tests run without any external dependencies.

### Integration tests

Integration tests require a running PostgreSQL instance with pgvector:

```bash
docker compose up -d postgres

TEST_POSTGRES_URL="postgres://creel:creel@localhost:5432/creel?sslmode=disable" go test ./...
```

Integration tests are gated on the `TEST_POSTGRES_URL` environment variable. Without it, they are skipped. In CI, a PostgreSQL service container is started automatically and `TEST_POSTGRES_URL` is set, so integration tests always run there.

### Coverage check

The coverage enforcement script runs all tests and verifies that every uncovered line has a `// coverage:ignore - <reason>` annotation:

```bash
TEST_POSTGRES_URL="postgres://creel:creel@localhost:5432/creel?sslmode=disable" ./scripts/check-coverage.sh
```

This is the same script CI runs. Use `// coverage:ignore - <reason>` only for lines that are genuinely unreachable in tests (database connection failures, OIDC provider infrastructure). Never use it for testable code.

### Running the full pipeline

```bash
make all
```

This runs lint, vet, test, and build in sequence.

## Protobuf codegen

Proto definitions live in `proto/`. After modifying `.proto` files:

```bash
make proto-gen
```

This runs `buf generate` and `go mod tidy`. Lint proto files separately with:

```bash
make proto-lint
```

## Linting

```bash
make lint
```

Uses golangci-lint. Configuration is in `.golangci.yml` if present.

## Code style

### Import ordering

Group imports as stdlib, external, internal with a blank line between groups. Use `goimports` to format automatically.

```go
import (
    "context"
    "fmt"

    "google.golang.org/grpc"

    "github.com/Tight-Line/creel/internal/store"
)
```

### Error wrapping

Always wrap errors with context:

```go
return fmt.Errorf("creating topic: %w", err)
```

Never return bare errors.

### N+1 query avoidance

Never loop over a collection making one database call per iteration. Use batch queries with `WHERE id = ANY($1)`. See AGENTS.md for examples and the `internal/store/dbtest` package for query counter tests that enforce this.

## Adding a new RPC endpoint

1. **Define the proto**: add the RPC to the appropriate service in `proto/`, define request/response messages.
2. **Generate code**: run `make proto-gen`.
3. **Implement the server method**: add the handler in the relevant file under `internal/server/`.
4. **Add store methods**: if the RPC needs new database queries, add them in `internal/store/`.
5. **Write tests**: unit test the server method; add an integration test if it touches the database.
6. **Update API_REFERENCE.md**: document the new RPC.
7. **Update CHANGELOG.md**: add an entry under `[Unreleased]`.

## Project structure

```
cmd/creel/          server entrypoint
cmd/creel-cli/      CLI entrypoint
cmd/creel-chat/     interactive demo agent
internal/auth/      authentication + authorization (OIDC, API keys, grants)
internal/config/    configuration loading
internal/retrieval/ RAG search logic
internal/server/    gRPC service implementations
internal/store/     PostgreSQL persistence (chunks, documents, topics, grants)
internal/vector/    vector backend interface + implementations
migrations/         SQL migrations (golang-migrate)
proto/              protobuf definitions
deploy/docker/      Dockerfile
deploy/helm/creel/  Helm chart
```

## Useful references

- [Architecture](../ARCHITECTURE.md): full design document and roadmap
- [API Reference](../API_REFERENCE.md): all 28 RPCs
- [Concepts](CONCEPTS.md): data model and design for integrators
