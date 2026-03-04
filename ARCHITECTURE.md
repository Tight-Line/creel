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

- **Auth Proxy**: validates OIDC tokens, resolves principal identity and group memberships, attaches principal context to requests. Does not manage identities; delegates to external IdP.
- **Authorizer**: decides whether a principal can perform an action on a resource. Built-in implementation evaluates `TopicGrant` rows against both individual principal refs and group refs (resolved from OIDC `groups` claim). Pluggable interface allows delegation to external engines (SpiceDB, OpenFGA, OPA) in future.
- **Topic CRUD**: create/read/update/delete topics, documents, chunks. Manages sharing grants (principal + permission level per topic).
- **Link Engine**: creates/deletes chunk-to-chunk links. Handles compaction redirects (link targets that point to compacted chunks resolve to their summary chunk, recursively).
- **Retrieval Engine**: dual-mode retrieval. RAG mode: semantic similarity search with optional link traversal and reranking. Context mode: temporal ordering with compaction awareness. Both modes enforce ACLs.
- **Compactor API**: accepts client-provided summaries, tombstones old chunks, creates summary chunks, transfers outbound links from compacted chunks to summary.
- **Admin API**: system account management (create, rotate, revoke API keys), health checks, metrics.
- **CLI** (`creel`): command-line interface for admin operations and debugging. Single binary, authenticates via API key or OIDC token. Covers system account management, topic/grant administration, and diagnostic commands.
- **Event Bus**: internal pub/sub for async operations (auto-link suggestions on ingest, compaction notifications). Initially in-process; can be externalized (NATS, Redis Streams) for horizontal scaling.
- **Vector Backend Interface**: Go interface that any vector store must implement. Reference implementation: pgvector.

## 3. Data Model

### 3.1 Core Entities

**Principal** (external; not stored in Creel)

- Identity resolved from OIDC token (configurable claim: `sub`, `email`, or custom)
- Group memberships resolved from OIDC `groups` claim (claim name is configurable)
- Creel stores principal references in ACL grants, not principal records
- A principal ref is a typed string: `user:nick@example.com` or `group:ml-team`

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
  principal:   principal_ref (e.g. "user:nick@example.com" or "group:ml-team")
  permission:  enum(read, write, admin)
  granted_by:  principal_ref
  created_at:  timestamp
}
```

Grant resolution: a principal matches a grant if the grant's `principal` field matches either the principal's identity (`user:...`) or any of the principal's groups (`group:...`) as reported by the OIDC token's groups claim. The highest matching permission wins.

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

### 3.4 Authorizer Interface

```go
// Principal represents an authenticated identity with group memberships.
type Principal struct {
    ID     string   // e.g. "user:nick@example.com"
    Groups []string // e.g. ["group:ml-team", "group:engineering"]
}

type Action string

const (
    ActionRead  Action = "read"
    ActionWrite Action = "write"
    ActionAdmin Action = "admin"
)

// Authorizer decides whether a principal can perform an action on a resource.
type Authorizer interface {
    // Check returns true if the principal has at least the requested permission.
    Check(ctx context.Context, principal Principal, action Action, topicID string) (bool, error)

    // AccessibleTopics returns all topic IDs the principal can access at the given
    // permission level or higher. Used for list filtering and search scoping.
    AccessibleTopics(ctx context.Context, principal Principal, action Action) ([]string, error)
}
```

The built-in `GrantAuthorizer` evaluates `TopicGrant` rows: a principal matches a grant iff the grant targets the principal's ID or any of their groups. The interface is designed so that external authorization engines (SpiceDB, OpenFGA, OPA) can be plugged in later without changing the rest of the server.

## 4. Required Features for v1

### 4.1 Principal & Auth

- OIDC token validation (configurable issuer, audience, claims mapping)
- Principal identity extracted from token claims (sub, email, or custom claim; configurable)
- Group membership extracted from token claims (`groups` by default; claim name configurable)
- Grants can target individual principals (`user:...`) or groups (`group:...`)
- `Authorizer` interface with built-in implementation (TopicGrant table + group claim resolution)
- No built-in identity management; no built-in group management
- **No identity linking**: each (provider, principal_claim value) tuple is a distinct identity. If a person authenticates as `user:nmarden@avvo.com` via one IdP and `user:nick@marden.org` via another, Creel treats those as two separate principals. Identity federation (mapping multiple upstream identities to a single canonical principal) should be handled at the IdP layer, e.g. via Dex, Authelia, or IdP-to-IdP federation. Shared access across identities can also be achieved through group grants.

#### How authentication works at the wire level

Every request to Creel carries an `Authorization: Bearer <token>` header. Creel supports two token types:

**OIDC JWTs (for humans and OIDC-capable services):**
The caller obtains a JWT from their IdP through a standard OAuth2 flow (browser-based login, CLI device flow, client credentials grant, etc.). Creel never participates in this flow; it only validates the resulting token. At startup, Creel fetches each configured provider's public signing keys from their well-known OIDC discovery endpoint (`https://{issuer}/.well-known/openid-configuration`) and caches them, refreshing periodically. On each request, Creel validates the token's cryptographic signature against the cached keys, checks expiry and audience, and extracts the principal identity and group memberships from the token's claims. This is pure local computation with no per-request round-trip to the IdP. The tradeoff is that token revocation at the IdP is not instantaneous; Creel will accept a revoked token until it expires (typically 5-60 minutes depending on IdP configuration).

**API keys (for system accounts):**
System accounts are managed through the Admin API (create, rotate, revoke). Each system account has a name, a principal identity (e.g., `system:ingestion-pipeline`), and one or more API keys. Keys are stored as hashes in PostgreSQL. On each request, Creel recognizes the key by its prefix (`creel_ak_...`), verifies the hash, and resolves the associated system principal. No IdP is involved. Key rotation supports a configurable grace period where both old and new keys are valid. Revocation is immediate.

System accounts are first-class principals that receive topic grants like any other principal. A bootstrap API key can be configured in the config file to create the initial admin system account; after that, all system account management happens through the API.

#### System Account Management

The Admin API provides:

- Create system account (name, description); returns API key
- List system accounts
- Rotate key (generate new key; old key valid for configurable grace period)
- Revoke key (immediate)
- Delete system account

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

### 4.9 CLI

Single binary (`creel`) for admin operations and debugging. Connects to a Creel server over gRPC; authenticates via API key or OIDC token.

```
creel config set-endpoint https://creel.internal:8443
creel config set-key creel_ak_...

# System account management
creel admin create-account --name ingestion-pipeline --description "Nightly doc ingestion"
creel admin list-accounts
creel admin rotate-key --account ingestion-pipeline --grace 3600
creel admin revoke-key --account ingestion-pipeline

# Topic & grant management
creel topic create --slug ml-research --name "ML Research"
creel topic list
creel topic grant --topic ml-research --principal group:ml-team --permission write
creel topic grant --topic ml-research --principal system:ingestion-pipeline --permission write
creel topic grants --topic ml-research

# Diagnostics
creel health
creel search --topic ml-research --query "transformer architecture" --top-k 5
creel context --document ml-research/session-2026-03-04 --last 20
```

Configuration stored in `~/.creel/config.yaml`. Supports multiple named profiles for managing different Creel instances.

## 5. API Surface

### 5.1 gRPC (primary) + REST (via grpc-gateway)

gRPC as the primary protocol for performance and strong typing. REST via grpc-gateway for broad compatibility. Proto files are the source of truth for both.

For detailed method signatures, request/response shapes, permission requirements, and behavioral specifications for all 28 RPC methods, see [API_REFERENCE.md](API_REFERENCE.md).

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

**Backend dependencies** follow a two-mode pattern for every supported backend (PostgreSQL, Qdrant, Weaviate, etc.):

1. **Embedded**: install via subchart using the upstream/canonical Helm chart for that backend (e.g., `postgresql` from the CloudNativePG operator chart, `qdrant/qdrant` from Qdrant's official chart). No Bitnami charts; always prefer the vendor's own chart or a well-maintained community chart.
2. **External**: point to an existing instance via `values.yaml`. Connection details (host, port, credentials) reference a Kubernetes Secret. Example:

```yaml
postgresql:
  enabled: false           # do not install subchart
  external:
    host: my-pg.example.com
    port: 5432
    database: creel
    secretName: creel-pg-credentials   # keys: username, password

qdrant:
  enabled: false
  external:
    host: qdrant.example.com
    port: 6334
    apiKeySecretName: creel-qdrant-credentials  # key: api-key
```

As new vector backends are added, each must have both an embedded subchart option and an external connection option in `values.yaml`.

### 7.2 Configuration

```yaml
# creel.yaml
server:
  grpc_port: 8443
  rest_port: 8080
  metrics_port: 9090

auth:
  providers:                        # multiple IdPs supported
    - issuer: https://accounts.google.com
      audience: creel
    - issuer: https://login.microsoftonline.com/{tenant}/v2.0
      audience: creel
  principal_claim: email            # which token claim is the principal ID
  groups_claim: groups              # which token claim carries group memberships
  api_keys:                         # for service-to-service calls
    - name: my-service
      key_hash: sha256:...
      principal: user:my-service@system   # identity this key authenticates as

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

- `ghcr.io/tight-line/creel:latest` - server (also includes `creel-cli` binary)
- `ghcr.io/tight-line/creel-mcp:latest` - MCP server

Multi-arch (amd64, arm64).

### 7.4 Local Development (Docker Compose)

`docker-compose.yml` runs the Creel server plus **every supported backend**, so developers and CI can test against all of them without external dependencies.

**Standing rule**: when a new vector backend is added, it must be added to `docker-compose.yml` and to the CI integration test matrix before the backend is considered complete.

Baseline services:

- **PostgreSQL** (with pgvector extension); serves as both metadata store and pgvector backend
- **Qdrant**
- **Weaviate**

As backends are added (e.g., Milvus, Chroma), they get a new service block in Compose and a new entry in the CI test matrix. Each backend runs its conformance test suite as part of `make test-integration`.

Example structure:

```yaml
services:
  postgres:
    image: pgvector/pgvector:pg17
    ports: ["5432:5432"]
    environment:
      POSTGRES_DB: creel
      POSTGRES_USER: creel
      POSTGRES_PASSWORD: creel

  qdrant:
    image: qdrant/qdrant:latest
    ports: ["6333:6333", "6334:6334"]

  weaviate:
    image: cr.weaviate.io/semitechnologies/weaviate:latest
    ports: ["8081:8080", "50051:50051"]
    environment:
      AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED: "true"
      PERSISTENCE_DATA_PATH: /var/lib/weaviate

  creel:
    build:
      context: .
      dockerfile: deploy/docker/Dockerfile
    depends_on: [postgres, qdrant, weaviate]
    ports: ["8443:8443", "8080:8080", "9090:9090"]
    environment:
      CREEL_METADATA_POSTGRES_URL: postgres://creel:creel@postgres:5432/creel?sslmode=disable
```

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
│   ├── creel/
│   │   └── main.go             # server entrypoint
│   └── creel-cli/
│       └── main.go             # CLI entrypoint
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

## 10. v1 Build-out Checklist

### Phase 1: Foundation

- [x] CI/CD pipeline (GitHub Actions: lint, test, build)
- [x] PostgreSQL schema migrations (golang-migrate)
- [x] Protobuf codegen pipeline (buf or protoc)
- [x] Configuration loading (YAML + env vars)
- [x] Auth middleware: OIDC token validation (JWKS fetch + cache + periodic refresh)
- [x] Auth middleware: API key validation (hash lookup in PostgreSQL)
- [x] Principal context extraction (identity + groups from token claims)
- [x] Authorizer interface definition
- [x] Built-in GrantAuthorizer (TopicGrant table, individual + group matching)
- [x] System account management (create, list, delete)
- [x] API key lifecycle (rotate with grace period, revoke)
- [x] Bootstrap API key (config file, for initial admin setup)
- [x] Topic CRUD (create, get, list, update, delete)
- [x] Topic grants: individual principal grants (user:...)
- [x] Topic grants: group grants (group:...)
- [x] ACL enforcement on topic operations via Authorizer
- [x] Document CRUD (create, get, list, update, delete)
- [x] Chunk ingestion (single, with pre-computed embedding)
- [x] Chunk ingestion (batch, with pre-computed embeddings)
- [x] Vector backend interface definition
- [x] pgvector backend implementation
- [x] pgvector backend conformance tests
- [x] Basic RAG search (single topic, no link traversal)
- [x] ACL filtering in search (restrict to accessible topics)
- [ ] Metadata filtering in search results
- [x] Dockerfile (multi-stage, multi-arch)
- [x] Docker Compose with all current backends (PostgreSQL/pgvector)
- [x] Basic Helm chart (deployment, service, configmap)
- [x] Helm: PostgreSQL via canonical subchart (CloudNativePG or upstream; no Bitnami)
- [x] Helm: external PostgreSQL option (host, port, secretName in values.yaml)
- [x] Health endpoint
- [x] gRPC server wiring (all Phase 1 services)
- [x] CLI: project scaffold (cobra or similar)
- [x] CLI: config management (endpoint, API key, profiles)
- [x] CLI: `creel health`
- [x] CLI: `creel admin create-account / list-accounts / rotate-key / revoke-key`
- [x] CLI: `creel topic create / list / grant / grants`
- [x] CLI: `creel search` and `creel context` (basic)

### Phase 2: Linking & Traversal

- [ ] Link CRUD (create, delete, list outbound + backlinks)
- [ ] Link ACL enforcement (read on both endpoints' topics)
- [ ] Auto-link on ingest (similarity search across accessible topics)
- [ ] Configurable auto-link threshold
- [ ] Permission-gated link traversal in RAG search
- [ ] Configurable traversal depth
- [ ] Reranking pool with linked chunks
- [ ] Compaction redirects (links to compacted chunks resolve to summary)
- [ ] Recursive redirect resolution

### Phase 3: Context Mode & Compaction

- [ ] Context mode retrieval (temporal ordering by sequence)
- [ ] Configurable context window (last N chunks, since timestamp)
- [ ] Compaction API: accept summary + chunk range
- [ ] Summary chunk creation
- [ ] Chunk tombstoning (status=compacted, compacted_by=summary)
- [ ] Outbound link transfer from compacted chunks to summary
- [ ] Compaction-aware context retrieval (summaries + active chunks)
- [ ] Un-compact (admin restore)
- [ ] Archival access to compacted chunks

### Phase 4: Integration Layers

- [ ] grpc-gateway REST API
- [ ] Python client library (full API coverage)
- [ ] Python client: auth token handling, retries
- [ ] Python client: `compact_with_llm` convenience method
- [ ] TypeScript client library (full API coverage)
- [ ] TypeScript client: auth token handling, retries
- [ ] Tool schemas: OpenAI function calling format (JSON)
- [ ] Tool schemas: Anthropic tool_use format (JSON)
- [ ] Tool schemas: language-native objects in Python + TS clients
- [ ] MCP server (SSE transport)
- [ ] MCP server (stdio transport)
- [ ] MCP server Docker image

### Phase 5: Additional Backends & Hardening

- [ ] Embedding provider interface
- [ ] OpenAI embedding provider implementation
- [ ] Ollama embedding provider implementation
- [ ] Chunk ingestion without pre-computed embedding (server-side)
- [ ] Qdrant vector store backend
- [ ] Qdrant backend conformance tests
- [ ] Docker Compose: add Qdrant service
- [ ] CI: add Qdrant to integration test matrix
- [ ] Helm: Qdrant via canonical subchart (qdrant/qdrant)
- [ ] Helm: external Qdrant option (host, port, apiKeySecretName)
- [ ] Weaviate vector store backend
- [ ] Weaviate backend conformance tests
- [ ] Docker Compose: add Weaviate service
- [ ] CI: add Weaviate to integration test matrix
- [ ] Helm: Weaviate via canonical subchart (weaviate/weaviate)
- [ ] Helm: external Weaviate option (host, port, secretName)
- [ ] OpenAI vector store backend
- [ ] OpenAI backend conformance tests
- [ ] Prometheus metrics (request latency, chunk/link counts, backend latency)
- [ ] Helm chart hardening (HPA, PDB, network policies, secret refs)
- [ ] Helm chart: optional MCP sidecar
- [ ] Ingress configuration (gRPC + REST)
- [ ] Integration test suite (CRUD, search, links, compaction, ACLs)
- [ ] Vector backend conformance test harness (reusable across backends)

## 11. Verification

- **Unit tests**: each internal package has tests; vector backend interface has a conformance test suite that all implementations must pass
- **Integration tests**: Docker Compose setup with PostgreSQL + Creel server; tests cover full CRUD, search, link traversal, compaction, ACL enforcement
- **Client library tests**: each SDK has integration tests against a running Creel server
- **MCP conformance**: test MCP server against MCP inspector tool
- **Helm chart**: test install on kind (Kubernetes in Docker) cluster
- **CI**: GitHub Actions runs unit tests, integration tests, builds Docker images, lints proto files
