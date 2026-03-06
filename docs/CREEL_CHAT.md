# creel-chat

An interactive REPL demo agent that uses Creel for conversation memory.

## What it does

creel-chat is a terminal-based chat interface that demonstrates Creel's memory capabilities. Each conversation turn:

1. Takes your input.
2. Embeds the message and searches Creel for semantically relevant context from previous conversations.
3. Builds a prompt with retrieved context and current session history.
4. Sends the prompt to an LLM for a response.
5. Stores both the user message and assistant response as chunks in Creel with embeddings.

Every message is persisted. Conversations can be resumed later by document ID.

## Prerequisites

- A running Creel server (see [Quickstart](QUICKSTART.md))
- A Creel API key
- An LLM API key (Anthropic or OpenAI)
- An embedding API key (OpenAI) or a local Ollama instance

## CLI flags

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--endpoint` | `CREEL_ENDPOINT` | `localhost:8443` | Creel gRPC endpoint |
| `--api-key` | `CREEL_API_KEY` | (required) | Creel API key |
| `--tls` | | `false` | Use TLS for gRPC connection |
| `--provider` | | `anthropic` | Chat LLM provider: `anthropic` or `openai` |
| `--model` | | (provider default) | Override LLM model name |
| `--embed-provider` | | `openai` | Embedding provider: `openai` or `ollama` |
| `--embed-model` | | `text-embedding-3-small` | Embedding model name |
| `--ollama-url` | | `http://localhost:11434` | Ollama API base URL |
| `--topic` | | `creel-chat` | Topic slug for conversation storage |
| `--top-k` | | `5` | Number of context chunks to retrieve |
| `--resume` | | | Resume previous session by document ID |

## Provider setup

### Anthropic (default LLM)

```bash
export ANTHROPIC_API_KEY="your-key"
bin/creel-chat --api-key "$CREEL_API_KEY"
```

Default model: Claude Sonnet 4.6.

### OpenAI (LLM)

```bash
export OPENAI_API_KEY="your-key"
bin/creel-chat --api-key "$CREEL_API_KEY" --provider openai
```

Default model: GPT-5.4.

### OpenAI (embeddings, default)

```bash
export OPENAI_API_KEY="your-key"
```

Uses text-embedding-3-small (1536 dimensions) by default.

### Ollama (embeddings)

Start Ollama locally, then:

```bash
bin/creel-chat --api-key "$CREEL_API_KEY" --embed-provider ollama --embed-model nomic-embed-text
```

## Session management

On exit, creel-chat prints the document ID for the current session:

```
Session document ID: abc123-def456
To resume: bin/creel-chat --api-key "$CREEL_API_KEY" --resume abc123-def456
```

Use `--resume` to continue a previous conversation. The session history is loaded from Creel and the LLM receives the full prior context.

## How retrieval works

Currently, creel-chat uses **RAG-only** retrieval: each user message is embedded and used to search all chunks in the topic for semantically relevant context. This works well for pulling in knowledge from previous sessions but has a known limitation: it does not preserve temporal ordering within the current session beyond what is held in local memory.

The planned improvement is two-layer retrieval:

1. **Temporal**: always include the full current session history in sequence order.
2. **RAG**: search other sessions/topics for semantically relevant long-term context.

This requires the `GetContext` RPC (Phase 3) or a `ListChunks`-by-document endpoint.

## Known limitations

- RAG-only retrieval may miss recent conversational context if the session grows long.
- Embeddings are computed client-side; both user messages and assistant responses are embedded individually.
- No streaming: the full LLM response is generated before display.
- No tool use or function calling; creel-chat is a simple conversational demo.
