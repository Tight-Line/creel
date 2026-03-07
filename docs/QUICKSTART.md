# Quickstart

Get Creel running locally in under 5 minutes.

## Prerequisites

- Go 1.26+
- Docker (for PostgreSQL/pgvector)

## 1. Start PostgreSQL and the Creel server

```bash
docker compose up -d
```

This starts pgvector/pgvector:pg17 on port 5432 and the Creel server on port 8443 (gRPC). Migrations run automatically on first startup.

## 2. Build the CLI tools

```bash
make build
```

This produces `bin/creel-cli` and `bin/creel-chat`.

## 3. Source the dev environment

The repository ships with a pre-configured dev API key. Source `.env` to set `CREEL_ENDPOINT` and `CREEL_API_KEY`:

```bash
source .env
```

No need to generate keys or create a config file for local development.

## 4. Create a topic

```bash
bin/creel-cli topic create my-notes "My Notes"
```

## 5. Search

The search command reads an embedding vector from stdin (JSON array of floats). For example, with a pre-computed embedding:

```bash
echo '[0.1, 0.2, 0.3, ...]' | bin/creel-cli search --top-k 5
```

For a more practical workflow, use `creel-chat` which handles embedding computation automatically.

## 6. Try creel-chat (optional)

For an interactive demo with LLM-powered conversation memory:

```bash
export OPENAI_API_KEY="<your OpenAI key>"  # for LLM + embeddings
bin/creel-chat
```

OpenAI is the default provider for both chat (GPT-5.4) and embeddings (text-embedding-3-small). See [CREEL_CHAT.md](CREEL_CHAT.md) for Anthropic, Ollama, and other options.

## Next steps

- [Concepts](CONCEPTS.md): understand the data model and design
- [API Reference](API_REFERENCE.md): all 28 RPCs
- [Development](DEVELOPMENT.md): set up a dev environment
- [Deployment](DEPLOYMENT.md): production deployment with Helm
