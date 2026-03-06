# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
