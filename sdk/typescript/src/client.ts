import { CreelError } from "./errors";
import type {
  AddMemoryRequest,
  Chunk,
  CompactRequest,
  CompactResponse,
  CompactionHistoryEntry,
  CreateDocumentRequest,
  CreateLinkRequest,
  CreateTopicRequest,
  Document,
  GetCompactionHistoryResponse,
  GetContextRequest,
  GetContextResponse,
  GetMemoryResponse,
  Grant,
  GrantAccessRequest,
  HealthResponse,
  IngestChunk,
  IngestChunksResponse,
  Job,
  Link,
  ListDocumentsResponse,
  ListGrantsResponse,
  ListJobsParams,
  ListJobsResponse,
  ListLinksResponse,
  ListMemoriesResponse,
  ListScopesResponse,
  ListTopicsResponse,
  Memory,
  PageParams,
  RequestCompactionRequest,
  RequestCompactionResponse,
  SearchMemoriesRequest,
  SearchMemoriesResponse,
  SearchRequest,
  SearchResponse,
  Topic,
  UncompactResponse,
  UpdateDocumentRequest,
  UpdateMemoryRequest,
  UpdateTopicRequest,
  UploadDocumentRequest,
  UploadDocumentResponse,
} from "./types";

export interface CreelClientOptions {
  baseUrl: string;
  apiKey: string;
}

/**
 * REST client for the Creel memory-as-a-service platform.
 * Wraps the gRPC-Gateway HTTP/JSON endpoints.
 */
export class CreelClient {
  private readonly baseUrl: string;
  private readonly apiKey: string;

  constructor(options: CreelClientOptions) {
    // Strip trailing slash so callers don't have to worry about it.
    this.baseUrl = options.baseUrl.replace(/\/+$/, "");
    this.apiKey = options.apiKey;
  }

  // -------------------------------------------------------------------------
  // Internal helpers
  // -------------------------------------------------------------------------

  private headers(): Record<string, string> {
    return {
      Authorization: `Bearer ${this.apiKey}`,
      "Content-Type": "application/json",
    };
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    const init: RequestInit = {
      method,
      headers: this.headers(),
    };

    if (body !== undefined) {
      init.body = JSON.stringify(body);
    }

    const res = await fetch(url, init);

    if (!res.ok) {
      let errBody: unknown;
      try {
        errBody = await res.json();
      } catch {
        errBody = await res.text().catch(() => null);
      }
      const msg =
        typeof errBody === "object" &&
        errBody !== null &&
        "message" in errBody
          ? String((errBody as Record<string, unknown>).message)
          : `HTTP ${res.status}`;
      throw new CreelError(res.status, msg, errBody);
    }

    // 204 No Content
    if (res.status === 204) {
      return undefined as T;
    }

    return (await res.json()) as T;
  }

  private queryString(params: object): string {
    const parts: string[] = [];
    for (const [key, value] of Object.entries(params as Record<string, unknown>)) {
      if (value !== undefined && value !== null) {
        parts.push(
          `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`,
        );
      }
    }
    return parts.length > 0 ? `?${parts.join("&")}` : "";
  }

  // -------------------------------------------------------------------------
  // TopicService
  // -------------------------------------------------------------------------

  async createTopic(req: CreateTopicRequest): Promise<Topic> {
    return this.request<Topic>("POST", "/v1/topics", req);
  }

  async getTopic(id: string): Promise<Topic> {
    return this.request<Topic>("GET", `/v1/topics/${encodeURIComponent(id)}`);
  }

  async listTopics(params?: PageParams): Promise<ListTopicsResponse> {
    const qs = this.queryString(params ?? {});
    return this.request<ListTopicsResponse>("GET", `/v1/topics${qs}`);
  }

  async updateTopic(id: string, req: UpdateTopicRequest): Promise<Topic> {
    return this.request<Topic>(
      "PATCH",
      `/v1/topics/${encodeURIComponent(id)}`,
      req,
    );
  }

  async deleteTopic(id: string): Promise<void> {
    return this.request<void>(
      "DELETE",
      `/v1/topics/${encodeURIComponent(id)}`,
    );
  }

  async grantAccess(topicId: string, req: GrantAccessRequest): Promise<Grant> {
    return this.request<Grant>(
      "POST",
      `/v1/topics/${encodeURIComponent(topicId)}/grants`,
      req,
    );
  }

  async revokeAccess(topicId: string, principal: string): Promise<void> {
    return this.request<void>(
      "DELETE",
      `/v1/topics/${encodeURIComponent(topicId)}/grants/${encodeURIComponent(principal)}`,
    );
  }

  async listGrants(topicId: string): Promise<ListGrantsResponse> {
    return this.request<ListGrantsResponse>(
      "GET",
      `/v1/topics/${encodeURIComponent(topicId)}/grants`,
    );
  }

  // -------------------------------------------------------------------------
  // DocumentService
  // -------------------------------------------------------------------------

  async createDocument(req: CreateDocumentRequest): Promise<Document> {
    return this.request<Document>("POST", "/v1/documents", req);
  }

  async getDocument(id: string): Promise<Document> {
    return this.request<Document>(
      "GET",
      `/v1/documents/${encodeURIComponent(id)}`,
    );
  }

  async listDocuments(
    topicId: string,
    params?: PageParams,
  ): Promise<ListDocumentsResponse> {
    const qs = this.queryString(params ?? {});
    return this.request<ListDocumentsResponse>(
      "GET",
      `/v1/topics/${encodeURIComponent(topicId)}/documents${qs}`,
    );
  }

  async updateDocument(
    id: string,
    req: UpdateDocumentRequest,
  ): Promise<Document> {
    return this.request<Document>(
      "PATCH",
      `/v1/documents/${encodeURIComponent(id)}`,
      req,
    );
  }

  async deleteDocument(id: string): Promise<void> {
    return this.request<void>(
      "DELETE",
      `/v1/documents/${encodeURIComponent(id)}`,
    );
  }

  async uploadDocument(req: UploadDocumentRequest): Promise<UploadDocumentResponse> {
    // Encode file bytes as base64 for JSON transport.
    const fileBase64 = base64Encode(req.file);
    const body = {
      topic_id: req.topic_id,
      name: req.name,
      file: fileBase64,
      slug: req.slug,
      source_url: req.source_url,
      content_type: req.content_type,
      metadata: req.metadata,
    };
    return this.request<UploadDocumentResponse>(
      "POST",
      "/v1/documents:upload",
      body,
    );
  }

  // -------------------------------------------------------------------------
  // ChunkService
  // -------------------------------------------------------------------------

  async ingestChunks(
    documentId: string,
    chunks: IngestChunk[],
  ): Promise<IngestChunksResponse> {
    return this.request<IngestChunksResponse>(
      "POST",
      `/v1/documents/${encodeURIComponent(documentId)}/chunks`,
      { chunks },
    );
  }

  async getChunk(id: string): Promise<Chunk> {
    return this.request<Chunk>("GET", `/v1/chunks/${encodeURIComponent(id)}`);
  }

  async deleteChunk(id: string): Promise<void> {
    return this.request<void>(
      "DELETE",
      `/v1/chunks/${encodeURIComponent(id)}`,
    );
  }

  // -------------------------------------------------------------------------
  // RetrievalService
  // -------------------------------------------------------------------------

  async search(req: SearchRequest): Promise<SearchResponse> {
    return this.request<SearchResponse>("POST", "/v1/search", req);
  }

  async getContext(
    documentId: string,
    params?: GetContextRequest,
  ): Promise<GetContextResponse> {
    const qs = this.queryString(params ?? {});
    return this.request<GetContextResponse>(
      "GET",
      `/v1/documents/${encodeURIComponent(documentId)}/context${qs}`,
    );
  }

  // -------------------------------------------------------------------------
  // MemoryService
  // -------------------------------------------------------------------------

  async getMemory(scope: string): Promise<GetMemoryResponse> {
    return this.request<GetMemoryResponse>(
      "GET",
      `/v1/memories/${encodeURIComponent(scope)}`,
    );
  }

  async searchMemories(
    req: SearchMemoriesRequest,
  ): Promise<SearchMemoriesResponse> {
    return this.request<SearchMemoriesResponse>(
      "POST",
      "/v1/memories:search",
      req,
    );
  }

  async addMemory(req: AddMemoryRequest): Promise<Memory> {
    return this.request<Memory>("POST", "/v1/memories", req);
  }

  async updateMemory(id: string, req: UpdateMemoryRequest): Promise<Memory> {
    return this.request<Memory>(
      "PATCH",
      `/v1/memories/${encodeURIComponent(id)}`,
      req,
    );
  }

  async deleteMemory(id: string): Promise<void> {
    return this.request<void>(
      "DELETE",
      `/v1/memories/${encodeURIComponent(id)}`,
    );
  }

  async listMemories(
    scope: string,
    includeInvalidated?: boolean,
  ): Promise<ListMemoriesResponse> {
    const qs = this.queryString({
      include_invalidated: includeInvalidated,
    });
    return this.request<ListMemoriesResponse>(
      "GET",
      `/v1/memories/${encodeURIComponent(scope)}/list${qs}`,
    );
  }

  async listScopes(): Promise<ListScopesResponse> {
    return this.request<ListScopesResponse>("GET", "/v1/memories:scopes");
  }

  // -------------------------------------------------------------------------
  // LinkService
  // -------------------------------------------------------------------------

  async createLink(req: CreateLinkRequest): Promise<Link> {
    return this.request<Link>("POST", "/v1/links", req);
  }

  async deleteLink(id: string): Promise<void> {
    return this.request<void>(
      "DELETE",
      `/v1/links/${encodeURIComponent(id)}`,
    );
  }

  async listLinks(
    chunkId: string,
    includeBacklinks?: boolean,
  ): Promise<ListLinksResponse> {
    const qs = this.queryString({ include_backlinks: includeBacklinks });
    return this.request<ListLinksResponse>(
      "GET",
      `/v1/chunks/${encodeURIComponent(chunkId)}/links${qs}`,
    );
  }

  // -------------------------------------------------------------------------
  // CompactionService
  // -------------------------------------------------------------------------

  async compact(req: CompactRequest): Promise<CompactResponse> {
    return this.request<CompactResponse>("POST", "/v1/compact", req);
  }

  async uncompact(summaryChunkId: string): Promise<UncompactResponse> {
    return this.request<UncompactResponse>("POST", "/v1/uncompact", {
      summary_chunk_id: summaryChunkId,
    });
  }

  async requestCompaction(
    req: RequestCompactionRequest,
  ): Promise<RequestCompactionResponse> {
    return this.request<RequestCompactionResponse>(
      "POST",
      "/v1/compact/request",
      req,
    );
  }

  async getCompactionHistory(
    documentId: string,
  ): Promise<GetCompactionHistoryResponse> {
    return this.request<GetCompactionHistoryResponse>(
      "GET",
      `/v1/documents/${encodeURIComponent(documentId)}/compaction-history`,
    );
  }

  // -------------------------------------------------------------------------
  // JobService
  // -------------------------------------------------------------------------

  async getJob(id: string): Promise<Job> {
    return this.request<Job>("GET", `/v1/jobs/${encodeURIComponent(id)}`);
  }

  async listJobs(params?: ListJobsParams): Promise<ListJobsResponse> {
    const qs = this.queryString(params ?? {});
    return this.request<ListJobsResponse>("GET", `/v1/jobs${qs}`);
  }

  // -------------------------------------------------------------------------
  // AdminService
  // -------------------------------------------------------------------------

  async health(): Promise<HealthResponse> {
    return this.request<HealthResponse>("GET", "/v1/health");
  }
}

// ---------------------------------------------------------------------------
// Base64 helper (works in Node.js, Deno, Bun, and modern browsers)
// ---------------------------------------------------------------------------

function base64Encode(bytes: Uint8Array): string {
  if (typeof Buffer !== "undefined") {
    return Buffer.from(bytes).toString("base64");
  }
  // Browser fallback
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}
