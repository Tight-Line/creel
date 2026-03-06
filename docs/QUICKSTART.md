# Quickstart

Get Creel running locally in under 5 minutes.

## Prerequisites

- Go 1.24+
- Docker (for PostgreSQL/pgvector)

## 1. Start PostgreSQL

```bash
docker compose up -d postgres
```

This starts pgvector/pgvector:pg17 on port 5432 with user/password `creel`.

## 2. Build

```bash
make build
```

This produces `bin/creel`, `bin/creel-cli`, and `bin/creel-chat`.

## 3. Generate a bootstrap API key

```bash
bin/creel bootstrap-key --name quickstart
```

This prints a key hash and a plaintext API key. Copy the key hash.

## 4. Create a config file

Create `creel.yaml`:

```yaml
server:
  grpc_port: 8443

auth:
  api_keys:
    - name: quickstart
      key_hash: "<paste key hash here>"
      principal: "quickstart-user"

metadata:
  postgres_url: "postgres://creel:creel@localhost:5432/creel?sslmode=disable"

vector_backend:
  type: pgvector
```

## 5. Start the server

```bash
bin/creel --config creel.yaml --migrate
```

The `--migrate` flag runs database migrations on first startup. The server listens on port 8443 (gRPC).

## 6. Create a topic and ingest chunks

```bash
export CREEL_ENDPOINT=localhost:8443
export CREEL_API_KEY="<your plaintext API key>"

# Create a topic
bin/creel-cli topic create --slug my-notes --name "My Notes"

# Ingest some chunks (embeddings must be pre-computed)
bin/creel-cli topic ingest --slug my-notes --file chunks.jsonl
```

## 7. Search

```bash
bin/creel-cli search --slug my-notes --query "your search query" --top-k 5
```

## 8. Try creel-chat (optional)

For an interactive demo with LLM-powered conversation memory:

```bash
export OPENAI_API_KEY="<your OpenAI key>"  # for embeddings
export ANTHROPIC_API_KEY="<your Anthropic key>"  # for chat

bin/creel-chat --api-key "$CREEL_API_KEY"
```

See [CREEL_CHAT.md](CREEL_CHAT.md) for full setup instructions.

## Next steps

- [Concepts](CONCEPTS.md): understand the data model and design
- [API Reference](../API_REFERENCE.md): all 28 RPCs
- [Development](DEVELOPMENT.md): set up a dev environment
- [Deployment](DEPLOYMENT.md): production deployment with Helm
