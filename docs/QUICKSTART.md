# Quickstart

Get Creel running locally in under 5 minutes.

## Prerequisites

- Go 1.26+
- Docker (for PostgreSQL/pgvector)
- `jq` (for parsing JSON responses)
- An OpenAI API key (for embeddings, search, and chat)

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

## 4. Configure an embedding provider

Register your OpenAI API key so the server can compute embeddings for search:

```bash
export OPENAI_API_KEY="<your OpenAI key>"

bin/creel-cli config apikey create \
  --name openai --provider openai --api-key "$OPENAI_API_KEY" --default

APIKEY_ID=$(bin/creel-cli config apikey list | jq -r '.configs[0].id')

bin/creel-cli config embedding create \
  --name openai-embed --provider openai --model text-embedding-3-small \
  --dimensions 1536 --apikey-config "$APIKEY_ID" --default
```

## 5. Create a topic

```bash
bin/creel-cli topic create my-notes "My Notes"
```

## 6. Upload a document and search

Upload a document to your topic:

```bash
bin/creel-cli upload --topic my-notes --file notes.txt --name "Meeting Notes" --author "Nick"
```

Creel processes the document asynchronously. Check the job status:

```bash
bin/creel-cli jobs list --topic my-notes
```

Once processing completes, search for content:

```bash
bin/creel-cli search --topic my-notes --query "action items from meeting" --top-k 5
```

## 7. Try creel-chat (optional)

For an interactive demo with LLM-powered conversation memory:

```bash
bin/creel-chat
```

OpenAI is the default provider for both chat and embeddings. See [CREEL_CHAT.md](CREEL_CHAT.md) for Anthropic, Ollama, and other options.

## Next steps

- [Fullstart](FULLSTART.md): in-depth walkthrough of every feature (search, memory, compaction, chat)
- [Concepts](CONCEPTS.md): covers document processing, memory, and citations
- [API Reference](API_REFERENCE.md): all 63 RPCs
- [Development](DEVELOPMENT.md): set up a dev environment
- [Deployment](DEPLOYMENT.md): production deployment with Helm
