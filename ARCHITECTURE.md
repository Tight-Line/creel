# Creel Architecture

Creel is a self-hosted memory-as-a-service platform for AI agents and applications.

## 1. Executive Summary

Creel provides:

- **Topic-scoped memory** with principal-based RBAC and cross-principal sharing
- **Three-level hierarchy**: Topic > Document > Chunk
- **Zettelkasten-style chunk-to-chunk linking** across topic boundaries, with permission-gated traversal
- **Dual-mode retrieval**: semantic search (RAG) and temporal ordering (session context)
- **Compaction**: client-driven summarization of conversation history with link preservation
- **Pluggable vector backends**: pgvector reference implementation; OpenAI, Qdrant, Weaviate, etc. via backend interface
- **Multi-tier integration**: client library (Tier 1), tool/function schemas (Tier 2), MCP server (Tier 3), duck-typed SDK wrappers (Tier 4, future)

Creel runs in your infrastructure. The primary distribution is a Helm chart. It is infrastructure, not a SaaS platform.

## 2. Component Overview

```
┌──────────────────────────────────────────────────────┐
│                    Creel Clients                     │
│  Python SDK  │  TypeScript SDK  │  Go SDK  │  .NET   │
├──────────────────────────────────────────────────────┤
│              Integration Layers                      │
│  MCP Server  │  Tool Schemas  │  Duck-typed Wrappers │
├──────────────────────────────────────────────────────┤
│                                                      │
│                  Creel Server (Go)                   │
│                                                      │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌────────────┐  │
│  │  Auth   │ │  Topic  │ │  Link   │ │ Retrieval  │  │
│  │  Proxy  │ │  CRUD   │ │  Engine │ │  Engine    │  │
│  └─────────┘ └─────────┘ └─────────┘ └────────────┘  │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐                 │
│  │Compactor│ │  Admin  │ │  Event  │                 │
│  │  API    │ │  API    │ │  Bus    │                 │
│  └─────────┘ └─────────┘ └─────────┘                 │
│                                                      │
├──────────────────────────────────────────────────────┤
│              Vector Backend Interface                │
│  pgvector  │  OpenAI  │  Qdrant  │  Weaviate  │ ...  │
├──────────────────────────────────────────────────────┤
│              Metadata Store (PostgreSQL)             │
│  Topics, Documents, Chunks (metadata), Links, ACLs   │
└──────────────────────────────────────────────────────┘
```

**Key architectural decision**: PostgreSQL is always required for metadata (topics, documents, chunk metadata, links, ACLs, compaction state). The vector backend is pluggable and stores embeddings + chunk content for similarity search. When pgvector is the backend, both metadata and vectors live in the same database.

### Components

- **Auth Proxy**: validates OIDC tokens, resolves principal identity, attaches principal context to requests. Does not manage identities; delegates to external IdP.
- **Topic CRUD**: create/read/update/delete topics, documents, chunks. Manages sharing grants (principal + permission level per topic).
- **Link Engine**: creates/deletes chunk-to-chunk links. Handles compaction redirects (link targets that point to compacted chunks resolve to their summary chunk, recursively).
- **Retrieval Engine**: dual-mode retrieval. RAG mode: semantic similarity search with optional link traversal and reranking. Context mode: temporal ordering with compaction awareness. Both modes enforce ACLs.
- **Compactor API**: accepts client-provided summaries, tombstones old chunks, creates summary chunks, transfers outbound links from compacted chunks to summary.
- **Admin API**: principal management (list topics, sharing grants), health checks, metrics.
- **Event Bus**: internal pub/sub for async operations (auto-link suggestions on ingest, compaction notifications). Initially in-process; can be externalized (NATS, Redis Streams) for horizontal scaling.
- **Vector Backend Interface**: Go interface that any vector store must implement. Reference implementation: pgvector.

## 3. Data Model

### 3.1 Core Entities

**Principal** (external; not stored in Creel)

- Identity resolved from OIDC token
- Creel stores principal references (subject claim) in ACL grants, not principal records

**Topic**

```
topic {
  id:          uuid
  slug:        string (unique, URL-safe)
  name:        string
  description: string
  owner:       principal_ref
  created_at:  timestamp
  updated_at:  timestamp
}
```

**TopicGrant** (ACL)

```
topic_grant {
  id:          uuid
  topic_id:    uuid -> topic
  principal:   principal_ref
  permission:  enum(read, write, admin)
  granted_by:  principal_ref
  created_at:  timestamp
}
```

Permission semantics:

- `read`: search chunks, follow links into this topic, retrieve documents
- `write`: read + ingest documents/chunks, create links from/to chunks in this topic, compact documents
- `admin`: write + manage grants, delete documents/chunks, delete topic

Topic owners implicitly have `admin`.

**Document**

```
document {
  id:          uuid
  topic_id:    uuid -> topic
  slug:        string (unique within topic)
  name:        string
  doc_type:    enum(reference, session, import, ...)
  metadata:    jsonb
  created_at:  timestamp
  updated_at:  timestamp
}
```

`doc_type` is informational; Creel doesn't change behavior based on it. Clients can use it to distinguish uploaded PDFs from conversation sessions.

**Chunk**

```
chunk {
  id:           uuid
  document_id:  uuid -> document
  sequence:     int (ordering within document)
  content:      text
  embedding_id: string (reference to vector in backend)
  status:       enum(active, compacted)
  compacted_by: uuid -> chunk (nullable; points to summary chunk)
  metadata:     jsonb (role, turn_number, source_page, etc.)
  created_at:   timestamp
}
```

**Link**

```
link {
  id:           uuid
  source_chunk: uuid -> chunk
  target_chunk: uuid -> chunk
  link_type:    enum(manual, auto, compaction_transfer)
  created_by:   principal_ref
  metadata:     jsonb (annotation, confidence score for auto-links, etc.)
  created_at:   timestamp
}
```

Link constraints:

- Source and target can be in different topics and different documents
- The creating principal must have `read` access to both the source and target topics at creation time
- Traversal requires the querying principal to have `read` access to both endpoints' topics
- Links to compacted chunks resolve to the summary chunk (recursively)
- Links are directional but discoverable in both directions (backlinks)

### 3.2 Addressing

Every chunk has a canonical address:

```
creel://{topic_slug}/{document_slug}/{chunk_id}
```

Links reference these addresses. The address provides full provenance without embedding principal identity.

### 3.3 Vector Backend Interface

```go
type VectorBackend interface {
    // Store a chunk's embedding
    Store(ctx context.Context, id string, embedding []float64, metadata map[string]any) error

    // Delete a chunk's embedding
    Delete(ctx context.Context, id string) error

    // Search by similarity; returns ordered chunk IDs with scores
    Search(ctx context.Context, query []float64, filter Filter, topK int) ([]SearchResult, error)

    // Batch operations
    StoreBatch(ctx context.Context, items []StoreItem) error
    DeleteBatch(ctx context.Context, ids []string) error

    // Health check
    Ping(ctx context.Context) error
}

type Filter struct {
    ChunkIDs []string        // restrict search to specific chunks (for ACL filtering)
    Metadata map[string]any  // metadata filters
}

type SearchResult struct {
    ChunkID string
    Score   float64
}
```

ACL enforcement happens in the Creel server, not the backend. The server resolves which topic IDs the principal can access, fetches the chunk IDs in those topics from PostgreSQL, and passes them as a filter to the vector backend. This keeps ACL logic centralized and backends simple.

## 4. Required Features for v1

### 4.1 Principal & Auth

- OIDC token validation (configurable issuer, audience, claims mapping)
- Principal identity extracted from token claims (sub, email, or custom claim; configurable)
- API key auth as fallback for service-to-service calls
- No built-in identity management

### 4.2 Topic Management

- CRUD operations on topics
- Sharing: grant/revoke read, write, admin per principal per topic
- List topics accessible to the current principal
- Topic metadata (name, description, custom JSON)

### 4.3 Document Management

- CRUD operations on documents within topics
- Document types (informational, client-assigned)
- List documents in a topic
- Document metadata

### 4.4 Chunk Ingestion

- Ingest chunks with content + pre-computed embedding
- Ingest chunks with content only (Creel computes embedding via configured embedding provider)
- Batch ingest
- Chunk metadata (role, turn number, source page, timestamps, custom JSON)
- Chunk ordering within a document (sequence number)

### 4.5 Linking

- Create links between chunks (manual)
- Auto-link suggestions on ingest: when a chunk is ingested, search for similar chunks in other topics the principal can access; create links above a configurable similarity threshold
- Delete links
- List links for a chunk (outbound and inbound/backlinks)
- Link metadata (annotation, confidence score)

### 4.6 Retrieval

**RAG mode (semantic search):**

- Query a topic (or set of topics) by semantic similarity
- Top-k results with scores
- Optional link traversal: for each result chunk, follow outbound links to chunks in other accessible topics; include linked chunks in reranking pool
- Configurable traversal depth (default 1; max configurable, recommended max 2)
- Permission-gated: only chunks in accessible topics are returned or traversed
- Metadata filtering on results

**Context mode (temporal retrieval):**

- Retrieve chunks from a specific document in sequence order
- Compaction-aware: returns summary chunks for compacted ranges + recent active chunks
- Configurable window (last N chunks, or since timestamp)

### 4.7 Compaction

- Client submits: document ID + summary content + summary embedding (optional) + range of chunks to compact
- Server creates summary chunk, tombstones specified chunks (status=compacted, compacted_by=summary), transfers outbound links to summary chunk
- Reversible: admin can un-compact (restore chunk status, remove summary)
- Compacted chunks remain in metadata store for archival queries

### 4.8 Administration

- Health endpoint
- Prometheus metrics (request latency, chunk counts, link counts, backend latency)
- Configuration via environment variables and config file
- Embedding provider configuration (for server-side embedding; optional; supports OpenAI, Ollama, etc. via interface)

## 5. API Surface

### 5.1 gRPC (primary) + REST (via grpc-gateway)

gRPC as the primary protocol for performance and strong typing. REST via grpc-gateway for broad compatibility. Proto files are the source of truth for both.

### 5.2 Service Definitions

```protobuf
service TopicService {
  rpc CreateTopic(CreateTopicRequest) returns (Topic);
  rpc GetTopic(GetTopicRequest) returns (Topic);
  rpc ListTopics(ListTopicsRequest) returns (ListTopicsResponse);
  rpc UpdateTopic(UpdateTopicRequest) returns (Topic);
  rpc DeleteTopic(DeleteTopicRequest) returns (Empty);
  rpc GrantAccess(GrantAccessRequest) returns (TopicGrant);
  rpc RevokeAccess(RevokeAccessRequest) returns (Empty);
  rpc ListGrants(ListGrantsRequest) returns (ListGrantsResponse);
}

service DocumentService {
  rpc CreateDocument(CreateDocumentRequest) returns (Document);
  rpc GetDocument(GetDocumentRequest) returns (Document);
  rpc ListDocuments(ListDocumentsRequest) returns (ListDocumentsResponse);
  rpc UpdateDocument(UpdateDocumentRequest) returns (Document);
  rpc DeleteDocument(DeleteDocumentRequest) returns (Empty);
}

service ChunkService {
  rpc IngestChunks(IngestChunksRequest) returns (IngestChunksResponse);
  rpc GetChunk(GetChunkRequest) returns (Chunk);
  rpc DeleteChunk(DeleteChunkRequest) returns (Empty);
}

service LinkService {
  rpc CreateLink(CreateLinkRequest) returns (Link);
  rpc DeleteLink(DeleteLinkRequest) returns (Empty);
  rpc ListLinks(ListLinksRequest) returns (ListLinksResponse);
}

service RetrievalService {
  rpc Search(SearchRequest) returns (SearchResponse);
  rpc GetContext(GetContextRequest) returns (GetContextResponse);
}

service CompactionService {
  rpc Compact(CompactRequest) returns (CompactResponse);
  rpc Uncompact(UncompactRequest) returns (UncompactResponse);
}
```

### 5.3 Key Request/Response Shapes

**SearchRequest (RAG mode):**

```
{
  topic_ids: [uuid]           // topics to search (must have read access)
  query_embedding: [float64]  // pre-computed, OR:
  query_text: string          // Creel computes embedding
  top_k: int
  follow_links: bool
  link_depth: int             // default 1
  metadata_filter: Filter
}
```

**SearchResponse:**

```
{
  results: [{
    chunk: Chunk
    document: DocumentRef
    topic: TopicRef
    score: float64
    via_link: LinkRef (nullable; set if this result came from link traversal)
  }]
}
```

**GetContextRequest (context mode):**

```
{
  document_id: uuid
  last_n: int                 // last N active chunks
  since: timestamp            // alternative: chunks since this time
  include_summaries: bool     // include compaction summaries (default true)
}
```

**CompactRequest:**

```
{
  document_id: uuid
  chunk_ids: [uuid]           // chunks to compact
  summary_content: string     // client-generated summary
  summary_embedding: [float64] // optional; Creel computes if omitted
  summary_metadata: jsonb
}
```

## 6. Integration Layers

### 6.1 Client Libraries (Tier 1)

Thin, idiomatic wrappers around the gRPC API. Ship for:

- **Python** (primary; AI/ML ecosystem)
- **TypeScript/Node** (web, MCP ecosystem)
- **Go** (infrastructure, CLI tools)
- **.NET** (future; implementing Microsoft.Extensions.VectorData.Abstractions)

Each client library includes:

- Full API coverage (CRUD, search, context, compact, links)
- Connection management, retries, auth token handling
- Convenience methods (e.g., `compact_with_llm(doc_id, llm_callable)`)
- Pre-built tool schemas for the target language's AI frameworks

### 6.2 Tool/Function Schemas (Tier 2)

Shipped as JSON schema files and as language-native objects in each client library:

- OpenAI function calling format
- Anthropic tool_use format

Tools exposed:

- `creel_search` - semantic search with optional link traversal
- `creel_store` - ingest a chunk into a document
- `creel_get_context` - retrieve session context
- `creel_link` - create a link between chunks
- `creel_list_topics` - list accessible topics
- `creel_share_topic` - grant access to a principal
- `creel_create_topic` - create a new topic
- `creel_create_document` - create a new document in a topic

### 6.3 MCP Server (Tier 3)

Standalone MCP server process (or embedded in the Creel server) exposing the same tool set as Tier 2 over MCP transport. Supports:

- SSE transport (for Claude Desktop, Cursor, etc.)
- stdio transport (for CLI-based agents)

The MCP server is a thin adapter over the Tier 1 client library.

### 6.4 Context Propagation Spec

Define a `CreelContext` object for cross-tool identity and context propagation:

```json
{
  "creel_endpoint": "https://creel.internal:8443",
  "token": "ey...",
  "active_topics": ["topic-slug-1", "topic-slug-2"],
  "session_document": "creel://my-topic/session-2026-03-04"
}
```

Passed as a tool parameter, HTTP header (`X-Creel-Context`), or environment variable. Creel-aware tools use it; non-Creel-aware tools ignore it.

## 7. Deployment Architecture

### 7.1 Helm Chart

Single Helm chart installs:

- Creel server (Deployment, HPA-capable)
- PostgreSQL (optional; can point to external)
- MCP server sidecar (optional)
- ConfigMap for Creel config
- Secret references for OIDC config, API keys, vector backend credentials
- Ingress/Service for gRPC + REST

### 7.2 Configuration

```yaml
# creel.yaml
server:
  grpc_port: 8443
  rest_port: 8080
  metrics_port: 9090

auth:
  oidc_issuer: https://accounts.google.com
  oidc_audience: creel
  principal_claim: email
  api_keys:
    - name: my-service
      key_hash: sha256:...

metadata:
  postgres_url: postgres://...

vector_backend:
  type: pgvector
  config:
    postgres_url: postgres://...

embedding:
  provider: openai
  model: text-embedding-3-small
  api_key: sk-...

links:
  auto_link_on_ingest: true
  auto_link_threshold: 0.85
  max_traversal_depth: 2

compaction:
  retain_compacted_chunks: true
```

### 7.3 Docker Images

- `ghcr.io/tight-line/creel:latest` - server
- `ghcr.io/tight-line/creel-mcp:latest` - MCP server

Multi-arch (amd64, arm64).

## 8. Implementation Phases

**Phase 1: Foundation**

- Go module, project structure, CI/CD (GitHub Actions)
- PostgreSQL schema (migrations via golang-migrate)
- gRPC service definitions (protobuf)
- Auth middleware (OIDC + API key)
- Topic CRUD + grants
- Document CRUD
- Chunk ingestion (with pre-computed embeddings only)
- Vector backend interface + pgvector implementation
- Basic RAG search (single topic, no link traversal)
- Dockerfile, basic Helm chart

**Phase 2: Linking & Traversal**

- Link CRUD
- Auto-link on ingest
- Permission-gated link traversal in search
- Backlink discovery
- Compaction redirects for links

**Phase 3: Context Mode & Compaction**

- Context mode retrieval (temporal ordering)
- Compaction API
- Link transfer on compaction
- Archival access to compacted chunks

**Phase 4: Integration Layers**

- Python client library
- TypeScript client library
- Tool/function schemas (OpenAI + Anthropic formats)
- MCP server
- REST gateway (grpc-gateway)

**Phase 5: Additional Backends & Hardening**

- OpenAI vector store backend
- Qdrant backend
- Server-side embedding (embedding provider interface)
- Metrics, observability
- Helm chart production hardening (HPA, PDB, network policies)

**Phase 6: Advanced (post-v1)**

- Go client library
- .NET client library (with VectorData.Abstractions)
- Duck-typed SDK wrappers (Tier 4)
- Link analytics and visualization
- Event bus externalization (NATS)

## 9. Project Structure

```
creel/
├── ARCHITECTURE.md
├── README.md
├── LICENSE
├── go.mod
├── go.sum
├── cmd/
│   └── creel/
│       └── main.go
├── proto/
│   └── creel/
│       └── v1/
│           ├── topic.proto
│           ├── document.proto
│           ├── chunk.proto
│           ├── link.proto
│           ├── retrieval.proto
│           ├── compaction.proto
│           └── admin.proto
├── internal/
│   ├── auth/
│   ├── server/
│   ├── store/
│   ├── vector/
│   │   ├── backend.go
│   │   ├── pgvector/
│   │   ├── openai/
│   │   └── qdrant/
│   ├── link/
│   ├── retrieval/
│   ├── compaction/
│   └── config/
├── migrations/
├── deploy/
│   ├── docker/
│   │   └── Dockerfile
│   └── helm/
│       └── creel/
│           ├── Chart.yaml
│           ├── values.yaml
│           └── templates/
├── sdk/
│   ├── python/
│   │   └── creel/
│   ├── typescript/
│   │   └── src/
│   └── go/
│       └── creel/
├── mcp/
│   └── server.go
├── tools/
│   ├── openai/
│   └── anthropic/
└── docs/
```

## 10. Verification

- **Unit tests**: each internal package has tests; vector backend interface has a conformance test suite that all implementations must pass
- **Integration tests**: Docker Compose setup with PostgreSQL + Creel server; tests cover full CRUD, search, link traversal, compaction, ACL enforcement
- **Client library tests**: each SDK has integration tests against a running Creel server
- **MCP conformance**: test MCP server against MCP inspector tool
- **Helm chart**: test install on kind (Kubernetes in Docker) cluster
- **CI**: GitHub Actions runs unit tests, integration tests, builds Docker images, lints proto files
