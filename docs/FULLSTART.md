# Fullstart

A hands-on walkthrough that exercises every major Creel feature: provider configuration, document upload, processing pipeline, search with citations, per-principal memory, compaction, cross-topic RAG, and interactive chat.

The [Quickstart](QUICKSTART.md) gets you running in 5 minutes. This guide goes deeper; expect 20-30 minutes.

## Prerequisites

- Go 1.26+
- Docker (for PostgreSQL/pgvector and the Creel server)
- `curl` and `jq` (for REST API calls)
- An OpenAI API key (required for real embeddings, search, and chat)

## 1. Start the stack and build tools

```bash
docker compose up -d
make build
source .env
```

Verify everything is healthy:

```bash
bin/creel-cli health
```

You should see `"status":"ok"` and a version string.

## 2. Configure providers

Register your OpenAI API key so the server can compute real embeddings and power LLM-based memory extraction. Without this, the server falls back to stub providers that produce deterministic but non-semantic embeddings.

```bash
# Store your OpenAI API key (encrypted at rest)
bin/creel-cli config apikey create \
  --name "openai" \
  --provider openai \
  --api-key "$OPENAI_API_KEY" \
  --default

# Get the config ID for the next steps
APIKEY_ID=$(bin/creel-cli config apikey list | jq -r '.configs[0].id')

# Create a default embedding config
bin/creel-cli config embedding create \
  --name "openai-embed" \
  --provider openai \
  --model text-embedding-3-small \
  --dimensions 1536 \
  --apikey-config "$APIKEY_ID" \
  --default

# Create a default LLM config (used by memory extraction workers)
bin/creel-cli config llm create \
  --name "openai-llm" \
  --provider openai \
  --model gpt-4o \
  --apikey-config "$APIKEY_ID" \
  --default

# Verify
bin/creel-cli config embedding list
bin/creel-cli config llm list
```

All document processing and search from this point forward uses real OpenAI embeddings.

## 3. Create topics

We will create two topics to demonstrate cross-topic search later.

```bash
bin/creel-cli topic create fly-fishing "Fly Fishing Knowledge Base"
bin/creel-cli topic create ski-conditions "Ski Conditions Reports"
```

The CLI accepts slugs anywhere a topic ID is expected, so you can use `fly-fishing` and `ski-conditions` directly throughout this guide.

## 4. Upload documents (managed pipeline)

Create some sample documents and upload them. The server handles extraction, chunking, and embedding automatically.

```bash
cat > /tmp/hatch-chart.txt << 'EOF'
Western Maine Hatch Chart - 2026 Season

Early May: Hendrickson (Ephemerella subvaria). Size 12-14. Afternoon emergence.
Best patterns: Comparadun, Sparkle Dun, RS2 emerger.

Late May: March Brown (Stenonema vicarium). Size 10-12. Late afternoon.
Best patterns: Usual, March Brown wet fly.

Early June: Sulphur (Ephemerella dorothea). Size 16-18. Evening spinner falls.
Best patterns: Sulphur Comparadun, CDC Sulphur Emerger.

Mid June: Green Drake (Ephemera guttulata). Size 8-10. Late evening to dusk.
Best patterns: Extended body drake, Paradrake, Coffin Fly spinner.

July: Tricos (Tricorythodes). Size 22-26. Early morning spinner falls.
Best patterns: Spent wing trico, Trico spinner cluster.

August: Isonychia (Isonychia bicolor). Size 10-12. Evening.
Best patterns: Iso Dun, Leadwing Coachman.

All season: Caddis in various sizes. Elk Hair Caddis is always in the box.
EOF

bin/creel-cli upload \
  --topic fly-fishing \
  --file /tmp/hatch-chart.txt \
  --name "Western Maine Hatch Chart 2026" \
  --author "Rangeley Guides Association" \
  --content-type text/plain
```

Note the `job_id` in the response. Now upload an HTML document:

```bash
cat > /tmp/rangeley-report.html << 'EOF'
<html><body>
<h1>Rangeley Lake Fishing Report - March 2026</h1>
<p>Ice-out is projected for late April this year. Water temperatures are running
about 2 degrees below normal. The lake trout have been active in 30-40 feet of
water near the Oquossoc narrows.</p>
<p>Brook trout in the tributaries should start moving by mid-May once water temps
hit 50F. The Kennebago River traditionally fishes well starting Memorial Day
weekend with Hendricksons.</p>
<p>Landlocked salmon are stacking up below the Upper Dam. Expect good streamer
fishing once flows stabilize. Gray Ghosts and Black-Nosed Dace in sizes 4-6.</p>
</body></html>
EOF

bin/creel-cli upload \
  --topic fly-fishing \
  --file /tmp/rangeley-report.html \
  --name "Rangeley Lake Report March 2026" \
  --author "Maine IF&W" \
  --url "https://example.com/rangeley-report" \
  --content-type text/html
```

Upload a document to the ski topic:

```bash
cat > /tmp/ski-report.txt << 'EOF'
Saddleback Mountain Conditions - March 9, 2026

Base depth: 48 inches. New snow last 48h: 8 inches.
Surface: packed powder with fresh snow on top.

All 66 trails open. All 5 lifts running.
Rangeley chair has no wait. Kennebago quad is 5 minutes.

Conditions are excellent for early March. The recent storm dropped light, dry snow.
Expert terrain on Muleskinner and Tightline is in prime shape.

Grooming: all blue and green trails groomed overnight.
Black diamonds left ungroomed for bump skiing.

Season pass holders: spring skiing hours start March 15 (9am-4pm).
EOF

bin/creel-cli upload \
  --topic ski-conditions \
  --file /tmp/ski-report.txt \
  --name "Saddleback Conditions March 2026" \
  --author "Saddleback Mountain" \
  --content-type text/plain
```

## 5. Monitor processing jobs

Watch the pipeline process your documents:

```bash
# List all jobs for the fishing topic
bin/creel-cli jobs list --topic fly-fishing

# Check a specific job
bin/creel-cli jobs status <JOB_ID>
```

The pipeline runs extraction, then chunking, then embedding. Each stage creates a follow-on job. Wait until all jobs show `completed`:

```bash
bin/creel-cli jobs list --topic fly-fishing --status completed
bin/creel-cli jobs list --topic ski-conditions --status completed
```

Verify documents are ready:

```bash
bin/creel-cli document list --topic fly-fishing
```

You should see `"status": "ready"` for both fishing documents.

## 6. Search with citations

Now that documents are indexed with real embeddings, search returns semantically relevant results. You can search using natural language via `query_text`; the server computes the embedding using the default embedding config you registered in step 2.

```bash
# Search the fishing topic using the CLI
bin/creel-cli search --topic fly-fishing --query "best flies for evening fishing" --top-k 5
```

Or via REST with full citation details. The REST API requires UUIDs, so resolve the slugs first:

```bash
FISH_TOPIC=$(bin/creel-cli topic list | jq -r '.topics[] | select(.slug=="fly-fishing") | .id')
SKI_TOPIC=$(bin/creel-cli topic list | jq -r '.topics[] | select(.slug=="ski-conditions") | .id')

curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  -d "{\"topic_ids\": [\"$FISH_TOPIC\"], \"query_text\": \"best flies for evening fishing\", \"top_k\": 5}" \
  "http://localhost:8080/v1/search" | jq '.results[] | {
    content: .chunk.content[:80],
    score,
    citation: .documentCitation | {name, author, url}
  }'
```

Notice that each result includes `documentCitation` with the name, author, and URL you provided at upload time.

Search across both topics:

```bash
curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  -d "{\"topic_ids\": [\"$FISH_TOPIC\", \"$SKI_TOPIC\"], \"query_text\": \"conditions report\", \"top_k\": 5}" \
  "http://localhost:8080/v1/search" | jq '.results[] | {
    content: .chunk.content[:80],
    topicId,
    citation: .documentCitation.name
  }'
```

## 7. GetContext (temporal retrieval)

GetContext returns chunks from a single document in sequence order; this is the temporal layer.

```bash
# Get the first fishing document's ID
DOC_ID=$(curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  "http://localhost:8080/v1/topics/$FISH_TOPIC/documents" | jq -r '.documents[0].id')

# Retrieve all chunks in order
curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  "http://localhost:8080/v1/documents/$DOC_ID/context?last_n=10" | jq '.chunks[] | {
    sequence,
    content: .content[:100]
  }'
```

## 8. Direct chunk ingestion (power-user path)

Instead of uploading a file, you can ingest pre-chunked content directly. This is the "power user" path for agents that handle their own chunking.

```bash
# Create an empty document shell
DOC_DIRECT=$(curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  -d "{\"topic_id\": \"$FISH_TOPIC\", \"name\": \"Streamer Techniques\", \"author\": \"Lefty Kreh\"}" \
  "http://localhost:8080/v1/documents" | jq -r '.id')

echo "Direct document: $DOC_DIRECT"

# Ingest pre-chunked content (server computes embeddings)
curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  -d '{
    "document_id": "'"$DOC_DIRECT"'",
    "chunks": [
      {"content": "Strip-set, never trout-set, when fishing streamers. A downstream swing with a hard strip is the bread and butter retrieve.", "sequence": 1},
      {"content": "Articulated streamers like the Drunk and Disorderly trigger a lateral chase response. Use a sinking line to get them down in the water column.", "sequence": 2},
      {"content": "Streamer color selection: dark day, dark fly; bright day, bright fly. White and olive are reliable in stained water. Black is king on overcast days.", "sequence": 3}
    ]
  }' \
  "http://localhost:8080/v1/documents/$DOC_DIRECT/chunks" | jq '.chunks[] | {id, sequence, content: .content[:60]}'
```

Save the chunk IDs as a JSON array for the compaction step later:

```bash
CHUNK_IDS_JSON=$(curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  "http://localhost:8080/v1/documents/$DOC_DIRECT/context?last_n=10" | jq '[.chunks[].id]')
echo "Chunk IDs: $CHUNK_IDS_JSON"
```

## 9. Per-principal memory

Memory is scoped to your principal (the `dev` account in this case) and organized by named scopes.

```bash
# Add some memories
bin/creel-cli memory add --scope fishing --content "Prefers dry fly fishing over nymphing"
bin/creel-cli memory add --scope fishing --content "Home water is Rangeley Lake and Kennebago River"
bin/creel-cli memory add --scope fishing --content "Favorite fly is the Ausable Wulff in size 12"
bin/creel-cli memory add --scope skiing --content "Season pass holder at Saddleback Mountain"
bin/creel-cli memory add --scope skiing --content "Prefers bump skiing on ungroomed black diamonds"

# List memories in a scope
bin/creel-cli memory list --scope fishing

# List all scopes
bin/creel-cli memory scopes
```

Update a memory (simulating what the memory maintenance worker does):

```bash
# Get the memory ID for the Ausable Wulff memory
WULFF_ID=$(bin/creel-cli memory list --scope fishing | jq -r '.memories[] | select(.content | contains("Ausable Wulff")) | .id')

# Oops, it's actually a size 14
curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  -X PATCH \
  -d "{\"content\": \"Favorite fly is the Ausable Wulff in size 14\"}" \
  "http://localhost:8080/v1/memories/$WULFF_ID" | jq '{id, content, updatedAt}'
```

Delete (soft-invalidate) a memory:

```bash
bin/creel-cli memory delete "$WULFF_ID"

# Still visible with --all flag (shows invalidated status)
bin/creel-cli memory list --scope fishing --all
```

## 10. Compaction

Compaction replaces multiple chunks with a summary, preserving links.

```bash
# Compact all three streamer chunks into a summary
echo "{
  \"document_id\": \"$DOC_DIRECT\",
  \"chunk_ids\": $CHUNK_IDS_JSON,
  \"summary_content\": \"Streamer fishing essentials: always strip-set, use downstream swing retrieves. Articulated patterns trigger chase responses; fish them on sinking lines. Color rule: dark flies on dark days, bright flies on bright days.\"
}" | curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  -d @- \
  "http://localhost:8080/v1/compact" | jq '{
    summary_id: .summaryChunk.id,
    compacted_count: .compactedCount,
    summary: .summaryChunk.content[:120]
  }'
```

Save the summary chunk ID:

```bash
SUMMARY_ID=$(curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  "http://localhost:8080/v1/documents/$DOC_DIRECT/context?last_n=10" | jq -r '.chunks[0].id')
echo "Summary chunk: $SUMMARY_ID"
```

Verify the original chunks are now compacted and only the summary appears in context:

```bash
# GetContext only returns active chunks
curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  "http://localhost:8080/v1/documents/$DOC_DIRECT/context?last_n=10" | jq '.chunks | length'
# Should be 1 (just the summary)
```

Uncompact to restore the originals:

```bash
curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  -d "{\"summary_chunk_id\": \"$SUMMARY_ID\"}" \
  "http://localhost:8080/v1/uncompact" | jq '.restoredChunks | length'
# Should be 3

# Verify they are back
curl -s -H "Authorization: Bearer $CREEL_API_KEY" \
  "http://localhost:8080/v1/documents/$DOC_DIRECT/context?last_n=10" | jq '.chunks | length'
# Should be 3
```

## 11. Dashboard

The admin dashboard runs on port 3000. Open [http://localhost:3000](http://localhost:3000) and log in:

- **Username**: admin
- **Password**: admin

From the dashboard you can:

- Browse topics and their documents
- View system accounts and API keys
- Manage server configuration (LLM, embedding, prompt configs)

## 12. Interactive chat with creel-chat

creel-chat is a terminal REPL that uses Creel for RAG and memory, with streaming LLM responses. It calls OpenAI directly for both embeddings and chat completion, so you need `OPENAI_API_KEY` in your shell environment (separate from the server-side config you set in step 2).

```bash
export OPENAI_API_KEY="your-key-here"
```

### Session 1: RAG search and building memory

Start a chat session scoped to the fishing topic:

```bash
bin/creel-chat --topic fly-fishing --memory-scope fishing
```

Ask a question that the uploaded documents can answer:

```
you> What flies should I use for evening fishing in June?
```

The assistant's answer draws on the hatch chart and fishing report you uploaded in step 4. The RAG layer retrieves relevant chunks and the response references Sulphurs, Green Drakes, and Isonychia.

Now tell the assistant something about yourself:

```
you> I'm a fly fishing guide based in Rangeley, Maine. I mostly guide clients on the Kennebago River and Rangeley Lake. I need to keep up with hatch charts and conditions reports so I can plan trips.
```

The assistant acknowledges this. Behind the scenes, the memory extraction worker may pick up facts from the conversation (depending on your LLM config). You can also add memories explicitly:

```
you> /remember I prefer dry fly fishing over nymphing whenever possible
you> /remember My best day last season was 20 brook trout on Green Drakes in June on the Kennebago
```

Exit the session with Ctrl-D. Note the resume command printed on exit.

### Session 2: memory persists across sessions

Start a new session (do NOT use `--resume`; this is a fresh session that should recall memories from session 1):

```bash
bin/creel-chat --topic fly-fishing --memory-scope fishing
```

Ask a question that relies on what you told session 1:

```
you> What should I be preparing for this month on the river?
```

The assistant knows you are a guide on the Kennebago, that you prefer dry flies, and that you had a great Green Drake season. It combines RAG results (hatch chart, conditions report) with your stored memories to give a personalized answer.

Try something more specific:

```
you> Any tips for my clients who are beginners? What's the easiest hatch to fish right now?
```

The response should reference your guiding context and recommend appropriate hatches from the indexed documents.

Exit with Ctrl-D.

### Cross-topic search

Start a session with `--cross-topic` to search across all accessible topics:

```bash
bin/creel-chat --topic fly-fishing --memory-scope fishing --cross-topic
```

```
you> What are the conditions at Saddleback today?
```

Without `--cross-topic`, this returns nothing because you are scoped to the fly-fishing topic. With it, the RAG layer pulls chunks from the ski-conditions topic and the assistant can tell you about the 48-inch base and fresh powder.

Exit with Ctrl-D.

### Resuming a session

When creel-chat exits, it prints a resume command:

```
To resume this session:
  creel-chat --resume <document-id> --topic fly-fishing
```

Use it to continue the conversation with full session history intact. The assistant remembers everything from the original session verbatim.

## Cleanup

```bash
docker compose down -v   # removes containers and the pgdata volume
rm -f /tmp/hatch-chart.txt /tmp/rangeley-report.html /tmp/ski-report.txt
```

## What you exercised

| Feature | Steps |
|---------|-------|
| Provider configuration (API keys, embedding, LLM) | 2 |
| Managed document upload | 4, 5 |
| Processing pipeline (extraction, chunking, embedding) | 4, 5 |
| Search with citations (`query_text`) | 6 |
| Temporal context retrieval | 7 |
| Direct chunk ingestion | 8 |
| Per-principal memory (CRUD, scopes, soft-delete) | 9 |
| Compaction and uncompaction | 10 |
| Admin dashboard | 11 |
| Interactive chat with RAG + memory | 12 |
| Cross-topic search | 6, 12 |

## Next steps

- [Concepts](CONCEPTS.md): deep dive on the data model, auth, and retrieval modes
- [API Reference](API_REFERENCE.md): all 63 RPCs
- [Deployment](DEPLOYMENT.md): production Helm chart
- [Architecture](ARCHITECTURE.md): design document and roadmap
