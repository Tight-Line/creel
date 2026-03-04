# Creel

Self-hosted memory-as-a-service for AI agents.

Creel provides topic-scoped memory with principal-based RBAC, Zettelkasten-style chunk-to-chunk linking across topic boundaries, and dual-mode retrieval (semantic search + temporal context). It runs in your infrastructure via Helm chart.

## Status

Early development. See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

## Key Features

- **Topic > Document > Chunk** hierarchy with RBAC
- **Cross-topic linking** with permission-gated traversal
- **Dual-mode retrieval**: RAG (semantic search) and context (temporal ordering)
- **Client-driven compaction** with link preservation
- **Pluggable vector backends**: pgvector (reference), OpenAI, Qdrant, Weaviate
- **Multi-tier integration**: client SDKs, tool schemas, MCP server

## Building

```bash
go build ./cmd/creel
```

## License

Apache 2.0. See [LICENSE](LICENSE).
