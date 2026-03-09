# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Memory store for per-principal, scoped knowledge. Agents can store, update, search, and soft-delete memories organized by scope. Memories support optional subject/predicate/object triples and metadata.
- `MemoryService` gRPC API with 7 RPCs: `GetMemory`, `SearchMemories`, `AddMemory`, `UpdateMemory`, `DeleteMemory`, `ListMemories`, and `ListScopes`. All RPCs are automatically scoped to the calling principal.
- Semantic search over memories via vector embeddings. When an embedding provider is configured, new memories are automatically embedded; `SearchMemories` uses cosine similarity. Falls back to returning all active memories when no embeddings are available.
- CLI commands for memory management: `creel-cli memory list`, `memory add`, `memory delete`, `memory scopes`, and `memory search`.
- REST endpoints for all memory RPCs via grpc-gateway.
- Document processing pipeline now runs end-to-end: extraction, chunking, embedding. Uploaded documents are automatically split into chunks and embedded, becoming searchable without manual intervention.
- Chunking worker splits extracted text into fixed-size chunks with configurable overlap (default: 2048 characters, 200 character overlap).
- Embedding worker computes vector embeddings for document chunks and stores them in the vector backend. Documents are marked "ready" once all chunks are embedded.
- Topics support a `chunking_strategy` field (JSONB) to customize chunk size and overlap per topic. NULL uses server defaults.
- Pluggable `EmbeddingProvider` interface for computing embeddings. Ships with a deterministic stub provider for testing; real OpenAI/Ollama providers can be added later.
- `UploadDocument` RPC for managed document ingestion. Upload a file directly or provide a `source_url` to fetch from. Creates the document with status `pending` and enqueues an extraction job automatically.
- Document status tracking. Documents now have a `status` field (`pending`, `processing`, `ready`, `failed`) that reflects their processing state. Documents created via `CreateDocument` default to `ready`; uploads start as `pending`.
- Extraction worker that processes uploaded documents. Supports `text/plain` (passthrough) and `text/html` (tag stripping with script/style removal). Unsupported content types fail gracefully and mark the document as `failed`.
- Auto-generated slugs for topics and documents. When `slug` is omitted from `CreateTopic`, `CreateDocument`, or `UploadDocument`, a URL-friendly slug is generated from the name with a random 4-character suffix.
- HTTP fetcher for downloading documents from a `source_url` with a 30-second timeout and 100MB size limit.
- CLI `upload` command for uploading documents from local files or remote URLs.
- Separate `document_content` table for storing raw uploaded bytes and extracted text, keeping the main documents table lean.
- Background job infrastructure for document processing pipelines. Jobs track status (queued, running, completed, failed) with progress and error details. The extraction worker is registered; chunking and embedding workers are planned.
- New `JobService` API with `GetJob` and `ListJobs` RPCs. Jobs can be filtered by topic, document, or status. Permission checks ensure you can only see jobs for topics you have read access to.
- CLI commands `creel-cli jobs list` and `creel-cli jobs status <id>` for monitoring processing jobs.
- Worker pool configuration via `workers.concurrency` and `workers.poll_interval` settings (defaults: 4 workers, 5s poll interval).
- Documents now support optional citation fields: `url`, `author`, and `published_at`. These can be set when creating or updating a document.
- Search results include a `document_citation` with the source document's name, slug, URL, author, and publication date, making it easy to attribute results to their origin.
- Dashboard shows document citation fields (URL, author, published date) and allows editing them. Topics list now links to each topic's documents.
- CLI search output includes citation metadata (url, author, published date) when present.

## [0.1.11] - 2026-03-09

### Fixed

- Increased dashboard readiness probe `initialDelaySeconds` from 5 to 15 and liveness from 10 to 30. The startup script now runs `config:cache`, `route:cache`, and `view:cache` before launching supervisord, which needs more time before the first probe.

## [0.1.10] - 2026-03-09

### Fixed

- Auto-generated Laravel `APP_KEY` was double-base64-encoded. Sprig's `randBytes` already returns base64; piping through `b64enc` encoded it a second time, producing a 44-byte key instead of 32 bytes. Laravel rejected it with "Unsupported cipher or incorrect key length."
- Auto-generated bootstrap API key used `creel_` prefix instead of `creel_ak_`. The auth middleware requires the `creel_ak_` prefix to recognize a token as an API key.
- Dashboard login credentials were read via `env()` instead of `config()`, returning null when config is cached. Moved `CREEL_DASHBOARD_USERNAME` and `CREEL_DASHBOARD_PASSWORD` into `config/creel.php`.
- Mixed-content blocking on HTTPS deployments. Dashboard now trusts reverse proxies (`trustProxies(at: '*')`) so Laravel respects `X-Forwarded-Proto` from the ingress and generates HTTPS asset URLs.
- Restored `config:cache`, `route:cache`, and `view:cache` in the startup script (after `.env` generation). Caching now works correctly since `.env` is populated before artisan runs.

## [0.1.9] - 2026-03-09

### Fixed

- Simplified dashboard startup: removed all `artisan` caching commands (`config:cache`, `route:cache`, `view:cache`). The startup script now just generates `.env` from container env vars and launches supervisord. Caching was causing NULL values because phpdotenv's repository isn't populated when `config:cache` runs inside the container.
- Default gRPC endpoint changed from `localhost:8443` to `127.0.0.1:8443` in creel-chat, creel-cli, docs, and `.env`. Avoids 20-second connection delays on dual-stack systems where `localhost` resolves to `::1` (IPv6) first.

## [0.1.8] - 2026-03-09

### Fixed

- Dashboard startup in Docker/Kubernetes: Laravel's `env()` reads from `.env` via phpdotenv, not from system environment variables. The startup script generates `.env` from the container environment at boot.

## [0.1.7] - 2026-03-09

### Added

- Auto-generated secrets for bootstrap API key, Laravel APP_KEY, dashboard password, and PostgreSQL password. All stored in a single Kubernetes Secret; preserved across upgrades via `lookup`. Zero required values when `postgresql.install=true` and ingress is disabled.
- `NOTES.txt` output after `helm install` showing retrieval commands for each auto-generated secret.
- Init container ordering: creel Deployment waits for PostgreSQL readiness and completed migrations before starting. Dashboard waits for creel REST API. Migration Job waits for PostgreSQL when `postgresql.install=true`.

### Changed

- Migration Job hook changed from `pre-install,pre-upgrade` to `post-install,pre-upgrade` so PostgreSQL StatefulSet exists before migrations run on first install.
- `creel.yaml` config file moved from ConfigMap to Secret, making the Secret the single source of truth for all sensitive values.
- All container env vars for secrets now use `secretKeyRef` instead of inline values.
- Creel readiness and liveness probes switched from gRPC to HTTP (`/v1/health` on the REST port) because the server does not implement `grpc.health.v1.Health`.
- Dashboard config caching moved from Dockerfile build time to runtime startup script, fixing `MissingAppKeyException` when `APP_KEY` is injected via env var.
- Migration Job hook-delete-policy now includes `hook-failed` so failed Jobs are cleaned up automatically.

## [0.1.6] - 2026-03-08

### Added

- Two-phase multi-arch Docker builds: amd64 images are pushed immediately, then updated in-place with arm64 via manifest. Applies to both release and PR image workflows.
- PR image workflow: every PR builds `creel` and `creel-dashboard` images tagged as `pr-{number}-{sha}`, with a PR comment showing pull commands and Helm overrides. Cleanup runs on PR close and daily for stale images (>15 days).
- Database migration Job as a Helm pre-install/pre-upgrade hook. Runs `creel --migrate` once before the Deployment rolls out, avoiding race conditions with multiple replicas.
- PHPUnit coverage reported to both SonarCloud and Codecov alongside Go coverage. CI test job split into `test-go` and `test-php` running in parallel.

### Changed

- Dashboard container now runs as `www-data` instead of root.
- Replaced CloudNativePG operator dependency with a built-in PostgreSQL StatefulSet using `pgvector/pgvector:pg17`.

## [0.1.5] - 2026-03-08

### Added

- Server-side configuration registry for LLM models, embedding models, API keys, and extraction prompts. New `ConfigService` with 24 RPCs (CRUD + SetDefault for each of the four config types). System account authentication required.
- AES-256-GCM encryption for API key configs stored at rest. Configure via `encryption_key` in YAML or `CREEL_ENCRYPTION_KEY` env var.
- Topics can now be bound to LLM, embedding, and extraction prompt configs. NULL means "use default." Server enforces: extraction prompt requires LLM config; embedding config changes must match provider+model.
- REST API via grpc-gateway. All gRPC services are now accessible over HTTP on the REST port (default 8080) with JSON request/response bodies.
- Database migration 000004 adds `api_key_configs`, `llm_configs`, `embedding_configs`, and `extraction_prompt_configs` tables, plus nullable FK columns on `topics`.
- CLI commands for full CRUD on all config types: `creel-cli config apikey`, `creel-cli config llm`, `creel-cli config embedding`, and `creel-cli config prompt`. Each supports `create`, `get`, `list`, `update`, `delete`, and `set-default`.
- CLI `topic create` now accepts `--llm-config`, `--embedding-config`, and `--prompt-config` flags for binding configs at creation time.
- CLI `topic update` command for modifying topic name, description, and config bindings.
- Laravel admin dashboard (`dashboard/`) for managing configs, topics, and system accounts via the REST API. Session-based login with env var credentials. Runs on port 3000.
- Dashboard Helm chart templates: Deployment, Service, and Ingress for the admin dashboard. Dashboard is enabled by default and communicates with Creel via the internal REST API.
- Required-value validation in the Helm chart. Installing without `auth.bootstrapKeyHash`, `dashboard.auth.password`, `dashboard.auth.apiKey`, or ingress hostnames (when ingress is enabled) fails at template time with a clear error message.
- JSON structured logging channel for the dashboard. Set `LOG_CHANNEL=json` for single-line JSON log output; used automatically in Helm deployments.

### Changed

- Dashboard Dockerfile now uses nginx + php-fpm + supervisord for production serving instead of `php artisan serve`. The dev compose override still uses artisan serve for hot-reload.
- Fixed CloudNativePG Helm chart dependency name (was `postgresql`, now correctly references `cloudnative-pg` with alias).

- `TopicService.CreateTopic` and `UpdateTopic` now accept optional `llm_config_id`, `embedding_config_id`, and `extraction_prompt_config_id` fields.
- `Topic` proto message includes config ID fields (field numbers 8-10).
- All proto files now include `google.api.http` annotations for REST routing.

## [0.1.4] - 2026-03-07

### Added

- `GetContext` RPC for temporal context retrieval. Returns chunks from a single document in sequence order, with optional `last_n` and `since` filtering. `include_summaries` is accepted but not yet implemented (compaction awareness is deferred).
- Two-layer retrieval in creel-chat: resuming a session now loads full conversation history via `GetContext` (temporal layer) alongside RAG search (semantic layer). Previously, resumed sessions started with an empty context buffer.
- `exclude_document_ids` field on `SearchRequest`. Excluded documents are filtered server-side before the vector top-K limit, so callers always get up to K cross-session results.
- Air-based live-reload dev workflow. `make dev` bind-mounts source into a dev container and rebuilds on file changes. `make dev-down` and `make dev-migrate` for teardown and one-shot migrations.
- `make test-integration` target runs the full coverage suite against a local Postgres.
- Pre-configured dev API key for local development. `creel.example.yaml` ships with a working `auth.api_keys` entry; `source .env` sets the matching `CREEL_ENDPOINT` and `CREEL_API_KEY` for `creel-cli` and `creel-chat`.

### Changed

- Protobuf codegen now uses local `protoc-gen-go` and `protoc-gen-go-grpc` plugins instead of remote BSR execution. Eliminates BSR rate-limit issues and is faster.
- creel-chat excludes the current session document from RAG search, preventing self-referential results from consuming top-K slots.
- creel-chat system prompt now clearly distinguishes session history (authoritative, verbatim conversation record) from RAG context (snippets from other sessions). Improves LLM recall accuracy on session replay.

- Database tables now live in a dedicated PostgreSQL schema (default: `creel`), configurable via `postgres.schema` or `CREEL_POSTGRES_SCHEMA`. The schema is created automatically on startup.
- PostgreSQL connection is now structured fields (`host`, `port`, `user`, `password`, `name`, `schema`, `sslmode`) under the `postgres:` config key. Replaces the old `metadata.postgres_url` single-string approach. Supports Kubernetes secrets for passwords via Helm `postgresql.auth.existingSecret`.
- Helm chart uses `postgresql.source: helm|external` instead of `postgresql.enabled` to clarify intent.
- Integration tests gate on `CREEL_POSTGRES_HOST` instead of `TEST_POSTGRES_URL`.

## [0.1.3] - 2026-03-07

### Added

- PostgreSQL service in CI and SonarCloud workflows so integration tests run in CI
- Comprehensive unit tests for all store, server, auth, and retrieval error paths
- Integration tests for gRPC server (end-to-end) and retrieval search
- `.dockerignore` to prevent secrets and build artifacts from leaking into images
- Unit tests for json.Marshal metadata failures in ChunkStore, DocumentStore, and pgvector
- Mock-based unit tests for all pgvector DB error paths (Store, Delete, Search, StoreBatch, DeleteBatch, Ping)
- OIDC tests for empty principal claim default, missing claim field, and middleware dispatch branch
- AdminServer Health endpoint unit tests via new Pinger interface

### Changed

- Coverage script merges duplicate `-coverpkg` entries and serializes test packages to prevent flaky results
- Coverage script excludes `vectortest/` and `dbtest/` test helper packages from coverage requirements
- Dockerfile now runs as non-root user
- Helm deployment sets `automountServiceAccountToken: false` and default resource limits
- All GitHub Actions pinned to full commit SHAs
- `structToMap` now uses `structpb.Struct.AsMap()` instead of a JSON round-trip
- pgvector Backend accepts a `DBTX` interface instead of `*pgxpool.Pool`, enabling mock-based testing
- pgvector `Ping` uses `Exec("SELECT 1")` instead of `pool.Ping` for interface compatibility
- AdminServer accepts a `Pinger` interface instead of `*pgxpool.Pool`
- Removed dead `Claims` error check in OIDC validator (verified tokens always have parseable claims)

### Fixed

- Missing `defer tx.Rollback()` in `dbtest.QueryCounter.Begin()`
- Removed dead error checks after `auth.GenerateAPIKey()` (always returns nil error)
- Reduced `coverage:ignore` annotations from 68 to 2 (only genuine infrastructure boundaries in pool.go and migrate.go)

## [0.1.2] - 2026-03-06

### Added

- Codecov coverage reporting with 100% coverage enforcement
- SonarCloud code quality and security scanning
- Snyk dependency vulnerability scanning
- Code quality badges in README

## [0.1.1] - 2026-03-06

### Added

- creel-chat: interactive REPL demo agent with Creel-backed conversation memory
- creel-chat: --resume flag for session continuity with document ID printed on exit
- Phase 1.5 checklist items in ARCHITECTURE.md
- Developer documentation: Quickstart, Concepts, Development, Deployment, and creel-chat guides
- CHANGELOG.md with retroactive v0.1.0 release history
- Release tooling: `scripts/make-tag` and GitHub Actions release workflow

### Changed

- creel-chat: default LLM models updated to Claude Sonnet 4.6 (Anthropic) and GPT-5.4 (OpenAI)

## [0.1.0] - 2026-03-06

### Added

- gRPC server with 28 RPCs across 7 services (Topic, Document, Chunk, Link, Retrieval, Compaction, Admin)
- PostgreSQL metadata store with golang-migrate migrations
- pgvector backend for embedding storage and similarity search
- Authentication: OIDC token validation and API key validation with key rotation/revocation
- Principal-based RBAC with topic grants (read/write/admin) for individuals and groups
- Topic > Document > Chunk hierarchy with full CRUD
- Batch chunk ingestion with pre-computed embeddings
- RAG search with ACL filtering and metadata filtering
- System account management (create, list, delete) with API key lifecycle
- CLI client (creel-cli) with health, admin, topic, and search commands
- Docker Compose development environment (PostgreSQL/pgvector)
- Multi-stage Dockerfile for server, CLI, and chat binaries
- Helm chart with embedded PostgreSQL (CloudNativePG) or external PostgreSQL support
- CI pipeline (GitHub Actions: lint, test, build)
- Protobuf codegen pipeline (buf)
