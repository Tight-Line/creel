# creel-chat

An interactive REPL demo agent that uses Creel for conversation memory.

## What it does

creel-chat is a terminal-based chat interface that demonstrates Creel's memory capabilities. Each conversation turn:

1. Takes your input.
2. Embeds the message and searches Creel for semantically relevant context from **other** sessions (RAG layer).
3. Combines the RAG context with the full current session history (temporal layer), per-principal memories, and document citations into a structured prompt.
4. Streams the LLM response to the terminal as tokens arrive.
5. Stores both the user message and assistant response as chunks in Creel with embeddings.

Every message is persisted. Conversations can be resumed later by document ID.

## Prerequisites

- A running Creel server (see [Quickstart](QUICKSTART.md))
- A Creel API key
- An OpenAI API key (used for both LLM and embeddings by default)

## CLI flags

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--endpoint` | `CREEL_GRPC_ENDPOINT` | `http://127.0.0.1:8443` | gRPC endpoint URL (`https://` for TLS, `http://` for plaintext) |
| `--api-key` | `CREEL_API_KEY` | (required) | Creel API key |
| `--verify-tls` | `CREEL_VERIFY_TLS` | `true` | Verify TLS certificates (set to `false` for self-signed certs) |
| `--authority` | `CREEL_GRPC_AUTHORITY` | | Override the `:authority` header (for routing through proxies) |
| `--provider` | | `openai` | Chat LLM provider: `openai` or `anthropic` |
| `--model` | | (provider default) | Override LLM model name |
| `--embed-provider` | | `openai` | Embedding provider: `openai` or `ollama` |
| `--embed-model` | | `text-embedding-3-small` | Embedding model name |
| `--ollama-url` | | `http://localhost:11434` | Ollama API base URL |
| `--topic` | | `creel-chat` | Topic slug for conversation storage |
| `--top-k` | | `5` | Number of RAG context chunks to retrieve |
| `--resume` | | | Resume previous session by document ID |
| `--memory-scope` | | `default` | Memory scope for writing new memories (used by `AddMessages` and `/remember`) |
| `--memory-read-scopes` | | (all scopes) | Comma-separated list of scopes to read at session start. If omitted, all scopes are fetched. |
| `--cross-topic` | | `false` | Search across all accessible topics instead of just the current one |

## Provider setup

### OpenAI (default LLM + embeddings)

```bash
export OPENAI_API_KEY="your-key"
bin/creel-chat --api-key "$CREEL_API_KEY"
```

Default LLM model: GPT-5.4. Default embedding model: text-embedding-3-small (1536 dimensions).

### Anthropic (alternate LLM)

```bash
export ANTHROPIC_API_KEY="your-key"
bin/creel-chat --api-key "$CREEL_API_KEY" --provider anthropic
```

Default model: Claude Sonnet 4.6. Still requires OpenAI (or Ollama) for embeddings.

### Ollama (local embeddings)

Start Ollama locally, then:

```bash
bin/creel-chat --api-key "$CREEL_API_KEY" --embed-provider ollama --embed-model nomic-embed-text
```

## Session management

On exit, creel-chat prints the document ID for the current session:

```
To resume this session:
  creel-chat --resume abc123-def456 --topic creel-chat
```

Use `--resume` to continue a previous conversation. The full session history is loaded from Creel via the `GetContext` RPC and replayed to the LLM as user/assistant messages.

## Streaming responses

LLM responses are streamed to the terminal as tokens arrive. Both OpenAI and Anthropic providers use server-sent events (SSE) for streaming. The full response is collected and stored as a chunk after streaming completes.

## Memory integration

At session start (and on resume), creel-chat fetches per-principal memories via `GetMemories` and includes them in the system prompt as a "What I know about you" section. This gives the LLM persistent knowledge about the user across sessions. By default, all scopes are fetched. Use `--memory-read-scopes` to restrict which scopes are included (e.g., `--memory-read-scopes fishing,skiing`).

After each conversation turn, creel-chat automatically calls `AddMessages` with the user message and assistant response. The server enqueues `memory_messages` jobs that extract facts from the conversation via the configured LLM, then creates `memory_maintenance` jobs to deduplicate against existing memories. No special topic configuration is required; memory extraction is driven entirely by the `AddMessages` call.

Use `--memory-scope` to control which scope new memories are written to. The default scope is `default`.

## REPL commands

In addition to normal conversation, creel-chat supports slash commands:

- `/upload <filepath>` uploads a local file to Creel for processing via the managed document pipeline. Displays the document ID and job ID. Processing (extraction, chunking, embedding) runs in the background.
- `/remember <text>` queues a memory fact for processing in the current scope. The fact goes through LLM-based deduplication before being stored. The LLM will see this fact in future sessions.
- `/forget <text>` searches for the best-matching memory and deletes it. Prints what was forgotten.

## Cross-topic RAG

By default, creel-chat only searches the current topic for RAG context. Use `--cross-topic` to search across all topics the principal has access to. The current session document is always excluded from results.

## Citation display

When search results include document citation metadata (name, author, URL), creel-chat formats this information in the RAG context section of the prompt. The LLM can reference these citations when answering questions. Citations appear as:

```
[Source: "Document Name", by Author, https://example.com]
[role]: content text
```

## How retrieval works

creel-chat uses **two-layer retrieval**:

1. **Temporal (session history)**: On resume, `GetContext` loads all prior chunks from the session document in sequence order. These are sent as the user/assistant message history in the LLM prompt. During a live session, messages accumulate in the same buffer. This layer is the authoritative record of the conversation.

2. **RAG (cross-session context)**: Each turn, the user's message is embedded and used to search for semantically relevant chunks from **other** sessions in the same topic (or all accessible topics with `--cross-topic`). The current session document is excluded from RAG search via `exclude_document_ids`, so all top-K slots are filled with genuinely cross-session context.

The system prompt explicitly tells the LLM about both layers: session history is authoritative and verbatim; RAG context is supplementary background from other sessions.

## Known limitations

- Embeddings are computed client-side; both user messages and assistant responses are embedded individually. Server-side embedding via the managed document processing path is available for non-chat document ingestion.
- No tool use or function calling; creel-chat is a simple conversational demo.
- `GetContext` loads the entire session history on resume. Very long sessions may eventually need `last_n` filtering or compaction (not yet implemented).
