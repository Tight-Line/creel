# Creel

[![CI](https://github.com/Tight-Line/creel/actions/workflows/ci.yml/badge.svg)](https://github.com/Tight-Line/creel/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/Tight-Line/creel/graph/badge.svg?token=2TZQC2P4EC)](https://codecov.io/gh/Tight-Line/creel)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Tight-Line_creel&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Tight-Line_creel)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=Tight-Line_creel&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=Tight-Line_creel)
[![Snyk](https://snyk.io/test/github/Tight-Line/creel/badge.svg)](https://snyk.io/test/github/Tight-Line/creel)

Self-hosted memory-as-a-service for AI agents.

Creel provides topic-scoped memory with principal-based RBAC, Zettelkasten-style chunk-to-chunk linking across topic boundaries, and dual-mode retrieval (semantic search + temporal context). It runs in your infrastructure via Helm chart.

## Quick start

```bash
# Start PostgreSQL + Creel server
docker compose up -d

# Build the CLI tools
make build

# Source the dev environment (sets CREEL_ENDPOINT and CREEL_API_KEY)
source .env

# Create a topic
bin/creel-cli topic create my-notes "My Notes"
```

See [Quickstart](docs/QUICKSTART.md) for the full walkthrough.

## Key features

- **Topic > Document > Chunk** hierarchy with RBAC
- **Cross-topic linking** with permission-gated traversal
- **Dual-mode retrieval**: RAG (semantic search) and context (temporal ordering)
- **Client-driven compaction** with link preservation
- **Pluggable vector backends**: pgvector (reference), OpenAI, Qdrant, Weaviate
- **28 gRPC RPCs** across 7 services

## Documentation

| Guide | Description |
|-------|-------------|
| [Quickstart](docs/QUICKSTART.md) | End-to-end setup in under 5 minutes |
| [Concepts](docs/CONCEPTS.md) | Data model, auth, search modes, linking, compaction |
| [API Reference](docs/API_REFERENCE.md) | All 28 RPCs with request/response details |
| [Development](docs/DEVELOPMENT.md) | Dev environment, testing, adding new RPCs |
| [Deployment](docs/DEPLOYMENT.md) | Helm chart, configuration, OIDC setup |
| [creel-chat](docs/CREEL_CHAT.md) | Interactive demo agent with conversation memory |
| [Architecture](docs/ARCHITECTURE.md) | Full design document and roadmap |

## Status

Early development (v0.1.0). Phase 1 is complete; see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the roadmap and [CHANGELOG.md](CHANGELOG.md) for release history.

## License

Apache 2.0. See [LICENSE](LICENSE).
