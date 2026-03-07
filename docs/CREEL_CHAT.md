# creel-chat

An interactive REPL demo agent that uses Creel for conversation memory.

## What it does

creel-chat is a terminal-based chat interface that demonstrates Creel's memory capabilities. Each conversation turn:

1. Takes your input.
2. Embeds the message and searches Creel for semantically relevant context from **other** sessions (RAG layer).
3. Combines the RAG context with the full current session history (temporal layer) into a structured prompt.
4. Sends the prompt to an LLM for a response.
5. Stores both the user message and assistant response as chunks in Creel with embeddings.

Every message is persisted. Conversations can be resumed later by document ID.

## Prerequisites

- A running Creel server (see [Quickstart](QUICKSTART.md))
- A Creel API key
- An OpenAI API key (used for both LLM and embeddings by default)

## CLI flags

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--endpoint` | `CREEL_ENDPOINT` | `localhost:8443` | Creel gRPC endpoint |
| `--api-key` | `CREEL_API_KEY` | (required) | Creel API key |
| `--tls` | | `false` | Use TLS for gRPC connection |
| `--provider` | | `openai` | Chat LLM provider: `openai` or `anthropic` |
| `--model` | | (provider default) | Override LLM model name |
| `--embed-provider` | | `openai` | Embedding provider: `openai` or `ollama` |
| `--embed-model` | | `text-embedding-3-small` | Embedding model name |
| `--ollama-url` | | `http://localhost:11434` | Ollama API base URL |
| `--topic` | | `creel-chat` | Topic slug for conversation storage |
| `--top-k` | | `5` | Number of RAG context chunks to retrieve |
| `--resume` | | | Resume previous session by document ID |

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

## How retrieval works

creel-chat uses **two-layer retrieval**:

1. **Temporal (session history)**: On resume, `GetContext` loads all prior chunks from the session document in sequence order. These are sent as the user/assistant message history in the LLM prompt. During a live session, messages accumulate in the same buffer. This layer is the authoritative record of the conversation.

2. **RAG (cross-session context)**: Each turn, the user's message is embedded and used to search for semantically relevant chunks from **other** sessions in the same topic. The current session document is excluded from RAG search via `exclude_document_ids`, so all top-K slots are filled with genuinely cross-session context.

The system prompt explicitly tells the LLM about both layers: session history is authoritative and verbatim; RAG context is supplementary background from other sessions.

## Known limitations

- Embeddings are computed client-side; both user messages and assistant responses are embedded individually.
- No streaming: the full LLM response is generated before display.
- No tool use or function calling; creel-chat is a simple conversational demo.
- `GetContext` loads the entire session history on resume. Very long sessions may eventually need `last_n` filtering or compaction (not yet implemented).
