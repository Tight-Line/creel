# @creel/sdk

TypeScript SDK for the [Creel](https://github.com/your-org/creel) memory-as-a-service platform. This is a lightweight REST client that wraps the gRPC-Gateway HTTP/JSON endpoints. It uses native `fetch` with no external HTTP dependencies.

## Requirements

- Node.js >= 18 (or any runtime with global `fetch`: Deno, Bun, modern browsers)
- TypeScript >= 5.4 (for development)

## Installation

```bash
npm install @creel/sdk
```

## Quick Start

```typescript
import { CreelClient } from "@creel/sdk";

const creel = new CreelClient({
  baseUrl: "http://localhost:8080",
  apiKey: "your-api-key",
});

// Create a topic
const topic = await creel.createTopic({
  slug: "project-notes",
  name: "Project Notes",
  description: "Notes and context for the project",
});

// Create a document
const doc = await creel.createDocument({
  topicId: topic.id,
  name: "meeting-2026-03-10.md",
  docType: "markdown",
});

// Ingest chunks
await creel.ingestChunks(doc.id, [
  { content: "We decided to use pgvector for embeddings.", index: 0 },
  { content: "The launch date is April 15.", index: 1 },
]);

// Search
const results = await creel.search({
  topicIds: [topic.id],
  queryText: "when is the launch?",
  topK: 5,
});

for (const r of results.results) {
  console.log(`[${r.score.toFixed(3)}] ${r.chunk.content}`);
}
```

## API Reference

### Constructor

```typescript
const creel = new CreelClient({
  baseUrl: "http://localhost:8080",
  apiKey: "your-api-key",
});
```

### TopicService

```typescript
await creel.createTopic({ slug, name, description?, memoryEnabled? })
await creel.getTopic(id)
await creel.listTopics({ pageSize?, pageToken? })
await creel.updateTopic(id, { name?, description?, memoryEnabled? })
await creel.deleteTopic(id)
await creel.grantAccess(topicId, { principal, permission })
await creel.revokeAccess(topicId, principal)
await creel.listGrants(topicId)
```

### DocumentService

```typescript
await creel.createDocument({ topicId, name, slug?, docType?, metadata?, url?, author? })
await creel.getDocument(id)
await creel.listDocuments(topicId, { pageSize?, pageToken? })
await creel.updateDocument(id, { name?, docType?, metadata? })
await creel.deleteDocument(id)
await creel.uploadDocument({ topicId, name, file: Uint8Array, slug?, sourceUrl?, contentType?, metadata? })
```

### ChunkService

```typescript
await creel.ingestChunks(documentId, [{ content, index?, metadata?, embedding? }])
await creel.getChunk(id)
await creel.deleteChunk(id)
```

### RetrievalService

```typescript
await creel.search({ topicIds, queryText?, queryEmbedding?, topK?, followLinks?, linkDepth?, metadataFilter? })
await creel.getContext(documentId, { lastN?, since?, includeSummaries? })
```

### MemoryService

```typescript
await creel.getMemory(scope)
await creel.searchMemories({ scope?, queryText?, topK? })
await creel.addMemory({ scope, content, subject?, predicate?, object?, metadata? })  // returns { job_id }
await creel.updateMemory(id, { content?, metadata? })
await creel.deleteMemory(id)
await creel.listMemories(scope, includeInvalidated?)
await creel.listScopes()
```

### LinkService

```typescript
await creel.createLink({ sourceChunkId, targetChunkId, linkType?, metadata? })
await creel.deleteLink(id)
await creel.listLinks(chunkId, includeBacklinks?)
```

### CompactionService

```typescript
await creel.compact({ documentId, chunkIds, summaryContent, summaryEmbedding?, summaryMetadata? })
await creel.uncompact(summaryChunkId)
await creel.requestCompaction({ documentId, chunkIds? })
await creel.getCompactionHistory(documentId)
```

### JobService

```typescript
await creel.getJob(id)
await creel.listJobs({ topicId?, documentId?, status?, pageSize?, pageToken? })
```

### AdminService

```typescript
await creel.health()
```

## Error Handling

All methods throw `CreelError` on non-2xx responses.

```typescript
import { CreelError } from "@creel/sdk";

try {
  await creel.getTopic("nonexistent");
} catch (err) {
  if (err instanceof CreelError) {
    console.error(`Status ${err.statusCode}: ${err.message}`);
    console.error("Response body:", err.body);
  }
}
```

## Pagination

List methods accept `pageSize` and `pageToken` parameters and return a `nextPageToken` in the response.

```typescript
let pageToken: string | undefined;

do {
  const page = await creel.listTopics({ pageSize: 10, pageToken });
  for (const topic of page.topics) {
    console.log(topic.name);
  }
  pageToken = page.nextPageToken || undefined;
} while (pageToken);
```

## File Upload

The `uploadDocument` method accepts a `Uint8Array` for the file content. It base64-encodes the bytes for JSON transport.

```typescript
import { readFile } from "node:fs/promises";

const fileBytes = new Uint8Array(await readFile("report.pdf"));

const result = await creel.uploadDocument({
  topicId: topic.id,
  name: "report.pdf",
  file: fileBytes,
  contentType: "application/pdf",
});

console.log(`Created document ${result.document.id} with ${result.chunkCount} chunks`);
```
