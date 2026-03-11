export { CreelClient } from "./client";
export type { CreelClientOptions } from "./client";
export { CreelError } from "./errors";
export type {
  // Common
  PageParams,

  // Topics
  Topic,
  CreateTopicRequest,
  UpdateTopicRequest,
  ListTopicsResponse,

  // Grants
  Grant,
  GrantAccessRequest,
  ListGrantsResponse,

  // Documents
  Document,
  CreateDocumentRequest,
  UpdateDocumentRequest,
  ListDocumentsResponse,
  UploadDocumentRequest,
  UploadDocumentResponse,

  // Chunks
  Chunk,
  IngestChunk,
  IngestChunksResponse,

  // Retrieval
  SearchRequest,
  SearchResult,
  SearchResponse,
  GetContextRequest,
  GetContextResponse,

  // Memory
  Memory,
  GetMemoriesRequest,
  GetMemoriesResponse,
  AddMessagesRequest,
  AddMessagesResponse,
  AddMemoryRequest,
  AddMemoryResponse,
  UpdateMemoryRequest,
  ListMemoriesResponse,
  ListScopesResponse,

  // Links
  Link,
  CreateLinkRequest,
  ListLinksResponse,

  // Compaction
  CompactRequest,
  CompactResponse,
  UncompactResponse,
  RequestCompactionRequest,
  RequestCompactionResponse,
  CompactionHistoryEntry,
  GetCompactionHistoryResponse,

  // Jobs
  Job,
  ListJobsParams,
  ListJobsResponse,

  // Admin
  HealthResponse,
} from "./types";
