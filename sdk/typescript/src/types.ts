// ---------------------------------------------------------------------------
// Common
// ---------------------------------------------------------------------------

export interface PageParams {
  pageSize?: number;
  pageToken?: string;
}

// ---------------------------------------------------------------------------
// Topics
// ---------------------------------------------------------------------------

export interface Topic {
  id: string;
  slug: string;
  name: string;
  description: string;
  memoryEnabled: boolean;
  embeddingConfigId?: string;
  vectorBackendConfigId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateTopicRequest {
  slug: string;
  name: string;
  description?: string;
  memoryEnabled?: boolean;
}

export interface UpdateTopicRequest {
  name?: string;
  description?: string;
  memoryEnabled?: boolean;
}

export interface ListTopicsResponse {
  topics: Topic[];
  nextPageToken: string;
}

// ---------------------------------------------------------------------------
// Grants
// ---------------------------------------------------------------------------

export interface Grant {
  topicId: string;
  principal: string;
  permission: string;
  createdAt: string;
}

export interface GrantAccessRequest {
  principal: string;
  permission: string;
}

export interface ListGrantsResponse {
  grants: Grant[];
}

// ---------------------------------------------------------------------------
// Documents
// ---------------------------------------------------------------------------

export interface Document {
  id: string;
  topicId: string;
  slug: string;
  name: string;
  docType: string;
  metadata: Record<string, string>;
  url: string;
  author: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateDocumentRequest {
  topicId: string;
  name: string;
  slug?: string;
  docType?: string;
  metadata?: Record<string, string>;
  url?: string;
  author?: string;
}

export interface UpdateDocumentRequest {
  name?: string;
  docType?: string;
  metadata?: Record<string, string>;
}

export interface ListDocumentsResponse {
  documents: Document[];
  nextPageToken: string;
}

export interface UploadDocumentRequest {
  topicId: string;
  name: string;
  file: Uint8Array;
  slug?: string;
  sourceUrl?: string;
  contentType?: string;
  metadata?: Record<string, string>;
}

export interface UploadDocumentResponse {
  document: Document;
  chunkCount: number;
}

// ---------------------------------------------------------------------------
// Chunks
// ---------------------------------------------------------------------------

export interface Chunk {
  id: string;
  documentId: string;
  content: string;
  index: number;
  metadata: Record<string, string>;
  embedding: number[];
  compactedInto?: string;
  createdAt: string;
  updatedAt: string;
}

export interface IngestChunk {
  content: string;
  index?: number;
  metadata?: Record<string, string>;
  embedding?: number[];
}

export interface IngestChunksResponse {
  chunkIds: string[];
}

// ---------------------------------------------------------------------------
// Retrieval
// ---------------------------------------------------------------------------

export interface SearchRequest {
  topicIds: string[];
  queryText?: string;
  queryEmbedding?: number[];
  topK?: number;
  followLinks?: boolean;
  linkDepth?: number;
  metadataFilter?: Record<string, string>;
}

export interface SearchResult {
  chunk: Chunk;
  score: number;
  document?: Document;
  topic?: Topic;
}

export interface SearchResponse {
  results: SearchResult[];
}

export interface GetContextRequest {
  lastN?: number;
  since?: string;
  includeSummaries?: boolean;
}

export interface GetContextResponse {
  chunks: Chunk[];
}

// ---------------------------------------------------------------------------
// Memory
// ---------------------------------------------------------------------------

export interface Memory {
  id: string;
  scope: string;
  content: string;
  subject: string;
  predicate: string;
  object: string;
  metadata: Record<string, string>;
  invalidatedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface GetMemoryResponse {
  memories: Memory[];
}

export interface SearchMemoriesRequest {
  scope?: string;
  queryText?: string;
  topK?: number;
}

export interface SearchMemoriesResponse {
  memories: Memory[];
  scores: number[];
}

export interface AddMemoryRequest {
  scope: string;
  content: string;
  subject?: string;
  predicate?: string;
  object?: string;
  metadata?: Record<string, string>;
}

export interface UpdateMemoryRequest {
  content?: string;
  metadata?: Record<string, string>;
}

export interface ListMemoriesResponse {
  memories: Memory[];
}

export interface ListScopesResponse {
  scopes: string[];
}

// ---------------------------------------------------------------------------
// Links
// ---------------------------------------------------------------------------

export interface Link {
  id: string;
  sourceChunkId: string;
  targetChunkId: string;
  linkType: string;
  metadata: Record<string, string>;
  createdAt: string;
}

export interface CreateLinkRequest {
  sourceChunkId: string;
  targetChunkId: string;
  linkType?: string;
  metadata?: Record<string, string>;
}

export interface ListLinksResponse {
  links: Link[];
}

// ---------------------------------------------------------------------------
// Compaction
// ---------------------------------------------------------------------------

export interface CompactRequest {
  documentId: string;
  chunkIds: string[];
  summaryContent: string;
  summaryEmbedding?: number[];
  summaryMetadata?: Record<string, string>;
}

export interface CompactResponse {
  summaryChunkId: string;
}

export interface UncompactResponse {
  restoredChunkIds: string[];
}

export interface RequestCompactionRequest {
  documentId: string;
  chunkIds?: string[];
}

export interface RequestCompactionResponse {
  jobId: string;
}

export interface CompactionHistoryEntry {
  summaryChunkId: string;
  originalChunkIds: string[];
  createdAt: string;
}

export interface GetCompactionHistoryResponse {
  entries: CompactionHistoryEntry[];
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

export interface Job {
  id: string;
  topicId: string;
  documentId: string;
  jobType: string;
  status: string;
  progress: number;
  error: string;
  createdAt: string;
  updatedAt: string;
}

export interface ListJobsParams extends PageParams {
  topicId?: string;
  documentId?: string;
  status?: string;
}

export interface ListJobsResponse {
  jobs: Job[];
  nextPageToken: string;
}

// ---------------------------------------------------------------------------
// Admin
// ---------------------------------------------------------------------------

export interface HealthResponse {
  status: string;
}
