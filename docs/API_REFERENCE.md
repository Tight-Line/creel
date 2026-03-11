# Creel API Reference

72 RPC methods across 10 gRPC services. All methods are also available via REST (grpc-gateway).

Every request carries an `Authorization: Bearer <token>` header (OIDC JWT or API key). The server resolves the caller's principal, then checks permissions via the `Authorizer` interface before touching data.

**Permission boundary**: Topic is the permission boundary. Documents and chunks inherit access from their parent topic. There are no document-level or chunk-level grants.

---

## TopicService

### CreateTopic

```
rpc CreateTopic(CreateTopicRequest) returns (Topic)
```

| REST | `POST /v1/topics` |
|------|-------------------|

**Request**: `{slug, name, description}`
**Permission**: authenticated (any valid principal)
**Behavior**: Creates a topic. The caller becomes the owner (implicit admin). If `slug` is omitted, the server auto-generates one from `name` (slugified + short random suffix, e.g., `ml-research-a7x3`). If provided, the slug must be unique and URL-safe. Returns the created topic.

### GetTopic

```
rpc GetTopic(GetTopicRequest) returns (Topic)
```

| REST | `GET /v1/topics/{id}` |
|------|-----------------------|

**Request**: `{id}`
**Permission**: read
**Behavior**: Returns topic metadata.

### ListTopics

```
rpc ListTopics(ListTopicsRequest) returns (ListTopicsResponse)
```

| REST | `GET /v1/topics` |
|------|------------------|

**Request**: `{page_size, page_token}`
**Response**: `{topics[], next_page_token}`
**Permission**: authenticated
**Behavior**: Returns only topics the caller can access (any permission level). The Authorizer filters via `AccessibleTopics`.

### UpdateTopic

```
rpc UpdateTopic(UpdateTopicRequest) returns (Topic)
```

| REST | `PATCH /v1/topics/{id}` |
|------|-------------------------|

**Request**: `{id, name, description}`
**Permission**: admin
**Behavior**: Updates mutable fields. Slug is immutable.

### DeleteTopic

```
rpc DeleteTopic(DeleteTopicRequest) returns (DeleteTopicResponse)
```

| REST | `DELETE /v1/topics/{id}` |
|------|--------------------------|

**Request**: `{id}`
**Permission**: admin
**Behavior**: Cascading delete. Removes all documents, chunks, grants, and links with endpoints in this topic. Deletes embeddings from the vector backend.

### GrantAccess

```
rpc GrantAccess(GrantAccessRequest) returns (TopicGrant)
```

| REST | `POST /v1/topics/{topic_id}/grants` |
|------|-------------------------------------|

**Request**: `{topic_id, principal, permission}`
**Permission**: admin
**Behavior**: Creates or updates a grant. Principal is a typed ref (`user:...`, `group:...`, `system:...`). Upserts on the `(topic_id, principal)` unique constraint.

### RevokeAccess

```
rpc RevokeAccess(RevokeAccessRequest) returns (RevokeAccessResponse)
```

| REST | `DELETE /v1/topics/{topic_id}/grants/{principal}` |
|------|---------------------------------------------------|

**Request**: `{topic_id, principal}`
**Permission**: admin
**Behavior**: Deletes the grant row. Cannot revoke the owner's implicit admin.

### ListGrants

```
rpc ListGrants(ListGrantsRequest) returns (ListGrantsResponse)
```

| REST | `GET /v1/topics/{topic_id}/grants` |
|------|-------------------------------------|

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

| REST | `POST /v1/documents` |
|------|----------------------|

**Request**: `{topic_id, slug, name, doc_type, metadata, url, author, published_at}`
**Permission**: write
**Behavior**: Creates a document in the topic. If `slug` is omitted, the server auto-generates one from `name` (slugified + short random suffix). If provided, the slug must be unique within the topic. `doc_type` is informational only (Creel does not change behavior based on it). The `url`, `author`, and `published_at` fields are optional citation metadata; they are stored on the document and surfaced in search results via `DocumentCitation`.

### UploadDocument

```
rpc UploadDocument(UploadDocumentRequest) returns (UploadDocumentResponse)
```

| REST | `POST /v1/documents:upload` |
|------|------------------------------|

**Request**: `{topic_id, slug, name, url, author, published_at, metadata, file, source_url, doc_type}`
**Response**: `{document, job_id}`
**Permission**: write
**Behavior**: Accepts a raw file (as bytes) or a `source_url` for the server to fetch. Slug is optional; auto-generated from `name` if omitted. Creates a document record with citation metadata and sets its status to `pending`. If `source_url` is provided and `url` is not, the source URL is automatically used as the citation URL. An explicit `url` always takes precedence. Enqueues extraction, chunking, and embedding jobs for asynchronous processing. Returns immediately with the document and a job ID for tracking progress. The document transitions through `processing` to `ready` (or `failed`) as workers complete each stage.

### GetDocument

```
rpc GetDocument(GetDocumentRequest) returns (Document)
```

| REST | `GET /v1/documents/{id}` |
|------|--------------------------|

**Request**: `{id}`
**Permission**: read
**Behavior**: Returns document metadata. Server resolves the topic from the document and checks read access.

### ListDocuments

```
rpc ListDocuments(ListDocumentsRequest) returns (ListDocumentsResponse)
```

| REST | `GET /v1/topics/{topic_id}/documents` |
|------|---------------------------------------|

**Request**: `{topic_id, page_size, page_token}`
**Response**: `{documents[], next_page_token}`
**Permission**: read
**Behavior**: Lists documents in a topic.

### UpdateDocument

```
rpc UpdateDocument(UpdateDocumentRequest) returns (Document)
```

| REST | `PATCH /v1/documents/{id}` |
|------|----------------------------|

**Request**: `{id, name, doc_type, metadata, url, author, published_at}`
**Permission**: write
**Behavior**: Updates mutable fields including citation metadata. Slug is immutable.

### DeleteDocument

```
rpc DeleteDocument(DeleteDocumentRequest) returns (DeleteDocumentResponse)
```

| REST | `DELETE /v1/documents/{id}` |
|------|------------------------------|

**Request**: `{id}`
**Permission**: admin
**Behavior**: Cascading delete. Removes all chunks and their embeddings from the vector backend. Links with endpoints in deleted chunks are deleted.

---

## ChunkService

### IngestChunks

```
rpc IngestChunks(IngestChunksRequest) returns (IngestChunksResponse)
```

| REST | `POST /v1/documents/{document_id}/chunks` |
|------|-------------------------------------------|

**Request**: `{document_id, chunks[{content, embedding?, sequence, metadata}]}`
**Response**: `{chunks[]}`
**Permission**: write
**Behavior**: Batch ingest. For each chunk:

- If `embedding` is provided, stores it directly in the vector backend and sets `embedding_id`.
- If `embedding` is omitted, the chunk is created without an embedding and a background embedding job is enqueued. The embedding worker computes embeddings using the configured provider (topic-level or default).

If `auto_link_on_ingest` is enabled, fires an async auto-link job via the event bus. The job searches for similar chunks in other topics the caller can access and creates links above the configured similarity threshold. Auto-linking does not block the ingest response.

Returns the created chunks with their IDs.

### GetChunk

```
rpc GetChunk(GetChunkRequest) returns (Chunk)
```

| REST | `GET /v1/chunks/{id}` |
|------|-----------------------|

**Request**: `{id}`
**Permission**: read
**Behavior**: Returns chunk metadata and content. Server resolves the topic via the document and checks read access.

### DeleteChunk

```
rpc DeleteChunk(DeleteChunkRequest) returns (DeleteChunkResponse)
```

| REST | `DELETE /v1/chunks/{id}` |
|------|--------------------------|

**Request**: `{id}`
**Permission**: admin
**Behavior**: Deletes the chunk, its embedding from the vector backend, and all links where this chunk is source or target.

---

## LinkService

### CreateLink

```
rpc CreateLink(CreateLinkRequest) returns (Link)
```

| REST | `POST /v1/links` |
|------|-------------------|

**Request**: `{source_chunk_id, target_chunk_id, link_type, metadata}`
**Permission**: read on both the source and target chunks' topics
**Behavior**: Creates a directional link between two chunks. `link_type` defaults to `manual`. If either chunk is compacted, the link targets the summary chunk instead (transparent redirect). Source and target may be in different topics and different documents.

### DeleteLink

```
rpc DeleteLink(DeleteLinkRequest) returns (DeleteLinkResponse)
```

| REST | `DELETE /v1/links/{id}` |
|------|-------------------------|

**Request**: `{id}`
**Permission**: write on the source chunk's topic
**Behavior**: Deletes the link.

### ListLinks

```
rpc ListLinks(ListLinksRequest) returns (ListLinksResponse)
```

| REST | `GET /v1/chunks/{chunk_id}/links` |
|------|-----------------------------------|

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

| REST | `POST /v1/search` |
|------|-------------------|

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

| REST | `GET /v1/documents/{document_id}/context` |
|------|-------------------------------------------|

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

| REST | `POST /v1/compact` |
|------|---------------------|

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
2. Creates a new summary chunk with the provided content and embedding. If `summary_embedding` is omitted, enqueues a background embedding job so the configured provider computes it asynchronously.
3. Sets each specified chunk's status to `compacted` and `compacted_by` to the summary chunk's ID.
4. Transfers all outbound links from compacted chunks to the summary chunk. These links get `link_type = compaction_transfer`.
5. Inbound links to compacted chunks now resolve to the summary on traversal (transparent redirect).
6. Returns the summary chunk and the count of compacted chunks.

### Uncompact

```
rpc Uncompact(UncompactRequest) returns (UncompactResponse)
```

| REST | `POST /v1/uncompact` |
|------|----------------------|

**Request**: `{summary_chunk_id}`
**Response**: `{restored_chunks[]}`
**Permission**: admin
**Behavior**: Reverses a compaction.

1. Restores all chunks where `compacted_by = summary_chunk_id` to status=active and clears `compacted_by`.
2. Transfers `compaction_transfer` links back to their original source chunks.
3. Deletes the summary chunk and its embedding from the vector backend.
4. Enqueues an embedding job to recompute embeddings for the restored chunks.
5. Returns the restored chunks.

### RequestCompaction

```
rpc RequestCompaction(RequestCompactionRequest) returns (RequestCompactionResponse)
```

| REST | `POST /v1/compact/request` |
|------|----------------------------|

**Request**: `{document_id, chunk_ids[]}`
**Response**: `{job_id}`
**Permission**: write on the document's topic
**Behavior**: Enqueues a background compaction job that uses the configured LLM to synthesize a summary.

- If `chunk_ids` is provided, only those chunks are compacted. Otherwise, all active chunks for the document are compacted.
- The compaction worker calls the LLM, creates a summary chunk, computes its embedding, transfers links, and marks source chunks as compacted.
- Returns the job ID so the caller can poll for completion via `GetJob`.

### GetCompactionHistory

```
rpc GetCompactionHistory(GetCompactionHistoryRequest) returns (GetCompactionHistoryResponse)
```

| REST | `GET /v1/documents/{document_id}/compaction-history` |
|------|------------------------------------------------------|

**Request**: `{document_id}`
**Response**: `{records[]}`
**Permission**: read on the document's topic
**Behavior**: Returns all compaction records for a document. Each record includes the summary chunk ID, source chunk IDs, who created it, and when.

---

## MemoryService

Memories are per-principal, scoped key-value observations that persist across sessions. Each memory belongs to the calling principal and a named scope. Only the owning principal can access their own memories.

### GetMemory

```
rpc GetMemory(GetMemoryRequest) returns (GetMemoryResponse)
```

| REST | `GET /v1/memories/{scope}` |
|------|----------------------------|

**Request**: `{scope}`
**Response**: `{memories[]}`
**Permission**: authenticated
**Behavior**: Returns all active memories for the calling principal in the given scope.

### SearchMemories

```
rpc SearchMemories(SearchMemoriesRequest) returns (SearchMemoriesResponse)
```

| REST | `POST /v1/memories:search` |
|------|----------------------------|

**Request**: `{scope, query_text, query_embedding, top_k}`
**Response**: `{memories[]}`
**Permission**: authenticated
**Behavior**: Semantic search within the calling principal's memories in the given scope. Either `query_text` or `query_embedding` must be provided (not both). If `query_text` is provided, the server computes the embedding via the configured provider.

### AddMemory

```
rpc AddMemory(AddMemoryRequest) returns (Memory)
```

| REST | `POST /v1/memories` |
|------|---------------------|

**Request**: `{scope, content, metadata}`
**Response**: `{memory}`
**Permission**: authenticated
**Behavior**: Explicitly adds a memory for the calling principal in the given scope.

### UpdateMemory

```
rpc UpdateMemory(UpdateMemoryRequest) returns (Memory)
```

| REST | `PATCH /v1/memories/{id}` |
|------|---------------------------|

**Request**: `{id, content, metadata}`
**Response**: `{memory}`
**Permission**: authenticated
**Behavior**: Updates a specific memory. Only the owning principal can update their memories.

### DeleteMemory

```
rpc DeleteMemory(DeleteMemoryRequest) returns (DeleteMemoryResponse)
```

| REST | `DELETE /v1/memories/{id}` |
|------|----------------------------|

**Request**: `{id}`
**Response**: `{}`
**Permission**: authenticated
**Behavior**: Soft-deletes the memory by setting its status to `invalidated`. Only the owning principal can delete their memories.

### ListMemories

```
rpc ListMemories(ListMemoriesRequest) returns (ListMemoriesResponse)
```

| REST | `GET /v1/memories/{scope}/list` |
|------|----------------------------------|

**Request**: `{scope, include_invalidated}`
**Response**: `{memories[]}`
**Permission**: authenticated
**Behavior**: Lists all memories for the calling principal in the given scope. If `include_invalidated` is true, also returns memories with status `invalidated` for audit purposes.

### ListScopes

```
rpc ListScopes(ListScopesRequest) returns (ListScopesResponse)
```

| REST | `GET /v1/memories:scopes` |
|------|---------------------------|

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

| REST | `GET /v1/jobs/{id}` |
|------|---------------------|

**Request**: `{id}`
**Response**: `{job}`
**Permission**: authenticated
**Behavior**: Returns job details. Any authenticated user can view jobs for documents they have read access to.

### ListJobs

```
rpc ListJobs(ListJobsRequest) returns (ListJobsResponse)
```

| REST | `GET /v1/jobs` |
|------|----------------|

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

| REST | `GET /v1/health` |
|------|------------------|

**Request**: `{}`
**Response**: `{status, version}`
**Permission**: none (unauthenticated)
**Behavior**: Returns server health and version. Checks connectivity to the metadata store and vector backend.

### CreateSystemAccount

```
rpc CreateSystemAccount(CreateSystemAccountRequest) returns (CreateSystemAccountResponse)
```

| REST | `POST /v1/admin/accounts` |
|------|---------------------------|

**Request**: `{name, description}`
**Response**: `{account, api_key}`
**Permission**: admin (via bootstrap key or existing admin system account)
**Behavior**: Creates a system account with principal `system:{name}`. Generates an API key (`creel_ak_...`). The key is returned exactly once and is never retrievable again.

### ListSystemAccounts

```
rpc ListSystemAccounts(ListSystemAccountsRequest) returns (ListSystemAccountsResponse)
```

| REST | `GET /v1/admin/accounts` |
|------|--------------------------|

**Request**: `{}`
**Response**: `{accounts[]}`
**Permission**: admin
**Behavior**: Lists all system accounts without their keys.

### DeleteSystemAccount

```
rpc DeleteSystemAccount(DeleteSystemAccountRequest) returns (DeleteSystemAccountResponse)
```

| REST | `DELETE /v1/admin/accounts/{id}` |
|------|---------------------------------|

**Request**: `{id}`
**Permission**: admin
**Behavior**: Deletes the system account and all its keys. Topic grants referencing this account's principal remain as orphaned rows (no effect on access).

### RotateKey

```
rpc RotateKey(RotateKeyRequest) returns (RotateKeyResponse)
```

| REST | `POST /v1/admin/accounts/{account_id}/rotate` |
|------|------------------------------------------------|

**Request**: `{account_id, grace_period_seconds}`
**Response**: `{api_key}`
**Permission**: admin
**Behavior**: Generates a new key and returns it. The old key enters `grace_period` status and remains valid for `grace_period_seconds`. If `grace_period_seconds` is 0, the old key is revoked immediately.

### RevokeKey

```
rpc RevokeKey(RevokeKeyRequest) returns (RevokeKeyResponse)
```

| REST | `POST /v1/admin/accounts/{account_id}/revoke` |
|------|------------------------------------------------|

**Request**: `{account_id}`
**Permission**: admin
**Behavior**: Immediately invalidates all active keys for this system account.

### GetStats

```
rpc GetStats(GetStatsRequest) returns (GetStatsResponse)
```

| REST | `GET /v1/admin/stats` |
|------|----------------------|

**Request**: `{}`
**Response**: `{api_key_configs, llm_configs, embedding_configs, extraction_prompt_configs, topics, system_accounts, documents, chunks, memories}` (all `int64`)
**Permission**: admin
**Behavior**: Returns row counts for all major entity tables in a single database query. Used by the admin dashboard for the overview page.

---

## ConfigService

Managed server configuration. All ConfigService RPCs require a system account. Each config type follows the same CRUD+SetDefault pattern. `is_default` is a singleton flag; setting a new default clears the previous one.

### API Key Configs

Store third-party API keys (e.g. OpenAI) encrypted at rest. Referenced by LLM and embedding configs.

| RPC | REST |
|-----|------|
| CreateAPIKeyConfig | `POST /v1/config/apikey` |
| GetAPIKeyConfig | `GET /v1/config/apikey/{id}` |
| ListAPIKeyConfigs | `GET /v1/config/apikey` |
| UpdateAPIKeyConfig | `PATCH /v1/config/apikey/{id}` |
| DeleteAPIKeyConfig | `DELETE /v1/config/apikey/{id}` |
| SetDefaultAPIKeyConfig | `POST /v1/config/apikey/{id}/default` |

**Create request**: `{name, provider, api_key, is_default}`. The `api_key` field is write-only; it is encrypted at rest and never returned.
**Update request**: `{id, name, provider, api_key}`. If `api_key` is set, replaces the stored key.

### LLM Configs

Configure which LLM to use for compaction, memory extraction, and semantic chunking.

| RPC | REST |
|-----|------|
| CreateLLMConfig | `POST /v1/config/llm` |
| GetLLMConfig | `GET /v1/config/llm/{id}` |
| ListLLMConfigs | `GET /v1/config/llm` |
| UpdateLLMConfig | `PATCH /v1/config/llm/{id}` |
| DeleteLLMConfig | `DELETE /v1/config/llm/{id}` |
| SetDefaultLLMConfig | `POST /v1/config/llm/{id}/default` |

**Create request**: `{name, provider, model, parameters, api_key_config_id, is_default}`.

### Embedding Configs

Configure which embedding model to use for vector search.

| RPC | REST |
|-----|------|
| CreateEmbeddingConfig | `POST /v1/config/embedding` |
| GetEmbeddingConfig | `GET /v1/config/embedding/{id}` |
| ListEmbeddingConfigs | `GET /v1/config/embedding` |
| UpdateEmbeddingConfig | `PATCH /v1/config/embedding/{id}` |
| DeleteEmbeddingConfig | `DELETE /v1/config/embedding/{id}` |
| SetDefaultEmbeddingConfig | `POST /v1/config/embedding/{id}/default` |

**Create request**: `{name, provider, model, dimensions, api_key_config_id, is_default}`.
**Update request**: `{id, name, api_key_config_id}`. Provider, model, and dimensions cannot be changed (would require re-embedding all vectors).

### Extraction Prompt Configs

Custom prompts for the document extraction pipeline.

| RPC | REST |
|-----|------|
| CreateExtractionPromptConfig | `POST /v1/config/prompt` |
| GetExtractionPromptConfig | `GET /v1/config/prompt/{id}` |
| ListExtractionPromptConfigs | `GET /v1/config/prompt` |
| UpdateExtractionPromptConfig | `PATCH /v1/config/prompt/{id}` |
| DeleteExtractionPromptConfig | `DELETE /v1/config/prompt/{id}` |
| SetDefaultExtractionPromptConfig | `POST /v1/config/prompt/{id}/default` |

**Create request**: `{name, prompt, description, is_default}`.

### Vector Backend Configs

Configure vector storage backends (pgvector, Qdrant, Weaviate). Topics can reference a specific config to route their vectors to a different store.

| RPC | REST |
|-----|------|
| CreateVectorBackendConfig | `POST /v1/config/vector-backend` |
| GetVectorBackendConfig | `GET /v1/config/vector-backend/{id}` |
| ListVectorBackendConfigs | `GET /v1/config/vector-backend` |
| UpdateVectorBackendConfig | `PATCH /v1/config/vector-backend/{id}` |
| DeleteVectorBackendConfig | `DELETE /v1/config/vector-backend/{id}` |
| SetDefaultVectorBackendConfig | `POST /v1/config/vector-backend/{id}/default` |

**Create request**: `{name, backend, config, is_default}`. `backend` is the type (e.g. "pgvector", "qdrant", "weaviate"). `config` is a key-value map of backend-specific connection settings.
**Update request**: `{id, name, config}`. Backend type cannot be changed post-creation.
