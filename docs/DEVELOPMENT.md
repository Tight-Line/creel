# Development Guide

## Prerequisites

- Go 1.26+
- Docker (for PostgreSQL/pgvector)
- [buf](https://buf.build/docs/installation) (protobuf codegen)
- [golangci-lint](https://golangci-lint.run/welcome/install/) (linting)
- `protoc-gen-go` and `protoc-gen-go-grpc` (installed automatically by `go install`, see Protobuf codegen below)

## Local development environment

The repository ships with a pre-configured dev API key for local development. After cloning:

```bash
source .env
```

This sets `CREEL_ENDPOINT` and `CREEL_API_KEY` so that `creel-cli` and `creel-chat` authenticate against a local server running with `creel.example.yaml`. The dev key is intentionally public; it is only useful against a local instance.

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
make test-integration
```

This starts the docker-compose postgres (if not already running), sets the `CREEL_POSTGRES_*` environment variables, and runs the full coverage suite. It uses a separate `creel_test` schema so it won't clobber local dev data.

You can also run integration tests manually:

```bash
docker compose up -d postgres

CREEL_POSTGRES_HOST=localhost CREEL_POSTGRES_USER=creel CREEL_POSTGRES_PASSWORD=creel CREEL_POSTGRES_NAME=creel go test ./...
```

Integration tests are gated on the `CREEL_POSTGRES_HOST` environment variable. Without it, they are skipped. In CI, a PostgreSQL service container is started automatically and `CREEL_POSTGRES_*` vars are set, so integration tests always run there.

### Coverage check

The coverage enforcement script runs all tests and verifies that every uncovered line has a `// coverage:ignore - <reason>` annotation:

```bash
CREEL_POSTGRES_HOST=localhost CREEL_POSTGRES_USER=creel CREEL_POSTGRES_PASSWORD=creel CREEL_POSTGRES_NAME=creel ./scripts/check-coverage.sh
```

This is the same script CI runs. Use `// coverage:ignore - <reason>` only for lines that are genuinely unreachable in tests (database connection failures, OIDC provider infrastructure). Never use it for testable code.

### Running the full pipeline

```bash
make all
```

This runs lint, vet, test, and build in sequence.

## Live-reload dev workflow

For iterative development without rebuilding Docker images:

```bash
make dev
```

This starts postgres, runs migrations, and launches the Creel server inside a dev container with [Air](https://github.com/air-verse/air). Source files are bind-mounted; saving a `.go`, `.proto`, or `.yaml` file triggers an automatic rebuild and restart. Go module and build caches are stored in named volumes for fast rebuilds.

```bash
make dev-down      # tear down the dev stack
make dev-migrate   # run migrations without bouncing the stack
```

The production `docker-compose.yml` and `Dockerfile` are not modified; `docker-compose.dev.yml` is an override file.

## Protobuf codegen

Proto definitions live in `proto/`. Codegen uses local plugins (`protoc-gen-go`, `protoc-gen-go-grpc`), not remote BSR execution. Install them once:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

After modifying `.proto` files:

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

## Adding a new worker type

1. **Define the job type**: add the new value to the `processing_job` enum.
2. **Implement the worker**: create the worker in `internal/worker/`.
3. **Register the worker type**: wire it into the worker pool so it picks up jobs of the new type.
4. **Write integration tests**: use mock LLM/embedding providers to exercise the worker without real external calls.
5. **Update the dashboard**: if the worker has user-visible status, add or update the relevant dashboard views.

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
internal/worker/    background job processing (extraction, chunking, embedding, memory)
internal/memory/    memory extraction and maintenance logic
internal/crypto/    AES-256-GCM encryption for API key configs
migrations/         SQL migrations (golang-migrate)
proto/              protobuf definitions
deploy/docker/      Dockerfile (production) and Dockerfile.dev (live-reload)
deploy/helm/creel/  Helm chart
dashboard/          Laravel admin dashboard
```

## Useful references

- [Architecture](ARCHITECTURE.md): full design document and phase roadmap, covering document processing, memory, and server-side workers
- [API Reference](API_REFERENCE.md): all 28 RPCs
- [Concepts](CONCEPTS.md): data model and design for integrators
