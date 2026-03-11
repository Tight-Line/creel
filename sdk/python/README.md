# Creel Python SDK

A Python client for the [Creel](https://github.com/tightlinesoftware/creel) memory-as-a-service platform. Wraps the gRPC-Gateway REST endpoints with a clean, typed interface.

## Installation

```bash
pip install creel-sdk
```

Or install from source:

```bash
pip install -e sdk/python/
```

## Quick Start

```python
from creel import CreelClient

client = CreelClient(base_url="http://localhost:8080", api_key="your-api-key")

# Check server health
health = client.health()
print(health.status, health.version)

# Create a topic
topic = client.create_topic("my-notes", "My Notes", "Personal knowledge base")
print(topic.id, topic.slug)
```

## Usage

### Topics

```python
# List all topics
resp = client.list_topics(page_size=10)
for t in resp.topics:
    print(t.name)

# Update a topic
updated = client.update_topic(topic.id, description="Updated description")

# Delete a topic
client.delete_topic(topic.id)
```

### Access Grants

```python
grant = client.grant_access(topic.id, "user:alice", "PERMISSION_WRITE")

grants = client.list_grants(topic.id)
for g in grants.grants:
    print(g.principal, g.permission)

client.revoke_access(topic.id, "user:alice")
```

### Documents

```python
doc = client.create_document(
    topic.id,
    "Meeting Notes Q1",
    slug="meeting-notes-q1",
    doc_type="notes",
    metadata={"quarter": "Q1"},
)

# List documents in a topic
resp = client.list_documents(topic.id)
for d in resp.documents:
    print(d.name, d.status)

# Upload a file (bytes are base64-encoded automatically)
with open("report.pdf", "rb") as f:
    upload_resp = client.upload_document(
        topic.id,
        "Q1 Report",
        f.read(),
        content_type="application/pdf",
    )
print(upload_resp.job_id)  # track processing via JobService
```

### Chunks

```python
resp = client.ingest_chunks(
    doc.id,
    chunks=[
        {"content": "First paragraph of the document.", "sequence": 1},
        {"content": "Second paragraph with details.", "sequence": 2},
    ],
)
for chunk in resp.chunks:
    print(chunk.id, chunk.sequence)

chunk = client.get_chunk(resp.chunks[0].id)
client.delete_chunk(chunk.id)
```

### Search

```python
results = client.search(
    [topic.id],
    query_text="quarterly revenue",
    top_k=5,
    follow_links=True,
    link_depth=2,
)
for r in results.results:
    print(f"[{r.score:.3f}] {r.chunk.content[:80]}")
    if r.document_citation:
        print(f"  Source: {r.document_citation.name}")
```

### Context Retrieval

```python
ctx = client.get_context(doc.id, last_n=20, include_summaries=True)
for chunk in ctx.chunks:
    print(chunk.content)
```

### Memory

```python
# Add a memory (returns a job_id; processing is async)
resp = client.add_memory(
    "project-alpha",
    "The deployment deadline is March 15th.",
    subject="deployment",
    predicate="has_deadline",
    object="2026-03-15",
)
print(resp.job_id)  # poll via client.get_job(resp.job_id)

# Retrieve all memories in a scope
memories = client.get_memory("project-alpha")
for m in memories:
    print(m.content)

# Search memories
resp = client.search_memories(scope="project-alpha", query_text="deadline", top_k=5)
for r in resp.results:
    print(f"[{r.score:.3f}] {r.memory.content}")

# List all scopes
scopes = client.list_scopes()
print(scopes.scopes)

# List memories with invalidated ones included
resp = client.list_memories("project-alpha", include_invalidated=True)

# Update and delete
client.update_memory(mem.id, content="Deadline moved to March 20th.")
client.delete_memory(mem.id)
```

### Links

```python
link = client.create_link(
    source_chunk_id=chunk_a.id,
    target_chunk_id=chunk_b.id,
    link_type="LINK_TYPE_MANUAL",
)

links = client.list_links(chunk_a.id, include_backlinks=True)
for lk in links.links:
    print(lk.source_chunk_id, "->", lk.target_chunk_id)

client.delete_link(link.id)
```

### Compaction

```python
# Compact chunks into a summary
resp = client.compact(
    doc.id,
    chunk_ids=[c.id for c in old_chunks],
    summary_content="Summary of the compacted chunks.",
)
print(resp.summary_chunk.id, resp.compacted_count)

# Undo compaction
undo = client.uncompact(resp.summary_chunk.id)
print(f"Restored {len(undo.restored_chunks)} chunks")

# Request async compaction (returns a job ID)
job_resp = client.request_compaction(doc.id)
print(job_resp.job_id)

# View compaction history
history = client.get_compaction_history(doc.id)
for rec in history.records:
    print(rec.summary_chunk_id, rec.source_chunk_ids)
```

### Jobs

```python
job = client.get_job(job_id)
print(job.status, job.job_type)

resp = client.list_jobs(topic_id=topic.id, status="running", page_size=20)
for j in resp.jobs:
    print(j.id, j.status)
```

### Error Handling

```python
from creel import CreelClient, CreelError

client = CreelClient(base_url="http://localhost:8080", api_key="bad-key")
try:
    client.health()
except CreelError as e:
    print(e.status_code)  # e.g. 401
    print(e.message)      # e.g. "unauthorized"
```

### Context Manager

The client can be used as a context manager to ensure the underlying HTTP connection is closed:

```python
with CreelClient(base_url="http://localhost:8080", api_key="key") as client:
    health = client.health()
```

## Requirements

- Python 3.10+
- httpx >= 0.25.0
