# Creel Roadmap

This document tracks the implementation phases for Creel's v1 build-out. Each phase lists its scope and completion status. For architectural details, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Completed Phases

### Phase 1: Foundation (v0.1.0)

Go module, project structure, CI/CD (GitHub Actions), PostgreSQL schema (migrations via golang-migrate), gRPC service definitions (protobuf) + REST via grpc-gateway, auth middleware (OIDC + API key), topic CRUD + grants, document CRUD, chunk ingestion (batch, with pre-computed embeddings), vector backend interface + pgvector implementation, basic RAG search (ACL filtering, metadata filtering), context mode retrieval, Dockerfile, Docker Compose, Helm chart, CLI (`creel-cli`), creel-chat demo agent, server-side config registry, Laravel admin dashboard, dev workflow (Air live-reload, pre-configured dev key).

### Phase 2: Document Metadata & Citations (v0.2.0)

Structured citation fields on documents (name, URL, author, published_at, custom JSONB). `SearchResponse` includes `DocumentCitation` per result. creel-chat displays citations. CLI and dashboard updated.

### Phase 3: Server-Side Document Processing (v0.3.0)

Worker pool infrastructure, `ProcessingJob` table, `UploadDocument` RPC, extraction workers (PDF, HTML, plain text, URL fetch), chunking workers (fixed-size + semantic via LLM), embedding workers (OpenAI, dynamically resolved from DB config), document status lifecycle, `JobService` RPCs, CLI upload and job commands, dashboard processing views.

### Phase 4: Memory System (v0.4.0)

`Memory` table, memory store, memory maintenance workers (ADD/UPDATE/DELETE/NOOP conflict resolution via LLM), soft-delete with audit trail, per-principal scoping with named scopes, `MemoryService` RPCs, configurable extraction/maintenance prompts, CLI memory commands, dashboard memory browser.

### Phase 5: creel-chat Enhancements (v0.5.0)

Streaming LLM responses, document upload flow (managed path), memory integration (fetch at session start, include in system prompt), cross-topic RAG retrieval, explicit memory commands (`/remember`, `/forget`).

### Phase 6: Integration Layers (v0.6.0)

Python client library, TypeScript client library, tool/function schemas (OpenAI + Anthropic formats), MCP server (SSE + stdio transport), MCP server Docker image.

Remaining:

- [ ] Python client: auth token handling, retries
- [ ] TypeScript client: auth token handling, retries
- [ ] Tool schemas: language-native objects in Python + TS clients

### Phase 7: Linking & Traversal (v0.6.0)

Link CRUD, link ACL enforcement, auto-link worker (similarity search on ingest), configurable auto-link threshold, compaction redirects.

Remaining:

- [ ] Permission-gated link traversal in RAG search
- [ ] Configurable traversal depth
- [ ] Reranking pool with linked chunks
- [ ] Recursive redirect resolution

### Phase 8: Server-Driven Compaction (v0.7.0)

Compaction worker (LLM-driven summaries), chunk tombstoning, outbound link transfer, un-compact (admin restore), manual `Compact` RPC.

Remaining:

- [ ] Compaction policy configuration per topic (age threshold, chunk count threshold)
- [ ] Compaction-aware context retrieval (summaries + active chunks)
- [ ] Archival access to compacted chunks

### Phase 9: Additional Backends & Hardening (v0.7.x)

Prometheus metrics (gRPC counters + latency histograms), Helm chart hardening (PDB, NetworkPolicy, ServiceMonitor, SecurityContext), optional MCP sidecar, VectorBackendConfig CRUD, per-topic `vector_backend_config_id`, vector backend registry with lazy initialization.

Remaining:

- [ ] Qdrant vector backend + conformance tests + Docker Compose + CI + Helm subchart
- [ ] Weaviate vector backend + conformance tests + Docker Compose + CI + Helm subchart
- [ ] OpenAI vector store backend + conformance tests
- [ ] Ingress configuration (gRPC + REST)

### Phase 10: Memory Redesign (v0.8.0)

Memory redesigned as per-principal behavior with automatic extraction from conversations. `memory_enabled` removed from topics. `SearchMemories` removed. `GetMemory` renamed to `GetMemories` with multi-scope filtering. `AddMessages` RPC for automatic fact extraction. `memory_messages` worker extracts facts via LLM and feeds them into maintenance pipeline. creel-chat automatically calls `AddMessages` after each turn; supports `--memory-read-scopes`. Extraction prompt tightened to only extract facts about the user.

## Upcoming Phases

### Phase 11: Scalability & Pagination

- [ ] Cursor-based pagination for unbounded list RPCs (`ListScopes`, `AccessibleTopics`, `ListTopics`, `ListMemories`, `ListDocuments`, etc.)
- [ ] Dashboard memory scope view: show principal + scope + memory count
- [ ] `GetJob` detail view for documentless jobs in dashboard

### Phase 12: Provider Endpoints

Add an optional `endpoint` field to LLM and embedding configs, enabling OpenAI-compatible APIs (Azure OpenAI, Ollama, vLLM, LiteLLM, Together AI, Groq, etc.). When NULL, providers use their default base URL; when set, they use the custom endpoint.

- [ ] Migration: add nullable `endpoint TEXT` to `llm_configs` and `embedding_configs`
- [ ] Store, proto, config server, provider constructors, dynamic providers in `main.go`
- [ ] Dashboard: endpoint field on LLM and embedding config forms
- [ ] SDKs, tool schemas, and docs updated

### Phase 13: Medical Journal Demo

Web UI demonstrating Creel's full value proposition for Internet Brands: RAG with automatic memory. A doctor searches medical journals; Creel remembers they're an oncologist and tailors future results. All powered by standard SDK calls with no special memory plumbing in the client.

- [ ] Medical journal topics with automatically downloaded papers (PubMed open access)
- [ ] Web chat UI with turn-based conversation
- [ ] RAG search with proper citations for medical professionals
- [ ] Automatic memory extraction from conversation turns
- [ ] Standard Creel SDK integration; memory emerges as a side effect

## Post-v1

- Go client library
- .NET client library (with VectorData.Abstractions)
- Duck-typed SDK wrappers (Tier 4)
- Link analytics and visualization
- Event bus externalization (NATS)
- MCP SSE transport for Kubernetes sidecar
- Helm-level embedding bootstrap
