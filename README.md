# Creel

[![CI](https://github.com/Tight-Line/creel/actions/workflows/ci.yml/badge.svg)](https://github.com/Tight-Line/creel/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/Tight-Line/creel/graph/badge.svg?token=2TZQC2P4EC)](https://codecov.io/gh/Tight-Line/creel)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Tight-Line_creel&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Tight-Line_creel)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=Tight-Line_creel&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=Tight-Line_creel)
[![Snyk](https://snyk.io/test/github/Tight-Line/creel/badge.svg)](https://snyk.io/test/github/Tight-Line/creel)

Self-hosted memory-as-a-service for AI agents.

**v0.5.x**: CompactionService (sync/async compaction, undo, history), chunk linking (LinkService), managed document pipeline, semantic chunking, per-principal memory, dashboard, creel-chat.

Creel provides topic-scoped memory with principal-based RBAC, server-side document processing, per-principal memory (Mem0-style), and dual-mode retrieval (semantic search with citations + temporal context). Upload documents and Creel handles extraction, chunking, and embedding automatically. It runs in your infrastructure via Helm chart.

## Quick start

```bash
# Start PostgreSQL + Creel server
docker compose up -d

# Build the CLI tools
make build

# Source the dev environment (sets CREEL_ENDPOINT and CREEL_API_KEY)
source .env

# Create a topic and upload a document
bin/creel-cli topic create --slug my-notes --name "My Notes"
bin/creel-cli upload --topic my-notes --file notes.txt --name "Meeting Notes"

# Check processing status
bin/creel-cli jobs list --topic my-notes

# Search (once processing completes)
bin/creel-cli search --topic my-notes --query "action items" --top-k 5
```

See [Quickstart](docs/QUICKSTART.md) for the full walkthrough.

## Key features

- **Upload and forget**: upload PDFs, HTML, or plain text; Creel extracts text, chunks it (fixed-size or LLM-based semantic), and embeds in the background
- **Document citations**: search results include document metadata (title, author, URL, date) for proper attribution
- **Per-principal memory**: automatic fact extraction from conversations with Mem0-style conflict resolution (ADD/UPDATE/DELETE/NOOP)
- **Topic > Document > Chunk** hierarchy with RBAC and cross-topic linking
- **Dual-mode retrieval**: RAG (semantic search) and context (temporal ordering)
- **Server-driven compaction** with link preservation
- **Pluggable vector backends**: pgvector (reference), OpenAI, Qdrant, Weaviate
- **63 gRPC/REST RPCs** across 10 services
- **Two ingestion paths**: managed (upload and forget) and direct (pre-chunked, pre-embedded) for power users
- **Admin dashboard**: Laravel-based web UI for config, topic, system account, and memory management
- **creel-chat**: interactive demo agent with streaming, memory, cross-topic RAG, and citation display

## Documentation

| Guide | Description |
|-------|-------------|
| [Quickstart](docs/QUICKSTART.md) | End-to-end setup in under 5 minutes |
| [Fullstart](docs/FULLSTART.md) | Hands-on walkthrough of every feature |
| [Concepts](docs/CONCEPTS.md) | Data model, auth, search modes, memory, document processing |
| [API Reference](docs/API_REFERENCE.md) | All 63 RPCs with request/response details |
| [Development](docs/DEVELOPMENT.md) | Dev environment, testing, adding RPCs and workers |
| [Deployment](docs/DEPLOYMENT.md) | Helm chart, configuration, OIDC setup |
| [creel-chat](docs/CREEL_CHAT.md) | Interactive demo agent with conversation memory |
| [Architecture](docs/ARCHITECTURE.md) | Full design document and roadmap |

## Status

Active development (v0.4.x). Phases 1-5 complete plus PDF extraction, semantic chunking, dashboard memory browser, and chunk linking (LinkService). Phases 6, 8-9 (integration layers, compaction, additional backends) are next. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the roadmap and [CHANGELOG.md](CHANGELOG.md) for release history.

## License

Apache 2.0. See [LICENSE](LICENSE).
