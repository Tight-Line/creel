# Creel Architecture

Creel is a self-hosted memory-as-a-service platform for AI agents and applications.

## Table of Contents

- [1. Executive Summary](#1-executive-summary)
- [2. Component Overview](#2-component-overview)
- [3. Data Model](#3-data-model)
- [4. Required Features for v1](#4-required-features-for-v1)
- [5. API Surface](#5-api-surface)
- [6. Integration Layers](#6-integration-layers)
- [7. Deployment Architecture](#7-deployment-architecture)
- [8. Implementation Phases](#8-implementation-phases)
- [9. Project Structure](#9-project-structure)
- [10. v1 Build-out Checklist](#10-v1-build-out-checklist)
- [11. Verification](#11-verification)

## 1. Executive Summary

Creel provides:

- **Topic-scoped memory** with principal-based RBAC and cross-principal sharing
- **Three-level hierarchy**: Topic > Document > Chunk
- **Server-side document processing**: upload a file or URL; Creel extracts, chunks, embeds, and indexes it in the background using topic-level configs
- **Document metadata and citations**: documents carry structured metadata (name, URL, author, date) that is joined with chunk results in all RAG searches, enabling proper citation in generated text
- **Per-principal memory**: automatic extraction of facts, preferences, and knowledge from conversations; maintained by background workers using Mem0-style ADD/UPDATE/DELETE/NOOP conflict resolution; served to clients for inclusion in system prompts
- **Dual-mode retrieval**: semantic search (RAG) with document citation metadata, and temporal ordering (session context)
- **Zettelkasten-style chunk-to-chunk linking** across topic boundaries, with permission-gated traversal
- **Compaction**: server-driven summarization of conversation history with link preservation
- **Pluggable vector backends**: pgvector reference implementation; OpenAI, Qdrant, Weaviate, etc. via backend interface
- **Two ingestion paths**: managed (upload and forget; server handles everything) and direct (power users push pre-chunked, pre-embedded content)
- **Multi-tier integration**: client library (Tier 1), tool/function schemas (Tier 2), MCP server (Tier 3), duck-typed SDK wrappers (Tier 4, future)

Creel runs in your infrastructure. The primary distribution is a Helm chart. It is infrastructure, not a SaaS platform.

### Target use cases

- **Domain-specific AI search**: index medical papers, legal libraries, or technical documentation in topic silos with per-user memory ("what is my specialization? what topics have I been asking about?")
- **Knowledge-augmented assistants**: index an organization's Google Drive, email, and internal docs; provide feedback and answers grounded in prior knowledge
- **Conversational agents with long-term memory**: chat agents that remember user preferences, context, and history across sessions without client-side memory management

## 2. Component Overview

```
+------------------------------------------------------+
|                    Creel Clients                      |
|  Python SDK  |  TypeScript SDK  |  Go SDK  |  .NET   |
+------------------------------------------------------+
|              Integration Layers                       |
|  MCP Server  |  Tool Schemas  |  Duck-typed Wrappers  |
+------------------------------------------------------+
|                                                       |
|                  Creel Server (Go)                    |
|                                                       |
|  +---------+ +---------+ +---------+ +-------------+  |
|  |  Auth   | |  Topic  | |  Link   | |  Retrieval  |  |
|  |  Proxy  | |  CRUD   | |  Engine | |  Engine     |  |
|  +---------+ +---------+ +---------+ +-------------+  |
|  +---------+ +---------+ +---------+ +-------------+  |
|  | Doc     | | Memory  | |  Admin  | |   Config    |  |
|  | Upload  | | Service | |  API    | |   Registry  |  |
|  +---------+ +---------+ +---------+ +-------------+  |
|                                                       |
|  +--------------------------------------------------+ |
|  |              Worker Pool                          | |
|  |  Extraction | Chunking | Embedding | Memory Maint | |
|  |  Compaction | Auto-Link                           | |
|  +--------------------------------------------------+ |
|                                                       |
+------------------------------------------------------+
|              Vector Backend Interface                 |
|  pgvector  |  OpenAI  |  Qdrant  |  Weaviate  | ...  |
+------------------------------------------------------+
|              Metadata Store (PostgreSQL)              |
|  Topics, Documents, Chunks, Links, ACLs, Configs,    |
|  Memories, Jobs                                      |
+------------------------------------------------------+
```

**Key architectural decision**: PostgreSQL is always required for metadata (topics, documents, chunk metadata, links, ACLs, processing configs, memories, jobs). The vector backend is pluggable and stores embeddings + chunk content for similarity search. When pgvector is the backend, both metadata and vectors live in the same database.

### Components

- **Auth Proxy**: validates OIDC tokens, resolves principal identity and group memberships, attaches principal context to requests. Does not manage identities; delegates to external IdP.
- **Authorizer**: decides whether a principal can perform an action on a resource. Built-in implementation evaluates `TopicGrant` rows against both individual principal refs and group refs (resolved from OIDC `groups` claim). Pluggable interface allows delegation to external engines (SpiceDB, OpenFGA, OPA) in future.
- **Topic CRUD**: create/read/update/delete topics, documents, chunks. Manages sharing grants (principal + permission level per topic).
- **Document Upload**: accepts raw files (PDF, HTML, plain text) or URLs for server-side processing. Creates a document record, enqueues a processing job, and returns immediately. The managed ingestion path.
- **Config Registry**: CRUD for LLM configs, embedding configs, extraction prompt configs, and API key configs. Topics bind to these configs to control how their documents are processed.
- **Link Engine**: creates/deletes chunk-to-chunk links. Handles compaction redirects (link targets that point to compacted chunks resolve to their summary chunk, recursively).
- **Retrieval Engine**: dual-mode retrieval. RAG mode: semantic similarity search with document citation metadata, optional link traversal, and reranking. Context mode: temporal ordering with compaction awareness. Both modes enforce ACLs.
- **Memory Service**: per-principal memory with named scopes. Serves current memory facts for inclusion in prompts. Exposes CRUD so clients can explicitly add, update, or delete memories.
- **Admin API**: system account management (create, rotate, revoke API keys), health checks, metrics, job status.
- **CLI** (`creel-cli`): command-line interface for admin operations and debugging. Single binary, authenticates via API key or OIDC token. Covers system account management, topic/grant administration, config management, job monitoring, and diagnostic commands.
- **Worker Pool**: background workers that process async jobs. Each worker type handles a pipeline stage:
  - **Extraction**: pulls text from uploaded documents (PDF, HTML, plain text, URLs)
  - **Chunking**: splits extracted text into chunks using the topic's configured strategy (size, overlap, semantic boundaries)
  - **Embedding**: computes vector embeddings for chunks using the topic's configured embedding provider/model
  - **Memory Maintenance**: extracts facts from new conversation chunks, resolves conflicts with existing memories (ADD/UPDATE/DELETE/NOOP)
  - **Compaction**: summarizes old conversation chunks into denser summaries, transfers links
  - **Auto-Link**: searches for similar chunks across accessible topics on ingest, creates links above threshold
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
  id:                        uuid
  slug:                      string (unique, URL-safe; auto-generated from name if omitted)
  name:                      string
  description:               string
  owner:                     principal_ref
  llm_config_id:             uuid -> llm_config (nullable; NULL = use default)
  embedding_config_id:       uuid -> embedding_config (nullable; NULL = use default)
  extraction_prompt_config_id: uuid -> extraction_prompt_config (nullable; NULL = use default)
  chunking_strategy:         jsonb (size, overlap, mode; nullable = server defaults)
  memory_enabled:            bool (default false; controls whether memory extraction runs for conversations in this topic)
  created_at:                timestamp
  updated_at:                timestamp
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

Grant resolution: a principal matches a grant iff the grant's `principal` field matches either the principal's identity (`user:...`) or any of the principal's groups (`group:...`) as reported by the OIDC token's groups claim. The highest matching permission wins.

Permission semantics:

- `read`: search chunks, follow links into this topic, retrieve documents
- `write`: read + upload documents, ingest chunks (direct path), create links from/to chunks in this topic
- `admin`: write + manage grants, delete documents/chunks, delete topic

Topic owners implicitly have `admin`.

**Document**

```
document {
  id:          uuid
  topic_id:    uuid -> topic
  slug:        string (unique within topic; auto-generated from name if omitted)
  name:        string
  doc_type:    enum(reference, session, import, memory, ...)
  url:         string (nullable; source URL for citations)
  author:      string (nullable; author name for citations)
  published_at: timestamp (nullable; publication date for citations)
  metadata:    jsonb (custom fields: journal name, volume, DOI, etc.)
  status:      enum(pending, processing, ready, failed)
  created_at:  timestamp
  updated_at:  timestamp
}
```

`status` tracks the document processing pipeline. Documents uploaded via the managed path start as `pending`, move through `processing`, and end at `ready` (or `failed`). Documents created via the direct ingestion path start as `ready`.

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

**Memory**

```
memory {
  id:           uuid
  principal:    principal_ref (e.g. "user:nick@example.com")
  scope:        string (e.g. "default", "work", "home")
  content:      text (natural language fact statement)
  embedding_id: string (reference to vector in backend)
  subject:      string (nullable; structured metadata for filtering)
  predicate:    string (nullable)
  object:       string (nullable)
  source_chunk_id: uuid -> chunk (nullable; the chunk that produced this memory)
  status:       enum(active, invalidated)
  invalidated_at: timestamp (nullable; for audit trail)
  metadata:     jsonb (confidence, category, etc.)
  created_at:   timestamp
  updated_at:   timestamp
}
```

Memories are natural language fact statements (e.g., "User specializes in thrombosis research", "Prefers concise answers with citations"). All memories in a scope are returned to the client; the LLM decides what to do with new candidate facts (ADD/UPDATE/DELETE/NOOP) and handles compaction. Optional structured fields (subject/predicate/object) enable precise filtering. Invalidated memories are soft-deleted for audit trail.

**ProcessingJob**

```
processing_job {
  id:           uuid
  document_id:  uuid -> document (nullable; null for documentless jobs like memory_maintenance from AddMemory)
  job_type:     enum(extraction, chunking, embedding, memory_extraction, compaction, auto_link, memory_maintenance)
  status:       enum(queued, running, completed, failed)
  progress:     jsonb (stage-specific progress info; documentless jobs store "principal" here for authorization)
  error:        text (nullable; error message on failure)
  started_at:   timestamp (nullable)
  completed_at: timestamp (nullable)
  created_at:   timestamp
}
```

### 3.2 Configuration Entities

These entities are managed via the ConfigService and bound to topics.

**LLMConfig**

```
llm_config {
  id:           uuid
  name:         string (unique)
  provider:     string (e.g. "openai", "anthropic", "ollama")
  model:        string
  api_key_config_id: uuid -> api_key_config (nullable)
  parameters:   jsonb (temperature, max_tokens, etc.)
  is_default:   bool
  created_at:   timestamp
  updated_at:   timestamp
}
```

**EmbeddingConfig**

```
embedding_config {
  id:           uuid
  name:         string (unique)
  provider:     string (e.g. "openai", "ollama")
  model:        string
  dimensions:   int
  api_key_config_id: uuid -> api_key_config (nullable)
  is_default:   bool
  created_at:   timestamp
  updated_at:   timestamp
}
```

**ExtractionPromptConfig**

```
extraction_prompt_config {
  id:           uuid
  name:         string (unique)
  prompt_type:  enum(extraction, chunking, memory_extraction, memory_maintenance, compaction)
  system_prompt: text
  llm_config_id: uuid -> llm_config (required)
  is_default:   bool
  created_at:   timestamp
  updated_at:   timestamp
}
```

**APIKeyConfig**

```
api_key_config {
  id:           uuid
  name:         string (unique)
  provider:     string (e.g. "openai", "anthropic")
  encrypted_key: bytea (AES-256-GCM encrypted)
  created_at:   timestamp
  updated_at:   timestamp
}
```

**RerankConfig**

```
rerank_config {
  id:                uuid
  name:              string (unique)
  provider:          string (e.g. "cohere", "jina", "voyageai", "llm")
  model:             string (e.g. "rerank-v3.5", "jina-reranker-v2-base-multilingual")
  api_key_config_id: uuid -> api_key_config (nullable; not needed for "llm" provider)
  llm_config_id:     uuid -> llm_config (nullable; required when provider is "llm")
  top_n_candidates:  int (default over-fetch factor; e.g. 50)
  is_default:        bool
  created_at:        timestamp
  updated_at:        timestamp
}
```

When `provider` is `llm`, the reranker uses the referenced LLM config as a cross-encoder (relevance scoring via prompt). This is slower but requires no additional service.

### 3.3 Addressing

Every chunk has a canonical address:

```
creel://{topic_slug}/{document_slug}/{chunk_id}
```

Links reference these addresses. The address provides full provenance without embedding principal identity.

### 3.4 Vector Backend Interface

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

### 3.5 Authorizer Interface

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
- Topic processing config bindings (LLM, embedding, extraction prompt, chunking strategy)

### 4.3 Document Management

- CRUD operations on documents within topics
- Document types (informational, client-assigned)
- Structured citation metadata: name, URL, author, published date, custom JSONB
- Document processing status tracking (pending, processing, ready, failed)
- List documents in a topic (filterable by status)

### 4.4 Document Upload & Processing (Managed Path)

The default ingestion path. Clients upload a document; Creel handles everything else.

- **Upload endpoint**: accepts a file (PDF, HTML, plain text) or URL; creates a document record with citation metadata; enqueues a processing job; returns the document ID and job ID immediately
- **Extraction workers**: pull text from uploaded files using format-appropriate extractors. Strategy is configurable per-topic via extraction prompt config.
- **Chunking workers**: split extracted text into chunks. Strategy is configurable per-topic (size, overlap, semantic boundaries via LLM).
- **Embedding workers**: compute vector embeddings for each chunk using the topic's configured embedding provider and model.
- **Job tracking**: each processing stage creates a job record. Status is visible via the Admin API, CLI (`creel-cli jobs list`), and dashboard.

### 4.5 Direct Chunk Ingestion (Power User Path)

Preserved for users who have their own extraction/chunking/embedding pipeline.

- Ingest chunks with content + pre-computed embedding
- Batch ingest
- Chunk metadata (role, turn number, source page, timestamps, custom JSON)
- Chunk ordering within a document (sequence number)
- Skips the worker pipeline entirely; document status is immediately `ready`

### 4.6 Memory System

Per-principal memory with named scopes. Inspired by Mem0's architecture.

**Memory scoping:**

- Memory belongs to a principal, not a topic. RAG topics (e.g., "British Journal of Hematology") don't accumulate useless memory documents.
- Each principal can have multiple named scopes: `default`, `work`, `home`, `volunteer`, etc.
- Topics with `memory_enabled = true` trigger memory extraction when new conversation chunks are ingested.
- Workers route extracted memories to the appropriate scope based on topic hints or explicit client specification.

**Memory extraction (two-phase, Mem0-style):**

1. LLM extracts candidate facts from new conversation chunks. The extraction prompt controls what categories matter (preferences, personal details, professional context, etc.). Few-shot examples guide the output format.
2. For each candidate fact, embed it and vector-search existing memories in the same scope. Then prompt the LLM with the candidate + existing similar memories to decide: ADD (new fact), UPDATE (merge with existing), DELETE (contradicts existing), or NOOP (already known).

**Memory maintenance:**

- On UPDATE, old and new facts are merged into a single statement.
- On DELETE, the contradicted memory is soft-deleted (status=invalidated, timestamp preserved) for audit trail.
- Memory extraction prompts are configurable via ExtractionPromptConfig.

**Memory retrieval:**

- Clients request memory by principal and scope name.
- Returns all active facts in the scope, suitable for injection into a system prompt.
- Semantic search within memory is also available for targeted retrieval.

**Explicit memory management:**

- `AddMemory` queues a `memory_maintenance` job instead of inserting directly. The maintenance worker runs LLM-based deduplication (ADD/UPDATE/DELETE/NOOP) against existing memories before storing. This ensures that both automatic extraction and explicit adds go through the same conflict resolution pipeline.
- Clients can also update or delete specific memories via the Memory API.
- "Forget X" in a conversation can trigger explicit deletion if the client supports it.

### 4.7 Retrieval

**RAG mode (semantic search):**

- Query a topic (or set of topics) by semantic similarity
- Top-k results with scores
- **Document citation metadata**: every search result includes the parent document's structured metadata (name, URL, author, published date, custom fields) alongside the chunk content. This enables LLMs to generate properly cited responses without additional lookups.
- Optional link traversal: for each result chunk, follow outbound links to chunks in other accessible topics; include linked chunks in reranking pool
- Configurable traversal depth (default 1; max configurable, recommended max 2)
- Permission-gated: only chunks in accessible topics are returned or traversed
- Metadata filtering on results
- **Reranking**: optional second-stage scoring via a cross-encoder reranker (Cohere, Jina, VoyageAI, or LLM-based). Vector search over-fetches candidates (e.g., 50), the reranker re-scores them, and the final `top_k` are returned. Configurable via `RerankConfig` in the config registry; overridable per request.

**Context mode (temporal retrieval):**

- Retrieve chunks from a specific document in sequence order
- Compaction-aware: returns summary chunks for compacted ranges + recent active chunks
- Configurable window (last N chunks, or since timestamp)

### 4.8 Linking

- Create links between chunks (manual)
- Auto-link suggestions on ingest: when a chunk is ingested, search for similar chunks in other topics the principal can access; create links above a configurable similarity threshold
- Delete links
- List links for a chunk (outbound and inbound/backlinks)
- Link metadata (annotation, confidence score)

### 4.9 Compaction

Server-driven compaction via background workers. Topics can be configured with compaction policies (e.g., compact session documents older than N days, or when chunk count exceeds a threshold). Workers use the topic's LLM config to generate summaries.

- Workers create summary chunks, tombstone old chunks (status=compacted, compacted_by=summary), transfer outbound links to summary chunk
- Reversible: admin can un-compact (restore chunk status, remove summary)
- Compacted chunks remain in metadata store for archival queries
- Manual compaction via API is also supported for the direct path

### 4.10 Administration

- Health endpoint
- Prometheus metrics (request latency, chunk counts, link counts, backend latency)
- Configuration via environment variables and config file
- Config registry for LLM, embedding, extraction prompt, and API key configs
- Job monitoring (list, status, progress) via API, CLI, and dashboard
- Laravel admin dashboard for config, topic, and system account management

### 4.11 CLI

Single binary (`creel-cli`) for admin operations and debugging. Connects to a Creel server over gRPC; authenticates via API key or OIDC token.

```
creel-cli config set-endpoint https://creel.internal:8443
creel-cli config set-key creel_ak_...

# System account management
creel-cli admin create-account --name ingestion-pipeline --description "Nightly doc ingestion"
creel-cli admin list-accounts
creel-cli admin rotate-key --account ingestion-pipeline --grace 3600
creel-cli admin revoke-key --account ingestion-pipeline

# Topic & grant management
creel-cli topic create --slug ml-research --name "ML Research"
creel-cli topic list
creel-cli topic grant --topic ml-research --principal group:ml-team --permission write
creel-cli topic grants --topic ml-research

# Config management
creel-cli config llm create --name gpt4 --provider openai --model gpt-4o
creel-cli config embedding create --name embed-small --provider openai --model text-embedding-3-small
creel-cli topic update --slug ml-research --embedding-config embed-small

# Document upload (managed path)
creel-cli upload --topic ml-research --file paper.pdf --author "Smith et al." --name "Thrombosis Study 2026"

# Job monitoring
creel-cli jobs list --topic ml-research
creel-cli jobs status <job-id>

# Diagnostics
creel-cli health
creel-cli search --topic ml-research --query "transformer architecture" --top-k 5
creel-cli context --document ml-research/session-2026-03-04 --last 20
```

Configuration stored in `~/.creel/config.yaml`. Supports multiple named profiles for managing different Creel instances.

## 5. API Surface

### 5.1 gRPC (primary) + REST (via grpc-gateway)

gRPC as the primary protocol for performance and strong typing. REST via grpc-gateway for broad compatibility. Proto files are the source of truth for both.

For detailed method signatures, request/response shapes, permission requirements, and behavioral specifications, see [API_REFERENCE.md](API_REFERENCE.md).

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
  rpc UploadDocument(UploadDocumentRequest) returns (UploadDocumentResponse);
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

service MemoryService {
  rpc GetMemory(GetMemoryRequest) returns (GetMemoryResponse);
  rpc SearchMemories(SearchMemoriesRequest) returns (SearchMemoriesResponse);
  rpc AddMemory(AddMemoryRequest) returns (AddMemoryResponse);
  rpc UpdateMemory(UpdateMemoryRequest) returns (Memory);
  rpc DeleteMemory(DeleteMemoryRequest) returns (DeleteMemoryResponse);
  rpc ListMemories(ListMemoriesRequest) returns (ListMemoriesResponse);
  rpc ListScopes(ListScopesRequest) returns (ListScopesResponse);
}

service JobService {
  rpc GetJob(GetJobRequest) returns (ProcessingJob);
  rpc ListJobs(ListJobsRequest) returns (ListJobsResponse);
}

service AdminService {
  rpc Health(HealthRequest) returns (HealthResponse);
  rpc CreateSystemAccount(CreateSystemAccountRequest) returns (CreateSystemAccountResponse);
  rpc ListSystemAccounts(ListSystemAccountsRequest) returns (ListSystemAccountsResponse);
  rpc DeleteSystemAccount(DeleteSystemAccountRequest) returns (DeleteSystemAccountResponse);
  rpc RotateKey(RotateKeyRequest) returns (RotateKeyResponse);
  rpc RevokeKey(RevokeKeyRequest) returns (RevokeKeyResponse);
}

service ConfigService {
  // 30 RPCs: CRUD + SetDefault for LLMConfig, EmbeddingConfig,
  // ExtractionPromptConfig, APIKeyConfig, and RerankConfig
}
```

### 5.3 Key Request/Response Shapes

**UploadDocumentRequest (managed path):**

```
{
  topic_id: uuid
  slug: string
  name: string                  // document title (for citations)
  url: string                   // source URL (for citations; nullable)
  author: string                // author name (for citations; nullable)
  published_at: timestamp       // publication date (for citations; nullable)
  metadata: jsonb               // custom citation fields (journal, DOI, etc.)
  file: bytes                   // the raw file content, OR:
  source_url: string            // URL to fetch the document from
  doc_type: string              // informational type label
}
```

**UploadDocumentResponse:**

```
{
  document: Document
  job_id: uuid                  // track processing status
}
```

**SearchRequest (RAG mode):**

```
{
  topic_ids: [uuid]             // topics to search (must have read access)
  query_embedding: [float64]    // pre-computed, OR:
  query_text: string            // Creel computes embedding
  top_k: int
  follow_links: bool
  link_depth: int               // default 1
  metadata_filter: Filter
  exclude_document_ids: [uuid]  // omit chunks from these documents
  rerank: bool                  // enable reranking (default: true if default RerankConfig exists)
  rerank_config_id: uuid        // override reranker (nullable; uses default if omitted)
  rerank_candidates: int        // override candidate pool size (nullable; falls back to config, then top_k)
}
```

**SearchResponse:**

```
{
  results: [{
    chunk: Chunk
    document: DocumentCitation {
      id: uuid
      slug: string
      name: string
      url: string
      author: string
      published_at: timestamp
      metadata: jsonb
    }
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
  last_n: int                   // last N active chunks
  since: timestamp              // alternative: chunks since this time
  include_summaries: bool       // include compaction summaries (default true)
}
```

**GetMemoryRequest:**

```
{
  scope: string                 // e.g. "default", "work" (required)
}
```

**GetMemoryResponse:**

```
{
  memories: [{
    id: uuid
    content: string             // natural language fact
    subject: string
    predicate: string
    object: string
    metadata: jsonb
    created_at: timestamp
    updated_at: timestamp
  }]
}
```

The response is scoped to the calling principal automatically. No need to pass a principal ID.

**CompactRequest:**

```
{
  document_id: uuid
  chunk_ids: [uuid]             // chunks to compact
  summary_content: string       // client-generated summary
  summary_embedding: [float64]  // optional; Creel computes if omitted
  summary_metadata: jsonb
}
```

## 6. Integration Layers

### 6.1 Client Libraries (Tier 1)

Thin, idiomatic wrappers around the gRPC API. Clients upload documents and retrieve results; they do not need to know about chunking, embedding, or compaction. Ship for:

- **Python** (primary; AI/ML ecosystem)
- **TypeScript/Node** (web, MCP ecosystem)
- **Go** (infrastructure, CLI tools)
- **.NET** (future; implementing Microsoft.Extensions.VectorData.Abstractions)

Each client library includes:

- Full API coverage (upload, search, context, memory, links)
- Connection management, retries, auth token handling
- Convenience methods (e.g., `upload_and_wait(topic, file, metadata)`)
- Pre-built tool schemas for the target language's AI frameworks

### 6.2 Tool/Function Schemas (Tier 2)

Shipped as JSON schema files and as language-native objects in each client library:

- OpenAI function calling format
- Anthropic tool_use format

Tools exposed:

- `creel_search` - semantic search with optional link traversal; results include citation metadata
- `creel_upload` - upload a document to a topic
- `creel_get_context` - retrieve session context
- `creel_get_memory` - retrieve memory for the current principal/scope
- `creel_remember` - explicitly add a memory
- `creel_forget` - explicitly delete a memory
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
  "session_document": "creel://my-topic/session-2026-03-04",
  "memory_scope": "work"
}
```

Passed as a tool parameter, HTTP header (`X-Creel-Context`), or environment variable. Creel-aware tools use it; non-Creel-aware tools ignore it.

## 7. Deployment Architecture

### 7.1 Helm Chart

Single Helm chart installs:

- Creel server (Deployment, HPA-capable)
- Worker pool (Deployment; can scale independently of the server)
- PostgreSQL (optional; can point to external)
- MCP server sidecar (optional)
- Dashboard (optional; Laravel admin UI)
- ConfigMap for Creel config
- Secret references for OIDC config, API keys, vector backend credentials, LLM provider keys
- Ingress/Service for gRPC + REST

**Backend dependencies** follow a two-mode pattern for every supported backend (PostgreSQL, Qdrant, Weaviate, etc.):

1. **Embedded**: install via subchart using the upstream/canonical Helm chart for that backend. No Bitnami charts; always prefer the vendor's own chart or a well-maintained community chart.
2. **External**: point to an existing instance via `values.yaml`. Connection details (host, port, credentials) reference a Kubernetes Secret. Example:

```yaml
postgresql:
  source: external
  host: my-pg.example.com
  port: 5432
  name: creel
  auth:
    username: creel
    existingSecret: creel-pg-credentials
    secretKey: password

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
      principal: user:my-service@system

postgres:
  host: localhost
  port: 5432
  user: creel
  password: creel
  name: creel
  schema: creel
  sslmode: disable

vector_backend:
  type: pgvector
  config:
    postgres_url: postgres://...

encryption_key: <base64>           # AES-256-GCM key for API key config encryption

workers:
  concurrency: 4                   # number of concurrent worker goroutines
  poll_interval: 5s                # how often workers poll for new jobs

links:
  auto_link_on_ingest: true
  auto_link_threshold: 0.85
  max_traversal_depth: 2

compaction:
  retain_compacted_chunks: true
```

### 7.3 Docker Images

- `ghcr.io/tight-line/creel:latest` - server + workers + CLI binaries
- `ghcr.io/tight-line/creel-dashboard:latest` - Laravel admin dashboard
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

### Phase 1: Foundation (complete)

- Go module, project structure, CI/CD (GitHub Actions)
- PostgreSQL schema (migrations via golang-migrate)
- gRPC service definitions (protobuf) + REST via grpc-gateway
- Auth middleware (OIDC + API key)
- Topic CRUD + grants (individual + group)
- Document CRUD
- Chunk ingestion (with pre-computed embeddings; batch)
- Vector backend interface + pgvector implementation
- Basic RAG search (ACL filtering, metadata filtering)
- Context mode retrieval (temporal ordering by sequence)
- Dockerfile, Docker Compose, Helm chart
- CLI (`creel-cli`): health, admin, topic, search, context
- creel-chat demo agent (two-layer retrieval, session resume)
- Server-side config registry (LLM, embedding, extraction prompt, API key configs)
- Laravel admin dashboard
- Dev workflow (Air live-reload, pre-configured dev key)

### Phase 2: Document Metadata & Citations (complete)

Small scope, high impact. Make RAG results citable.

- Add structured citation fields to documents (name, URL, author, published_at, custom JSONB)
- Migration to add columns to the documents table
- Update `SearchResponse` to include `DocumentCitation` alongside each chunk result
- Update `CreateDocument` and `UpdateDocument` to accept citation fields
- Update creel-chat to display citations in LLM responses
- Update CLI `search` output to show citation info
- Update dashboard document views to show/edit citation metadata

### Phase 3: Server-Side Document Processing (complete)

The big infrastructure phase. Prerequisite to the memory system.

- Worker pool infrastructure (goroutine pool, job polling, graceful shutdown)
- `ProcessingJob` table and migrations
- `UploadDocument` RPC (accepts file bytes or URL; returns document + job ID)
- Extraction workers (PDF text extraction, HTML-to-text, plain text passthrough)
- Chunking workers (configurable strategy: fixed-size with overlap, semantic boundaries via LLM)
- Embedding workers (compute embeddings via topic's configured embedding provider/model)
- Document status lifecycle (pending -> processing -> ready | failed)
- Job status API (`JobService`: GetJob, ListJobs)
- CLI: `creel-cli upload`, `creel-cli jobs list`, `creel-cli jobs status`
- Dashboard: document processing status, job monitoring
- Direct ingestion path preserved (IngestChunks still works; skips pipeline)

### Phase 4: Memory System (complete)

Depends on Phase 3's worker infrastructure and LLM/embedding configs.

- `Memory` table and migrations
- Memory extraction workers (conversation chunks in, candidate facts out via LLM)
- Memory maintenance workers (ADD/UPDATE/DELETE/NOOP conflict resolution via LLM)
- Scope-based memory retrieval (all memories in a scope returned to client; no vector search)
- Per-principal scoping with named scopes (default, work, home, etc.)
- `MemoryService` RPCs (GetMemory, SearchMemories, AddMemory, UpdateMemory, DeleteMemory, ListMemories, ListScopes)
- Soft-delete with audit trail (invalidated memories preserved with timestamp)
- Topic `memory_enabled` flag to control which topics trigger extraction
- Configurable extraction and maintenance prompts via ExtractionPromptConfig
- CLI: `creel-cli memory list`, `creel-cli memory search`, `creel-cli memory add`, `creel-cli memory delete`
- Dashboard: memory browser per principal/scope

### Phase 5: creel-chat Enhancements (complete)

Update the demo agent to showcase all new capabilities.

- Streaming LLM responses (display tokens as they arrive)
- Document upload flow (managed path; upload a file and have it indexed)
- Memory integration (fetch memory at session start, include in system prompt)
- Cross-topic RAG retrieval (search chunks in other topics accessible to the principal)
- Explicit memory commands ("/remember ...", "/forget ...")

### Phase 6: Integration Layers

- Python client library (full API coverage including upload, memory, citations)
- TypeScript client library (full API coverage)
- Tool/function schemas (OpenAI + Anthropic formats; includes upload, memory, citations)
- MCP server (SSE + stdio transport)
- MCP server Docker image

### Phase 7: Linking & Traversal

- Link CRUD (create, delete, list outbound + backlinks)
- Link ACL enforcement (read on both endpoints' topics)
- Auto-link on ingest (similarity search across accessible topics, via worker)
- Configurable auto-link threshold
- Permission-gated link traversal in RAG search
- Configurable traversal depth
- Reranking pool with linked chunks
- Compaction redirects (links to compacted chunks resolve to summary)
- Recursive redirect resolution

### Phase 8: Server-Driven Compaction

- Compaction policy configuration per topic (age threshold, chunk count threshold)
- Compaction workers (generate summaries via topic's LLM config)
- Automatic link transfer from compacted chunks to summary
- Compaction-aware context retrieval (summaries + active chunks)
- Un-compact (admin restore)
- Archival access to compacted chunks
- Manual `Compact` RPC preserved for direct path users

### Phase 9: Additional Backends & Hardening

- Qdrant vector backend + conformance tests
- Docker Compose: add Qdrant service; CI: add to integration test matrix
- Helm: Qdrant subchart + external option
- Weaviate vector backend + conformance tests
- Docker Compose: add Weaviate service; CI: add to integration test matrix
- Helm: Weaviate subchart + external option
- OpenAI vector store backend + conformance tests
- Prometheus metrics (request latency, chunk/link counts, backend latency, job throughput)
- Helm chart hardening (HPA, PDB, network policies, secret refs)
- Helm chart: optional MCP sidecar

### Post-v1

- Go client library
- .NET client library (with VectorData.Abstractions)
- Duck-typed SDK wrappers (Tier 4)
- Link analytics and visualization
- Event bus externalization (NATS)

## 9. Project Structure

```
creel/
├── CHANGELOG.md
├── CLAUDE.md
├── README.md
├── LICENSE
├── go.mod
├── go.sum
├── cmd/
│   ├── creel/
│   │   └── main.go             # server entrypoint
│   ├── creel-cli/
│   │   └── main.go             # CLI entrypoint
│   └── creel-chat/
│       └── main.go             # demo agent entrypoint
├── proto/
│   └── creel/
│       └── v1/
│           ├── topic.proto
│           ├── document.proto
│           ├── chunk.proto
│           ├── link.proto
│           ├── retrieval.proto
│           ├── compaction.proto
│           ├── memory.proto
│           ├── job.proto
│           ├── config.proto
│           └── admin.proto
├── internal/
│   ├── auth/
│   ├── config/
│   ├── crypto/
│   ├── server/
│   ├── store/
│   │   └── dbtest/
│   ├── retrieval/
│   ├── vector/
│   │   ├── backend.go
│   │   ├── pgvector/
│   │   ├── openai/
│   │   ├── qdrant/
│   │   └── vectortest/
│   ├── worker/              # worker pool and job processing
│   │   ├── pool.go
│   │   ├── extraction.go
│   │   ├── chunking.go
│   │   ├── embedding.go
│   │   └── memory.go
│   ├── memory/              # memory extraction and maintenance logic
│   ├── link/
│   └── compaction/
├── dashboard/               # Laravel admin dashboard
├── migrations/
├── deploy/
│   ├── docker/
│   │   ├── Dockerfile
│   │   └── Dockerfile.dev
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
├── scripts/
└── docs/
    ├── ARCHITECTURE.md
    ├── API_REFERENCE.md
    ├── CONCEPTS.md
    ├── QUICKSTART.md
    ├── FULLSTART.md
    ├── DEVELOPMENT.md
    ├── DEPLOYMENT.md
    └── CREEL_CHAT.md
```

## 10. v1 Build-out Checklist

### Phase 1: Foundation (complete)

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
- [x] Metadata filtering in search results
- [x] Context mode retrieval (temporal ordering by sequence)
- [x] Configurable context window (last N chunks, since timestamp)
- [x] Dockerfile (multi-stage, multi-arch)
- [x] Docker Compose with all current backends (PostgreSQL/pgvector)
- [x] Basic Helm chart (deployment, service, configmap)
- [x] Helm: PostgreSQL StatefulSet with pgvector
- [x] Helm: external PostgreSQL option (host, port, secretName in values.yaml)
- [x] Health endpoint
- [x] gRPC server wiring (all Phase 1 services)
- [x] REST API via grpc-gateway
- [x] Server-side config registry (LLM, embedding, extraction prompt, API key configs)
- [x] AES-256-GCM encryption for API key configs at rest
- [x] Topic config bindings (LLM, embedding, extraction prompt)
- [x] CLI: project scaffold (cobra)
- [x] CLI: config management (endpoint, API key, profiles)
- [x] CLI: `creel-cli health`
- [x] CLI: `creel-cli admin create-account / list-accounts / rotate-key / revoke-key`
- [x] CLI: `creel-cli topic create / list / grant / grants / update`
- [x] CLI: `creel-cli config` (CRUD for all config types)
- [x] CLI: `creel-cli search` and `creel-cli context`
- [x] creel-chat: REPL with two-layer retrieval (temporal + RAG)
- [x] creel-chat: client-side embedding (OpenAI, Ollama)
- [x] creel-chat: client-side LLM (Anthropic, OpenAI)
- [x] creel-chat: session resume via `--resume`
- [x] Laravel admin dashboard (configs, topics, system accounts)
- [x] Auto-generated secrets in Helm chart
- [x] Air-based live-reload dev workflow
- [x] Pre-configured dev API key

### Phase 2: Document Metadata & Citations (complete)

- [x] Migration: add `url`, `author`, `published_at` columns to documents table
- [x] Update Document proto with citation fields
- [x] Update `CreateDocument` / `UpdateDocument` to accept citation fields
- [x] Add `DocumentCitation` message to proto
- [x] Update `SearchResponse` to include `DocumentCitation` per result
- [x] Update retrieval engine to join document metadata in search results
- [x] Update creel-chat to display citations in responses
- [x] Update CLI `search` output to show citation info
- [x] Update dashboard document views to show/edit citation metadata

### Phase 3: Server-Side Document Processing (complete)

- [x] Worker pool infrastructure (goroutine pool, job polling, graceful shutdown)
- [x] `ProcessingJob` table migration
- [x] Job store (CRUD for processing jobs)
- [x] `UploadDocument` RPC (file bytes or URL; returns document + job ID)
- [x] PDF text extraction worker
- [x] HTML-to-text extraction worker
- [x] Plain text passthrough worker
- [x] URL fetch worker (download from source_url)
- [x] Chunking worker: fixed-size with overlap
- [x] Chunking worker: semantic boundaries via LLM (optional)
- [x] Embedding worker (compute via topic's embedding config)
- [x] OpenAI embedding provider (server-side, dynamically resolved from DB config)
- [x] Content type auto-detection (falls back to `http.DetectContentType` when not provided)
- [x] Document status lifecycle (pending -> processing -> ready | failed)
- [x] `JobService` RPCs (GetJob, ListJobs)
- [x] CLI: `creel-cli upload`
- [x] CLI: `creel-cli jobs list` / `creel-cli jobs status`
- [x] CLI: `creel-cli document list` / `get` / `delete`
- [x] CLI: slug-to-UUID resolution for topics and documents
- [x] Dashboard: document processing status view
- [x] Dashboard: job monitoring view
- [x] Worker configuration (concurrency, poll interval) in creel.yaml

### Phase 4: Memory System (complete)

- [x] `Memory` table migration
- [x] Memory store (CRUD, search, scope listing)
- [x] Scope-based memory retrieval (all memories returned by scope; embedding removed)
- [x] Memory extraction worker (LLM extracts candidate facts from conversation chunks)
- [x] Memory extraction prompt (few-shot examples, category guidance)
- [x] Memory maintenance worker (ADD/UPDATE/DELETE/NOOP conflict resolution via LLM)
- [x] Soft-delete with invalidated status and timestamp
- [x] Per-principal scoping with named scopes
- [x] Topic `memory_enabled` flag (migration + topic update)
- [x] `MemoryService` RPCs (GetMemory, SearchMemories, AddMemory, UpdateMemory, DeleteMemory, ListMemories, ListScopes)
- [x] Configurable memory extraction/maintenance prompts via ExtractionPromptConfig
- [x] CLI: `creel-cli memory list / search / add / delete`
- [x] Dashboard: memory browser per principal/scope

### Phase 5: creel-chat Enhancements (complete)

- [x] Streaming LLM responses
- [x] Document upload flow (managed path via `UploadDocument` RPC)
- [x] Memory integration (fetch memory at session start, include in system prompt)
- [x] Cross-topic RAG retrieval
- [x] Explicit memory commands (`/remember`, `/forget`)
- [x] Removed self-grant workaround (no longer needed with `AccessibleTopics` ownership fix)

### Phase 6: Integration Layers (complete)

- [x] Python client library (full API: upload, search, context, memory, citations)
- [ ] Python client: auth token handling, retries
- [x] TypeScript client library (full API: upload, search, context, memory, citations)
- [ ] TypeScript client: auth token handling, retries
- [x] Tool schemas: OpenAI function calling format (JSON)
- [x] Tool schemas: Anthropic tool_use format (JSON)
- [ ] Tool schemas: language-native objects in Python + TS clients
- [x] MCP server (SSE transport)
- [x] MCP server (stdio transport)
- [x] MCP server Docker image

### Phase 7: Linking & Traversal (complete)

- [x] Link CRUD (create, delete, list outbound + backlinks)
- [x] Link ACL enforcement (read on both endpoints' topics)
- [x] Auto-link worker (similarity search across accessible topics on ingest)
- [x] Configurable auto-link threshold
- [ ] Permission-gated link traversal in RAG search
- [ ] Configurable traversal depth
- [ ] Reranking pool with linked chunks
- [x] Compaction redirects (links to compacted chunks resolve to summary)
- [ ] Recursive redirect resolution

### Phase 8: Server-Driven Compaction (complete)

- [ ] Compaction policy configuration per topic (age threshold, chunk count threshold)
- [x] Compaction worker (generate summaries via topic's LLM config)
- [x] Chunk tombstoning (status=compacted, compacted_by=summary)
- [x] Outbound link transfer from compacted chunks to summary
- [ ] Compaction-aware context retrieval (summaries + active chunks)
- [x] Un-compact (admin restore)
- [ ] Archival access to compacted chunks
- [x] Manual `Compact` RPC preserved

### Phase 9: Additional Backends & Hardening

- [ ] Qdrant vector backend implementation
- [ ] Qdrant backend conformance tests
- [ ] Docker Compose: add Qdrant service
- [ ] CI: add Qdrant to integration test matrix
- [ ] Helm: Qdrant subchart + external option
- [ ] Weaviate vector backend implementation
- [ ] Weaviate backend conformance tests
- [ ] Docker Compose: add Weaviate service
- [ ] CI: add Weaviate to integration test matrix
- [ ] Helm: Weaviate subchart + external option
- [ ] OpenAI vector store backend + conformance tests
- [x] Prometheus metrics (gRPC request counters and latency histograms via go-grpc-prometheus)
- [x] Helm chart hardening (PDB, NetworkPolicy, ServiceMonitor, SecurityContext)
- [x] Helm chart: optional MCP sidecar
- [ ] Ingress configuration (gRPC + REST)
- [x] VectorBackendConfig CRUD in config registry (managed vector backend configurations)
- [x] Per-topic `vector_backend_config_id` FK (topics can reference a specific vector backend)
- [x] Vector backend registry (`vector.Registry`) with lazy initialization and factory-based creation

### Phase 10: Scalability & Pagination

- [ ] Cursor-based pagination for unbounded list RPCs (`ListScopes`, `AccessibleTopics`, `ListTopics`, `ListMemories`, `ListDocuments`, etc.). Current implementations return everything, which breaks at scale.
- [ ] Dashboard memory scope view: show principal + scope + memory count (requires admin-capable `ListScopes` variant that spans principals)
- [ ] `GetJob` detail view for documentless jobs in dashboard (memory maintenance jobs have no document to link to)

## 11. Verification

- **Unit tests**: each internal package has tests; vector backend interface has a conformance test suite that all implementations must pass
- **Integration tests**: Docker Compose setup with PostgreSQL + Creel server; tests cover full CRUD, search, link traversal, compaction, ACL enforcement, document processing pipeline, memory extraction
- **Worker tests**: integration tests for each worker type with mock LLM/embedding providers
- **Client library tests**: each SDK has integration tests against a running Creel server
- **MCP conformance**: test MCP server against MCP inspector tool
- **Helm chart**: test install on kind (Kubernetes in Docker) cluster
- **CI**: GitHub Actions runs unit tests, integration tests, builds Docker images, lints proto files
