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

## Embeddings

Creel stores embedding vectors alongside chunks for semantic search. Currently, embeddings are **client-side**: the caller computes embeddings before ingesting chunks via `IngestChunks`.

Key constraints:

- All chunks in a topic must use the same embedding dimension.
- The search query embedding must match the topic's dimension.
- Dimension is set by the first chunk ingested into a topic.

Supported embedding providers (in creel-chat):

- OpenAI (text-embedding-3-small, 1536 dimensions)
- Ollama (local models, dimension varies by model)

Server-side embedding is planned for Phase 5.

## Search modes

Creel supports two retrieval modes:

### RAG (semantic search)

`Search` performs vector similarity search across one or more topics. Results are ranked by cosine similarity and filtered by the caller's ACL. Supports metadata filtering.

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

## Compaction

Over time, conversation documents accumulate many chunks. Compaction lets the client summarize older chunks into fewer, denser ones:

1. The client reads the chunks to compact.
2. The client generates a summary (typically via LLM).
3. The client calls `Compact` with the original chunk IDs and the new summary chunks.
4. Creel replaces the originals with the summaries, transferring all links.

Compaction is client-driven: Creel manages the bookkeeping, but the client decides when and how to summarize. The `retain_compacted_chunks` config option controls whether originals are preserved or deleted.

## Addressing scheme

Every chunk has a unique address:

```
creel://{topic_slug}/{document_slug}/{chunk_sequence}
```

This scheme is used in link references and search results to provide stable, human-readable identifiers.
