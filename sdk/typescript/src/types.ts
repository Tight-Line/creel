// ---------------------------------------------------------------------------
// Common
// ---------------------------------------------------------------------------

export interface PageParams {
  page_size?: number;
  page_token?: string;
}

// ---------------------------------------------------------------------------
// Topics
// ---------------------------------------------------------------------------

export interface Topic {
  id: string;
  slug: string;
  name: string;
  description: string;
  memory_enabled: boolean;
  embedding_config_id?: string;
  vector_backend_config_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateTopicRequest {
  slug: string;
  name: string;
  description?: string;
  memory_enabled?: boolean;
}

export interface UpdateTopicRequest {
  name?: string;
  description?: string;
  memory_enabled?: boolean;
}

export interface ListTopicsResponse {
  topics: Topic[];
  next_page_token: string;
}

// ---------------------------------------------------------------------------
// Grants
// ---------------------------------------------------------------------------

export interface Grant {
  topic_id: string;
  principal: string;
  permission: string;
  created_at: string;
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
  topic_id: string;
  slug: string;
  name: string;
  doc_type: string;
  metadata: Record<string, string>;
  url: string;
  author: string;
  created_at: string;
  updated_at: string;
}

export interface CreateDocumentRequest {
  topic_id: string;
  name: string;
  slug?: string;
  doc_type?: string;
  metadata?: Record<string, string>;
  url?: string;
  author?: string;
}

export interface UpdateDocumentRequest {
  name?: string;
  doc_type?: string;
  metadata?: Record<string, string>;
}

export interface ListDocumentsResponse {
  documents: Document[];
  next_page_token: string;
}

export interface UploadDocumentRequest {
  topic_id: string;
  name: string;
  file: Uint8Array;
  slug?: string;
  source_url?: string;
  content_type?: string;
  metadata?: Record<string, string>;
}

export interface UploadDocumentResponse {
  document: Document;
  chunk_count: number;
}

// ---------------------------------------------------------------------------
// Chunks
// ---------------------------------------------------------------------------

export interface Chunk {
  id: string;
  document_id: string;
  content: string;
  index: number;
  metadata: Record<string, string>;
  embedding: number[];
  compacted_into?: string;
  created_at: string;
  updated_at: string;
}

export interface IngestChunk {
  content: string;
  index?: number;
  metadata?: Record<string, string>;
  embedding?: number[];
}

export interface IngestChunksResponse {
  chunk_ids: string[];
}

// ---------------------------------------------------------------------------
// Retrieval
// ---------------------------------------------------------------------------

export interface SearchRequest {
  topic_ids: string[];
  query_text?: string;
  query_embedding?: number[];
  top_k?: number;
  follow_links?: boolean;
  link_depth?: number;
  metadata_filter?: Record<string, string>;
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
  last_n?: number;
  since?: string;
  include_summaries?: boolean;
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
  invalidated_at?: string;
  created_at: string;
  updated_at: string;
}

export interface GetMemoryResponse {
  memories: Memory[];
}

export interface SearchMemoriesRequest {
  scope?: string;
  query_text?: string;
  top_k?: number;
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
  source_chunk_id: string;
  target_chunk_id: string;
  link_type: string;
  metadata: Record<string, string>;
  created_at: string;
}

export interface CreateLinkRequest {
  source_chunk_id: string;
  target_chunk_id: string;
  link_type?: string;
  metadata?: Record<string, string>;
}

export interface ListLinksResponse {
  links: Link[];
}

// ---------------------------------------------------------------------------
// Compaction
// ---------------------------------------------------------------------------

export interface CompactRequest {
  document_id: string;
  chunk_ids: string[];
  summary_content: string;
  summary_embedding?: number[];
  summary_metadata?: Record<string, string>;
}

export interface CompactResponse {
  summary_chunk_id: string;
}

export interface UncompactResponse {
  restored_chunk_ids: string[];
}

export interface RequestCompactionRequest {
  document_id: string;
  chunk_ids?: string[];
}

export interface RequestCompactionResponse {
  job_id: string;
}

export interface CompactionHistoryEntry {
  summary_chunk_id: string;
  original_chunk_ids: string[];
  created_at: string;
}

export interface GetCompactionHistoryResponse {
  entries: CompactionHistoryEntry[];
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

export interface Job {
  id: string;
  topic_id: string;
  document_id: string;
  job_type: string;
  status: string;
  progress: number;
  error: string;
  created_at: string;
  updated_at: string;
}

export interface ListJobsParams extends PageParams {
  topic_id?: string;
  document_id?: string;
  status?: string;
}

export interface ListJobsResponse {
  jobs: Job[];
  next_page_token: string;
}

// ---------------------------------------------------------------------------
// Admin
// ---------------------------------------------------------------------------

export interface HealthResponse {
  status: string;
}
