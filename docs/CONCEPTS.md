# Concepts

Core data model and design for integrators.

## Topic / Document / Chunk hierarchy

Creel organizes memory in three levels:

- **Topic**: the permission boundary. Each topic has a slug, a name, and a set of grants controlling who can read, write, or administer it. Use topics to separate concerns (e.g., one topic per project, per user, or per agent).
- **Document**: a container within a topic. Documents group related chunks (e.g., a single conversation session, an ingested file, a compaction result). Documents have a slug, metadata, and a parent topic.
- **Chunk**: the atomic unit of memory. Each chunk has text content, an embedding vector, metadata, and a sequence number within its document. Chunks are what search returns.

The hierarchy is strict: every chunk belongs to a document, every document belongs to a topic. Deleting a topic cascades to its documents and chunks.

## Principals, authentication, and RBAC

Every API request carries a bearer token (OIDC JWT or API key). The server resolves the caller's **principal** from the token:

- OIDC tokens: the principal is the value of the configured claim (default: `sub`).
- API keys: the principal is the `principal` field from the key configuration.

**Topic grants** control access. Each grant maps a principal (or group) to a role on a topic:

- **read**: can search and retrieve chunks in the topic.
- **write**: can create documents and ingest chunks.
- **admin**: can manage grants and delete the topic.

The topic creator automatically gets admin. There are no document-level or chunk-level grants; access is always at the topic level.

## Document processing

Creel offers two ingestion paths:

- **Managed path** (default): Upload a document (PDF, HTML, plain text, or URL). Creel extracts text, chunks it, and computes embeddings in the background using the topic's configured providers and strategies. The client does not need to know about chunking, embedding, or extraction. Supported content types: `text/plain`, `text/html`, and `application/pdf`.
- **Direct path** (power users): Push pre-chunked, pre-embedded content via `IngestChunks`. This skips the worker pipeline entirely. Useful for users who have their own processing stack.

## Embeddings

Creel stores embedding vectors alongside chunks for semantic search. The default path is now server-side: when a document is uploaded via the managed path, the server computes embeddings using the topic's configured embedding provider. The direct ingestion path (`IngestChunks` with pre-computed embeddings) is still available for power users.

Key constraints:

- All chunks in a topic must use the same embedding dimension.
- The search query embedding must match the topic's dimension.
- Dimension is set by the first chunk ingested into a topic.

## Document metadata & citations

Documents carry structured citation metadata: name, URL, author, publication date, and custom JSONB fields. All RAG search results include the parent document's citation metadata alongside each chunk, enabling LLMs to generate properly cited responses without additional lookups.

## Chunking strategies

Topics support two chunking strategies, controlled by the `chunking_strategy` JSONB field:

- **Fixed-size** (default): splits text into windows of `chunk_size` characters (default 2048) with `chunk_overlap` characters of overlap (default 200). Good for most use cases.
- **Semantic**: set `type` to `"semantic"` to use the topic's LLM to split text into topically coherent sections. The LLM identifies natural break points in the text and returns a JSON array of chunks. Requires an LLM provider to be configured on the server.

Example `chunking_strategy` values:

```json
{"type": "fixed", "chunk_size": 1024, "chunk_overlap": 100}
{"type": "semantic"}
```

When `chunking_strategy` is NULL or omitted, fixed-size chunking with server defaults is used.

## Search modes

Creel supports two retrieval modes:

### RAG (semantic search)

`Search` performs vector similarity search across one or more topics. Results are ranked by cosine similarity and filtered by the caller's ACL. Supports metadata filtering. Search results include the parent document's citation metadata for proper attribution.

Use RAG for: finding relevant context across all stored memory.

### Context (temporal ordering)

`GetContext` retrieves chunks in sequence order within a document, providing temporal context for a conversation or session. Supports `last_n` (return the last N active chunks) and `since` (return chunks created at or after a timestamp) filters.

Use context for: reconstructing conversation history, providing session continuity.

creel-chat combines both modes as two-layer retrieval: current session history via `GetContext` (temporal) plus relevant chunks from other sessions via `Search` with `exclude_document_ids` (semantic).

## Linking

Chunks can be linked to other chunks, even across topic boundaries. Links enable knowledge graph traversal during search.

- **Manual links**: created explicitly via `CreateLink`.
- **Automatic links**: created on ingestion when `auto_link_on_ingest` is enabled, based on embedding similarity exceeding a threshold.
- **Compaction transfers**: when chunks are compacted, their links transfer to the summary chunk.

Link traversal respects ACLs: if the caller lacks read access to the target topic, the link is not followed.

## Memory

Creel provides a per-principal memory system for maintaining long-term facts about users and agents.

- Memory belongs to a principal, not a topic. Each principal can have multiple named scopes (default, work, home, etc.).
- Memories are natural language fact statements (e.g., "User specializes in thrombosis research") maintained automatically by background workers.
- Clients send conversation messages via `AddMessages`, which enqueues `memory_messages` jobs. These jobs use the configured LLM to extract candidate facts from the conversation, then create `memory_maintenance` jobs that resolve conflicts with existing memories (ADD new facts, UPDATE existing, DELETE contradictions, or NOOP).
- Clients fetch memories via `GetMemories`, which supports multi-scope retrieval. Pass one or more scope names to filter, or omit scopes to retrieve all memories for the principal. Include the results in the system prompt so the LLM has persistent knowledge about the user.
- Clients can also explicitly add, update, or delete memories. `AddMemory` queues a `memory_maintenance` job that runs the same LLM-based deduplication as automatic extraction, so explicitly added memories are checked for conflicts with existing facts before being stored.

## Compaction

Over time, conversation documents accumulate many chunks. Compaction summarizes older chunks into fewer, denser ones. Workers handle compaction automatically in the background based on topic policies, using the topic's LLM configuration to generate summaries. Manual compaction via the API is preserved for the direct ingestion path.

Creel manages the bookkeeping: replacing originals with summaries and transferring all links. The `retain_compacted_chunks` config option controls whether originals are preserved or deleted.

## Addressing scheme

Every chunk has a unique address:

```
creel://{topic_slug}/{document_slug}/{chunk_sequence}
```

This scheme is used in link references and search results to provide stable, human-readable identifiers.
