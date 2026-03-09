# Creel API Reference

63 RPC methods across 9 gRPC services (plus ConfigService). All methods are also available via REST (grpc-gateway).

Every request carries an `Authorization: Bearer <token>` header (OIDC JWT or API key). The server resolves the caller's principal, then checks permissions via the `Authorizer` interface before touching data.

**Permission boundary**: Topic is the permission boundary. Documents and chunks inherit access from their parent topic. There are no document-level or chunk-level grants.

---

## TopicService

### CreateTopic

```
rpc CreateTopic(CreateTopicRequest) returns (Topic)
```

**Request**: `{slug, name, description}`
**Permission**: authenticated (any valid principal)
**Behavior**: Creates a topic. The caller becomes the owner (implicit admin). If `slug` is omitted, the server auto-generates one from `name` (slugified + short random suffix, e.g., `ml-research-a7x3`). If provided, the slug must be unique and URL-safe. Returns the created topic.

### GetTopic

```
rpc GetTopic(GetTopicRequest) returns (Topic)
```

**Request**: `{id}`
**Permission**: read
**Behavior**: Returns topic metadata.

### ListTopics

```
rpc ListTopics(ListTopicsRequest) returns (ListTopicsResponse)
```

**Request**: `{page_size, page_token}`
**Response**: `{topics[], next_page_token}`
**Permission**: authenticated
**Behavior**: Returns only topics the caller can access (any permission level). The Authorizer filters via `AccessibleTopics`.

### UpdateTopic

```
rpc UpdateTopic(UpdateTopicRequest) returns (Topic)
```

**Request**: `{id, name, description}`
**Permission**: admin
**Behavior**: Updates mutable fields. Slug is immutable.

### DeleteTopic

```
rpc DeleteTopic(DeleteTopicRequest) returns (DeleteTopicResponse)
```

**Request**: `{id}`
**Permission**: admin
**Behavior**: Cascading delete. Removes all documents, chunks, grants, and links with endpoints in this topic. Deletes embeddings from the vector backend.

### GrantAccess

```
rpc GrantAccess(GrantAccessRequest) returns (TopicGrant)
```

**Request**: `{topic_id, principal, permission}`
**Permission**: admin
**Behavior**: Creates or updates a grant. Principal is a typed ref (`user:...`, `group:...`, `system:...`). Upserts on the `(topic_id, principal)` unique constraint.

### RevokeAccess

```
rpc RevokeAccess(RevokeAccessRequest) returns (RevokeAccessResponse)
```

**Request**: `{topic_id, principal}`
**Permission**: admin
**Behavior**: Deletes the grant row. Cannot revoke the owner's implicit admin.

### ListGrants

```
rpc ListGrants(ListGrantsRequest) returns (ListGrantsResponse)
```

**Request**: `{topic_id}`
**Response**: `{grants[]}`
**Permission**: admin
**Behavior**: Lists all grants for a topic.

---

## DocumentService

### CreateDocument

```
rpc CreateDocument(CreateDocumentRequest) returns (Document)
```

**Request**: `{topic_id, slug, name, doc_type, metadata, url, author, published_at}`
**Permission**: write
**Behavior**: Creates a document in the topic. If `slug` is omitted, the server auto-generates one from `name` (slugified + short random suffix). If provided, the slug must be unique within the topic. `doc_type` is informational only (Creel does not change behavior based on it). The `url`, `author`, and `published_at` fields are optional citation metadata; they are stored on the document and surfaced in search results via `DocumentCitation`.

### UploadDocument

```
rpc UploadDocument(UploadDocumentRequest) returns (UploadDocumentResponse)
```

**Request**: `{topic_id, slug, name, url, author, published_at, metadata, file, source_url, doc_type}`
**Response**: `{document, job_id}`
**Permission**: write
**Behavior**: Accepts a raw file (as bytes) or a `source_url` for the server to fetch. Slug is optional; auto-generated from `name` if omitted. Creates a document record with citation metadata and sets its status to `pending`. Enqueues extraction, chunking, and embedding jobs for asynchronous processing. Returns immediately with the document and a job ID for tracking progress. The document transitions through `processing` to `ready` (or `failed`) as workers complete each stage.

### GetDocument

```
rpc GetDocument(GetDocumentRequest) returns (Document)
```

**Request**: `{id}`
**Permission**: read
**Behavior**: Returns document metadata. Server resolves the topic from the document and checks read access.

### ListDocuments

```
rpc ListDocuments(ListDocumentsRequest) returns (ListDocumentsResponse)
```

**Request**: `{topic_id, page_size, page_token}`
**Response**: `{documents[], next_page_token}`
**Permission**: read
**Behavior**: Lists documents in a topic.

### UpdateDocument

```
rpc UpdateDocument(UpdateDocumentRequest) returns (Document)
```

**Request**: `{id, name, doc_type, metadata, url, author, published_at}`
**Permission**: write
**Behavior**: Updates mutable fields including citation metadata. Slug is immutable.

### DeleteDocument

```
rpc DeleteDocument(DeleteDocumentRequest) returns (DeleteDocumentResponse)
```

**Request**: `{id}`
**Permission**: admin
**Behavior**: Cascading delete. Removes all chunks and their embeddings from the vector backend. Links with endpoints in deleted chunks are deleted.

---

## ChunkService

### IngestChunks

```
rpc IngestChunks(IngestChunksRequest) returns (IngestChunksResponse)
```

**Request**: `{document_id, chunks[{content, embedding?, sequence, metadata}]}`
**Response**: `{chunks[]}`
**Permission**: write
**Behavior**: Batch ingest. For each chunk:

- If `embedding` is provided, stores it directly in the vector backend.
- If `embedding` is omitted and an embedding provider is configured, the server computes it.
- Assigns an `embedding_id` linking the chunk metadata to its vector.

If `auto_link_on_ingest` is enabled, fires an async auto-link job via the event bus. The job searches for similar chunks in other topics the caller can access and creates links above the configured similarity threshold. Auto-linking does not block the ingest response.

Returns the created chunks with their IDs.

### GetChunk

```
rpc GetChunk(GetChunkRequest) returns (Chunk)
```

**Request**: `{id}`
**Permission**: read
**Behavior**: Returns chunk metadata and content. Server resolves the topic via the document and checks read access.

### DeleteChunk

```
rpc DeleteChunk(DeleteChunkRequest) returns (DeleteChunkResponse)
```

**Request**: `{id}`
**Permission**: admin
**Behavior**: Deletes the chunk, its embedding from the vector backend, and all links where this chunk is source or target.

---

## LinkService

### CreateLink

```
rpc CreateLink(CreateLinkRequest) returns (Link)
```

**Request**: `{source_chunk_id, target_chunk_id, link_type, metadata}`
**Permission**: read on both the source and target chunks' topics
**Behavior**: Creates a directional link between two chunks. `link_type` defaults to `manual`. If either chunk is compacted, the link targets the summary chunk instead (transparent redirect). Source and target may be in different topics and different documents.

### DeleteLink

```
rpc DeleteLink(DeleteLinkRequest) returns (DeleteLinkResponse)
```

**Request**: `{id}`
**Permission**: write on the source chunk's topic
**Behavior**: Deletes the link.

### ListLinks

```
rpc ListLinks(ListLinksRequest) returns (ListLinksResponse)
```

**Request**: `{chunk_id, include_backlinks}`
**Response**: `{links[]}`
**Permission**: read
**Behavior**: Lists outbound links from this chunk. If `include_backlinks` is true, also includes links where this chunk is the target. Only returns links where the caller has read access to both endpoints' topics. Compacted targets resolve to their summary chunks transparently.

---

## RetrievalService

### Search

```
rpc Search(SearchRequest) returns (SearchResponse)
```

**Request**:

```
{
  topic_ids: [uuid],
  query_embedding: [float64],   // pre-computed, OR:
  query_text: string,           // server computes embedding
  top_k: int,
  follow_links: bool,
  link_depth: int,              // default 1
  metadata_filter: Filter,
  exclude_document_ids: [uuid], // omit chunks from these documents
  rerank: bool,                 // enable reranking (default: true if default RerankConfig exists)
  rerank_config_id: uuid,       // override reranker (nullable; uses default if omitted)
  rerank_candidates: int        // override candidate pool size (nullable; falls back to config, then top_k)
}
```

**Response**:

```
{
  results: [{
    chunk: Chunk,
    document_id: string,
    document_citation: DocumentCitation {
      id: uuid,
      slug: string,
      name: string,
      url: string,
      author: string,
      published_at: timestamp,
      metadata: jsonb
    },
    topic_id: string,
    score: float64,
    via_link: Link               // nullable; set if reached via traversal
  }]
}
```

**Permission**: read on all specified `topic_ids`
**Behavior**: RAG mode semantic search.

1. Caller must have read on all specified `topic_ids`; server rejects if any fail authz.
2. Either `query_embedding` or `query_text` must be provided (not both). If `query_text`, the server computes the embedding via the configured embedding provider.
3. Server resolves all chunk IDs in the accessible topics from PostgreSQL and passes them as a filter to the vector backend's `Search` method. If reranking is enabled, the vector search fetches `rerank_candidates` results (resolution order: request field, then RerankConfig `top_n_candidates`, then `top_k`).
4. If `rerank` is true (or a default RerankConfig exists and `rerank` is not explicitly false), the candidate pool is re-scored by the configured reranker (Cohere, Jina, VoyageAI, or LLM-based cross-encoder) and trimmed to `top_k`.
5. If `follow_links` is true, for each top-k result, the server follows outbound links up to `link_depth` hops (default 1; max from server config). Linked chunks are only included if the caller has read access to the linked chunk's topic.
6. All candidates (direct hits + linked chunks) are scored and ranked into the final result set.
7. Results include a `via_link` reference when a result was reached through link traversal.

### GetContext

```
rpc GetContext(GetContextRequest) returns (GetContextResponse)
```

**Request**:

```
{
  document_id: uuid,
  last_n: int,                  // last N active chunks
  since: timestamp,             // alternative: chunks since this time
  include_summaries: bool       // default true
}
```

**Response**: `{chunks[]}`
**Permission**: read
**Behavior**: Context mode temporal retrieval.

- Returns active chunks from a single document in sequence order.
- If `last_n` is set, returns the last N active (non-compacted) chunks.
- If `since` is set, returns active chunks created at or after that timestamp.
- `include_summaries` is accepted but currently ignored (compaction-aware retrieval is not yet implemented).

---

## CompactionService

### Compact

```
rpc Compact(CompactRequest) returns (CompactResponse)
```

**Request**:

```
{
  document_id: uuid,
  chunk_ids: [uuid],
  summary_content: string,
  summary_embedding: [float64],  // optional; server computes if omitted
  summary_metadata: jsonb
}
```

**Response**: `{summary_chunk, compacted_count}`
**Permission**: write
**Behavior**:

1. All `chunk_ids` must belong to `document_id` and have status=active. Fails if any chunk is already compacted or belongs to a different document.
2. Creates a new summary chunk with the provided content and embedding (computes embedding if omitted and provider is configured).
3. Sets each specified chunk's status to `compacted` and `compacted_by` to the summary chunk's ID.
4. Transfers all outbound links from compacted chunks to the summary chunk. These links get `link_type = compaction_transfer`.
5. Inbound links to compacted chunks now resolve to the summary on traversal (transparent redirect).
6. Returns the summary chunk and the count of compacted chunks.

### Uncompact

```
rpc Uncompact(UncompactRequest) returns (UncompactResponse)
```

**Request**: `{summary_chunk_id}`
**Response**: `{restored_chunks[]}`
**Permission**: admin
**Behavior**: Reverses a compaction.

1. Restores all chunks where `compacted_by = summary_chunk_id` to status=active and clears `compacted_by`.
2. Transfers `compaction_transfer` links back to their original source chunks.
3. Deletes the summary chunk and its embedding from the vector backend.
4. Returns the restored chunks.

---

## MemoryService

Memories are per-principal, scoped key-value observations that persist across sessions. Each memory belongs to the calling principal and a named scope. Only the owning principal can access their own memories.

### GetMemory

```
rpc GetMemory(GetMemoryRequest) returns (GetMemoryResponse)
```

**Request**: `{scope}`
**Response**: `{memories[]}`
**Permission**: authenticated
**Behavior**: Returns all active memories for the calling principal in the given scope.

### SearchMemories

```
rpc SearchMemories(SearchMemoriesRequest) returns (SearchMemoriesResponse)
```

**Request**: `{scope, query_text, query_embedding, top_k}`
**Response**: `{memories[]}`
**Permission**: authenticated
**Behavior**: Semantic search within the calling principal's memories in the given scope. Either `query_text` or `query_embedding` must be provided (not both). If `query_text` is provided, the server computes the embedding via the configured provider.

### AddMemory

```
rpc AddMemory(AddMemoryRequest) returns (Memory)
```

**Request**: `{scope, content, metadata}`
**Response**: `{memory}`
**Permission**: authenticated
**Behavior**: Explicitly adds a memory for the calling principal in the given scope.

### UpdateMemory

```
rpc UpdateMemory(UpdateMemoryRequest) returns (Memory)
```

**Request**: `{id, content, metadata}`
**Response**: `{memory}`
**Permission**: authenticated
**Behavior**: Updates a specific memory. Only the owning principal can update their memories.

### DeleteMemory

```
rpc DeleteMemory(DeleteMemoryRequest) returns (DeleteMemoryResponse)
```

**Request**: `{id}`
**Response**: `{}`
**Permission**: authenticated
**Behavior**: Soft-deletes the memory by setting its status to `invalidated`. Only the owning principal can delete their memories.

### ListMemories

```
rpc ListMemories(ListMemoriesRequest) returns (ListMemoriesResponse)
```

**Request**: `{scope, include_invalidated}`
**Response**: `{memories[]}`
**Permission**: authenticated
**Behavior**: Lists all memories for the calling principal in the given scope. If `include_invalidated` is true, also returns memories with status `invalidated` for audit purposes.

### ListScopes

```
rpc ListScopes(ListScopesRequest) returns (ListScopesResponse)
```

**Request**: `{}`
**Response**: `{scopes[]}`
**Permission**: authenticated
**Behavior**: Lists all memory scopes that the calling principal has stored memories in.

---

## JobService

Jobs track asynchronous work such as document extraction, chunking, and embedding. Jobs are created automatically by operations like `UploadDocument`.

### GetJob

```
rpc GetJob(GetJobRequest) returns (Job)
```

**Request**: `{id}`
**Response**: `{job}`
**Permission**: authenticated
**Behavior**: Returns job details. Any authenticated user can view jobs for documents they have read access to.

### ListJobs

```
rpc ListJobs(ListJobsRequest) returns (ListJobsResponse)
```

**Request**: `{topic_id, document_id, status, page_size, page_token}`
**Response**: `{jobs[], next_page_token}`
**Permission**: read on topic
**Behavior**: Lists jobs, filterable by topic, document, or status. Pagination via `page_token`.

---

## AdminService

### Health

```
rpc Health(HealthRequest) returns (HealthResponse)
```

**Request**: `{}`
**Response**: `{status, version}`
**Permission**: none (unauthenticated)
**Behavior**: Returns server health and version. Checks connectivity to the metadata store and vector backend.

### CreateSystemAccount

```
rpc CreateSystemAccount(CreateSystemAccountRequest) returns (CreateSystemAccountResponse)
```

**Request**: `{name, description}`
**Response**: `{account, api_key}`
**Permission**: admin (via bootstrap key or existing admin system account)
**Behavior**: Creates a system account with principal `system:{name}`. Generates an API key (`creel_ak_...`). The key is returned exactly once and is never retrievable again.

### ListSystemAccounts

```
rpc ListSystemAccounts(ListSystemAccountsRequest) returns (ListSystemAccountsResponse)
```

**Request**: `{}`
**Response**: `{accounts[]}`
**Permission**: admin
**Behavior**: Lists all system accounts without their keys.

### DeleteSystemAccount

```
rpc DeleteSystemAccount(DeleteSystemAccountRequest) returns (DeleteSystemAccountResponse)
```

**Request**: `{id}`
**Permission**: admin
**Behavior**: Deletes the system account and all its keys. Topic grants referencing this account's principal remain as orphaned rows (no effect on access).

### RotateKey

```
rpc RotateKey(RotateKeyRequest) returns (RotateKeyResponse)
```

**Request**: `{account_id, grace_period_seconds}`
**Response**: `{api_key}`
**Permission**: admin
**Behavior**: Generates a new key and returns it. The old key enters `grace_period` status and remains valid for `grace_period_seconds`. If `grace_period_seconds` is 0, the old key is revoked immediately.

### RevokeKey

```
rpc RevokeKey(RevokeKeyRequest) returns (RevokeKeyResponse)
```

**Request**: `{account_id}`
**Permission**: admin
**Behavior**: Immediately invalidates all active keys for this system account.
