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

## 5. Upload a document and search

Upload a document to your topic:

```bash
bin/creel-cli upload --topic my-notes --file notes.pdf --name "Meeting Notes" --author "Nick"
```

Creel processes the document asynchronously. Check the job status:

```bash
bin/creel-cli jobs list --topic my-notes
```

Once processing completes, search for content:

```bash
bin/creel-cli search --topic my-notes --query "action items from meeting" --top-k 5
```

Note: search with `--query` requires an embedding provider to be configured on the server. See the [Fullstart](FULLSTART.md) guide for how to set up providers.

## 6. Try creel-chat (optional)

For an interactive demo with LLM-powered conversation memory:

```bash
export OPENAI_API_KEY="<your OpenAI key>"  # for LLM + embeddings
bin/creel-chat
```

OpenAI is the default provider for both chat (GPT-5.4) and embeddings (text-embedding-3-small). See [CREEL_CHAT.md](CREEL_CHAT.md) for Anthropic, Ollama, and other options.

## Next steps

- [Fullstart](FULLSTART.md): in-depth walkthrough of every feature (search, memory, compaction, chat)
- [Concepts](CONCEPTS.md): covers document processing, memory, and citations
- [API Reference](API_REFERENCE.md): all 63 RPCs
- [Development](DEVELOPMENT.md): set up a dev environment
- [Deployment](DEPLOYMENT.md): production deployment with Helm
