# Creel

Self-hosted memory-as-a-service for AI agents.

Creel provides topic-scoped memory with principal-based RBAC, Zettelkasten-style chunk-to-chunk linking across topic boundaries, and dual-mode retrieval (semantic search + temporal context). It runs in your infrastructure via Helm chart.

## Quick start

```bash
# Start PostgreSQL/pgvector
docker compose up -d postgres

# Build
make build

# Generate a bootstrap API key and create creel.yaml (see docs/QUICKSTART.md)
bin/creel bootstrap-key --name quickstart

# Start the server with migrations
bin/creel --config creel.yaml --migrate
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
| [API Reference](API_REFERENCE.md) | All 28 RPCs with request/response details |
| [Development](docs/DEVELOPMENT.md) | Dev environment, testing, adding new RPCs |
| [Deployment](docs/DEPLOYMENT.md) | Helm chart, configuration, OIDC setup |
| [creel-chat](docs/CREEL_CHAT.md) | Interactive demo agent with conversation memory |
| [Architecture](ARCHITECTURE.md) | Full design document and roadmap |

## Status

Early development (v0.1.0). Phase 1 is complete; see [ARCHITECTURE.md](ARCHITECTURE.md) for the roadmap and [CHANGELOG.md](CHANGELOG.md) for release history.

## License

Apache 2.0. See [LICENSE](LICENSE).
