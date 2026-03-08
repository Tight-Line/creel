# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

### Changed

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
