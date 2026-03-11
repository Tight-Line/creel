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

The repository ships with a pre-configured dev API key. Source `.env` to set `CREEL_GRPC_ENDPOINT` and `CREEL_API_KEY`:

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

## 5. Create a topic and upload a document

```bash
bin/creel-cli topic create my-notes "My Notes"
```

Create a sample document and upload it:

```bash
cat > /tmp/sample-notes.txt << 'EOF'
Team standup notes - March 2026

Sprint goals: finalize the v2 API migration and ship the new dashboard.

Action items:
- Sarah: update the authentication flow to support OIDC by Friday
- James: run load tests against the staging cluster; target 500 req/s
- Priya: draft the migration guide for existing API consumers

Decisions:
- We will deprecate the v1 endpoints on June 1 with a 90-day sunset notice
- The new rate limiter will default to 100 req/min per API key
- Dashboard will ship behind a feature flag for the first two weeks

Blockers:
- Staging database needs a pgvector extension upgrade before load tests
- The OIDC provider sandbox is down; Sarah is waiting on IT
EOF

bin/creel-cli upload \
  --topic my-notes \
  --file /tmp/sample-notes.txt \
  --name "Team Standup Notes" \
  --author "Nick"
```

Creel processes the document asynchronously. Check the job status:

```bash
bin/creel-cli jobs list --topic my-notes
```

## 6. Search

Once all jobs show `completed`, search for content:

```bash
bin/creel-cli search --topic my-notes --query "what are the action items" --top-k 5
```

## 7. Try creel-chat (optional)

creel-chat calls OpenAI directly for embeddings and chat (separate from the server-side config). It uses the same `OPENAI_API_KEY` you exported in step 4:

```bash
# --topic my-notes scopes RAG search to that topic.
# --cross-topic searches all your topics instead.
bin/creel-chat --topic my-notes
```

Ask it about your uploaded document:

```
you> What are the blockers from the standup?
assistant> The blockers from the standup are:

- Staging database needs a `pgvector` extension upgrade before load tests
- The OIDC provider sandbox is down; Sarah is waiting on IT
```

Conversation context persists across sessions. Have a conversation that mentions something specific, then exit and start a new session:

```
you> I need to get the pgvector upgrade done right away. I'm leaving early for my son's game.
assistant> Makes sense — the pgvector upgrade on staging is your top priority before you
head out. ...
```

Exit with Ctrl-D, then start a fresh session:

```bash
bin/creel-chat --cross-topic
```

```
you> What's on my docket?
assistant> The main thing I have from prior context is:

- Get pgvector installed/upgraded on staging before load tests
- You'd said that was your top priority because you were leaving early for your son's game
```

The new session recalled context from the previous one via RAG over stored conversation chunks.

See [CREEL_CHAT.md](CREEL_CHAT.md) for Anthropic, Ollama, and other provider options. The [Fullstart](FULLSTART.md) guide walks through memory, cross-topic search, and more.

## Next steps

- [Fullstart](FULLSTART.md): in-depth walkthrough of every feature (search, memory, compaction, chat)
- [Concepts](CONCEPTS.md): covers document processing, memory, and citations
- [API Reference](API_REFERENCE.md): all 63 RPCs
- [Development](DEVELOPMENT.md): set up a dev environment
- [Deployment](DEPLOYMENT.md): production deployment with Helm
