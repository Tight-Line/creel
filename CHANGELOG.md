# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
