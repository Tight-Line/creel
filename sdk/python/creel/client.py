"""Creel Python SDK; a REST client wrapping the gRPC-Gateway HTTP endpoints."""

from __future__ import annotations

import base64
from typing import Any, Optional

import httpx

from creel.exceptions import CreelError
from creel.models import (
    AddMemoryResponse,
    AddMessagesResponse,
    Chunk,
    ChunkingStrategy,
    CompactResponse,
    CompactionRecord,
    Document,
    DocumentCitation,
    GetCompactionHistoryResponse,
    GetContextResponse,
    HealthResponse,
    IngestChunksResponse,
    Link,
    ListDocumentsResponse,
    ListGrantsResponse,
    ListJobsResponse,
    ListLinksResponse,
    ListMemoriesResponse,
    ListScopesResponse,
    ListTopicsResponse,
    Memory,
    ProcessingJob,
    RequestCompactionResponse,
    SearchResponse,
    SearchResult,
    Topic,
    TopicGrant,
    UncompactResponse,
    UploadDocumentResponse,
)


class CreelClient:
    """Synchronous REST client for the Creel memory-as-a-service API.

    Args:
        base_url: Base URL of the Creel server (e.g. ``http://localhost:8080``).
        api_key: API key used in the ``Authorization: Bearer`` header.
        timeout: Request timeout in seconds. Defaults to 30.
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        api_key: str = "",
        timeout: float = 30.0,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._client = httpx.Client(
            base_url=self._base_url,
            headers={"Authorization": f"Bearer {api_key}"},
            timeout=timeout,
        )

    def close(self) -> None:
        """Close the underlying HTTP client."""
        self._client.close()

    def __enter__(self) -> CreelClient:
        return self

    def __exit__(self, *args: object) -> None:
        self.close()

    # -- internal helpers ----------------------------------------------------

    def _request(
        self,
        method: str,
        path: str,
        *,
        json: Optional[dict[str, Any]] = None,
        params: Optional[dict[str, Any]] = None,
    ) -> dict[str, Any]:
        """Send an HTTP request and return the decoded JSON body.

        Raises CreelError on non-2xx responses.
        """
        # Strip None values from params
        if params:
            params = {k: v for k, v in params.items() if v is not None}

        resp = self._client.request(method, path, json=json, params=params)
        if not resp.is_success:
            try:
                body = resp.json()
                message = body.get("message", resp.text)
            except Exception:
                message = resp.text
            raise CreelError(resp.status_code, message)

        if resp.status_code == 204 or not resp.content:
            return {}
        return resp.json()  # type: ignore[no-any-return]

    @staticmethod
    def _parse_topic(data: dict[str, Any]) -> Topic:
        cs = data.get("chunking_strategy")
        return Topic(
            id=data.get("id", ""),
            slug=data.get("slug", ""),
            name=data.get("name", ""),
            description=data.get("description", ""),
            owner=data.get("owner", ""),
            created_at=data.get("created_at", ""),
            updated_at=data.get("updated_at", ""),
            llm_config_id=data.get("llm_config_id"),
            embedding_config_id=data.get("embedding_config_id"),
            extraction_prompt_config_id=data.get(
                "extraction_prompt_config_id"
            ),
            chunking_strategy=(
                ChunkingStrategy(
                    chunk_size=cs.get("chunk_size", 0),
                    chunk_overlap=cs.get("chunk_overlap", 0),
                )
                if cs
                else None
            ),
        )

    @staticmethod
    def _parse_grant(data: dict[str, Any]) -> TopicGrant:
        return TopicGrant(
            id=data.get("id", ""),
            topic_id=data.get("topic_id", ""),
            principal=data.get("principal", ""),
            permission=data.get("permission", ""),
            granted_by=data.get("granted_by", ""),
            created_at=data.get("created_at", ""),
        )

    @staticmethod
    def _parse_document(data: dict[str, Any]) -> Document:
        return Document(
            id=data.get("id", ""),
            topic_id=data.get("topic_id", ""),
            slug=data.get("slug", ""),
            name=data.get("name", ""),
            doc_type=data.get("doc_type", ""),
            metadata=data.get("metadata"),
            created_at=data.get("created_at", ""),
            updated_at=data.get("updated_at", ""),
            url=data.get("url", ""),
            author=data.get("author", ""),
            published_at=data.get("published_at", ""),
            status=data.get("status", ""),
        )

    @staticmethod
    def _parse_chunk(data: dict[str, Any]) -> Chunk:
        return Chunk(
            id=data.get("id", ""),
            document_id=data.get("document_id", ""),
            sequence=data.get("sequence", 0),
            content=data.get("content", ""),
            embedding_id=data.get("embedding_id", ""),
            embedding_model=data.get("embedding_model", ""),
            status=data.get("status", ""),
            compacted_by=data.get("compacted_by", ""),
            metadata=data.get("metadata"),
            created_at=data.get("created_at", ""),
        )

    @staticmethod
    def _parse_link(data: dict[str, Any]) -> Link:
        return Link(
            id=data.get("id", ""),
            source_chunk_id=data.get("source_chunk_id", ""),
            target_chunk_id=data.get("target_chunk_id", ""),
            link_type=data.get("link_type", ""),
            created_by=data.get("created_by", ""),
            metadata=data.get("metadata"),
            created_at=data.get("created_at", ""),
        )

    @staticmethod
    def _parse_citation(data: dict[str, Any]) -> DocumentCitation:
        return DocumentCitation(
            id=data.get("id", ""),
            slug=data.get("slug", ""),
            name=data.get("name", ""),
            url=data.get("url", ""),
            author=data.get("author", ""),
            published_at=data.get("published_at", ""),
            metadata=data.get("metadata"),
        )

    @classmethod
    def _parse_search_result(cls, data: dict[str, Any]) -> SearchResult:
        chunk_data = data.get("chunk")
        link_data = data.get("via_link")
        cite_data = data.get("document_citation")
        return SearchResult(
            chunk=cls._parse_chunk(chunk_data) if chunk_data else None,
            document_id=data.get("document_id", ""),
            topic_id=data.get("topic_id", ""),
            score=data.get("score", 0.0),
            via_link=cls._parse_link(link_data) if link_data else None,
            document_citation=(
                cls._parse_citation(cite_data) if cite_data else None
            ),
        )

    @staticmethod
    def _parse_memory(data: dict[str, Any]) -> Memory:
        return Memory(
            id=data.get("id", ""),
            principal=data.get("principal", ""),
            scope=data.get("scope", ""),
            content=data.get("content", ""),
            subject=data.get("subject", ""),
            predicate=data.get("predicate", ""),
            object=data.get("object", ""),
            source_chunk_id=data.get("source_chunk_id", ""),
            status=data.get("status", ""),
            invalidated_at=data.get("invalidated_at", ""),
            metadata=data.get("metadata"),
            created_at=data.get("created_at", ""),
            updated_at=data.get("updated_at", ""),
        )

    @staticmethod
    def _parse_job(data: dict[str, Any]) -> ProcessingJob:
        return ProcessingJob(
            id=data.get("id", ""),
            document_id=data.get("document_id", ""),
            job_type=data.get("job_type", ""),
            status=data.get("status", ""),
            progress=data.get("progress"),
            error=data.get("error", ""),
            started_at=data.get("started_at", ""),
            completed_at=data.get("completed_at", ""),
            created_at=data.get("created_at", ""),
        )

    @staticmethod
    def _parse_compaction_record(data: dict[str, Any]) -> CompactionRecord:
        return CompactionRecord(
            id=data.get("id", ""),
            summary_chunk_id=data.get("summary_chunk_id", ""),
            source_chunk_ids=data.get("source_chunk_ids", []),
            document_id=data.get("document_id", ""),
            created_by=data.get("created_by", ""),
            created_at=data.get("created_at", ""),
        )

    # ========================================================================
    # TopicService
    # ========================================================================

    def create_topic(
        self,
        slug: str,
        name: str,
        description: str = "",
        *,
        llm_config_id: Optional[str] = None,
        embedding_config_id: Optional[str] = None,
        extraction_prompt_config_id: Optional[str] = None,
    ) -> Topic:
        body: dict[str, Any] = {
            "slug": slug,
            "name": name,
            "description": description,
        }
        if llm_config_id is not None:
            body["llm_config_id"] = llm_config_id
        if embedding_config_id is not None:
            body["embedding_config_id"] = embedding_config_id
        if extraction_prompt_config_id is not None:
            body["extraction_prompt_config_id"] = extraction_prompt_config_id
        return self._parse_topic(self._request("POST", "/v1/topics", json=body))

    def get_topic(self, topic_id: str) -> Topic:
        return self._parse_topic(self._request("GET", f"/v1/topics/{topic_id}"))

    def list_topics(
        self,
        *,
        page_size: Optional[int] = None,
        page_token: Optional[str] = None,
    ) -> ListTopicsResponse:
        data = self._request(
            "GET",
            "/v1/topics",
            params={"page_size": page_size, "page_token": page_token},
        )
        return ListTopicsResponse(
            topics=[self._parse_topic(t) for t in data.get("topics", [])],
            next_page_token=data.get("next_page_token", ""),
        )

    def update_topic(
        self,
        topic_id: str,
        *,
        name: Optional[str] = None,
        description: Optional[str] = None,
        llm_config_id: Optional[str] = None,
        embedding_config_id: Optional[str] = None,
        extraction_prompt_config_id: Optional[str] = None,
    ) -> Topic:
        body: dict[str, Any] = {}
        if name is not None:
            body["name"] = name
        if description is not None:
            body["description"] = description
        if llm_config_id is not None:
            body["llm_config_id"] = llm_config_id
        if embedding_config_id is not None:
            body["embedding_config_id"] = embedding_config_id
        if extraction_prompt_config_id is not None:
            body["extraction_prompt_config_id"] = extraction_prompt_config_id
        return self._parse_topic(
            self._request("PATCH", f"/v1/topics/{topic_id}", json=body)
        )

    def delete_topic(self, topic_id: str) -> None:
        self._request("DELETE", f"/v1/topics/{topic_id}")

    def grant_access(
        self, topic_id: str, principal: str, permission: str
    ) -> TopicGrant:
        body = {"principal": principal, "permission": permission}
        return self._parse_grant(
            self._request(
                "POST", f"/v1/topics/{topic_id}/grants", json=body
            )
        )

    def revoke_access(self, topic_id: str, principal: str) -> None:
        self._request(
            "DELETE", f"/v1/topics/{topic_id}/grants/{principal}"
        )

    def list_grants(self, topic_id: str) -> ListGrantsResponse:
        data = self._request("GET", f"/v1/topics/{topic_id}/grants")
        return ListGrantsResponse(
            grants=[self._parse_grant(g) for g in data.get("grants", [])]
        )

    # ========================================================================
    # DocumentService
    # ========================================================================

    def create_document(
        self,
        topic_id: str,
        name: str,
        *,
        slug: Optional[str] = None,
        doc_type: Optional[str] = None,
        metadata: Optional[dict[str, Any]] = None,
        url: Optional[str] = None,
        author: Optional[str] = None,
        published_at: Optional[str] = None,
    ) -> Document:
        body: dict[str, Any] = {"topic_id": topic_id, "name": name}
        if slug is not None:
            body["slug"] = slug
        if doc_type is not None:
            body["doc_type"] = doc_type
        if metadata is not None:
            body["metadata"] = metadata
        if url is not None:
            body["url"] = url
        if author is not None:
            body["author"] = author
        if published_at is not None:
            body["published_at"] = published_at
        return self._parse_document(
            self._request("POST", "/v1/documents", json=body)
        )

    def get_document(self, document_id: str) -> Document:
        return self._parse_document(
            self._request("GET", f"/v1/documents/{document_id}")
        )

    def list_documents(
        self,
        topic_id: str,
        *,
        page_size: Optional[int] = None,
        page_token: Optional[str] = None,
    ) -> ListDocumentsResponse:
        data = self._request(
            "GET",
            f"/v1/topics/{topic_id}/documents",
            params={"page_size": page_size, "page_token": page_token},
        )
        return ListDocumentsResponse(
            documents=[
                self._parse_document(d) for d in data.get("documents", [])
            ],
            next_page_token=data.get("next_page_token", ""),
        )

    def update_document(
        self,
        document_id: str,
        *,
        name: Optional[str] = None,
        doc_type: Optional[str] = None,
        metadata: Optional[dict[str, Any]] = None,
        url: Optional[str] = None,
        author: Optional[str] = None,
        published_at: Optional[str] = None,
    ) -> Document:
        body: dict[str, Any] = {}
        if name is not None:
            body["name"] = name
        if doc_type is not None:
            body["doc_type"] = doc_type
        if metadata is not None:
            body["metadata"] = metadata
        if url is not None:
            body["url"] = url
        if author is not None:
            body["author"] = author
        if published_at is not None:
            body["published_at"] = published_at
        return self._parse_document(
            self._request("PATCH", f"/v1/documents/{document_id}", json=body)
        )

    def delete_document(self, document_id: str) -> None:
        self._request("DELETE", f"/v1/documents/{document_id}")

    def upload_document(
        self,
        topic_id: str,
        name: str,
        file: bytes,
        *,
        slug: Optional[str] = None,
        source_url: Optional[str] = None,
        content_type: Optional[str] = None,
        doc_type: Optional[str] = None,
        metadata: Optional[dict[str, Any]] = None,
        author: Optional[str] = None,
        published_at: Optional[str] = None,
    ) -> UploadDocumentResponse:
        body: dict[str, Any] = {
            "topic_id": topic_id,
            "name": name,
            "file": base64.b64encode(file).decode("ascii"),
        }
        if slug is not None:
            body["slug"] = slug
        if source_url is not None:
            body["source_url"] = source_url
        if content_type is not None:
            body["content_type"] = content_type
        if doc_type is not None:
            body["doc_type"] = doc_type
        if metadata is not None:
            body["metadata"] = metadata
        if author is not None:
            body["author"] = author
        if published_at is not None:
            body["published_at"] = published_at
        data = self._request("POST", "/v1/documents:upload", json=body)
        doc_data = data.get("document")
        return UploadDocumentResponse(
            document=self._parse_document(doc_data) if doc_data else None,
            job_id=data.get("job_id", ""),
        )

    # ========================================================================
    # ChunkService
    # ========================================================================

    def ingest_chunks(
        self,
        document_id: str,
        chunks: list[dict[str, Any]],
    ) -> IngestChunksResponse:
        """Ingest chunks into a document.

        Each item in ``chunks`` should be a dict with keys: ``content``,
        ``sequence``, and optionally ``embedding`` (list of floats) and
        ``metadata`` (dict).
        """
        body: dict[str, Any] = {"chunks": chunks}
        data = self._request(
            "POST", f"/v1/documents/{document_id}/chunks", json=body
        )
        return IngestChunksResponse(
            chunks=[self._parse_chunk(c) for c in data.get("chunks", [])]
        )

    def get_chunk(self, chunk_id: str) -> Chunk:
        return self._parse_chunk(
            self._request("GET", f"/v1/chunks/{chunk_id}")
        )

    def delete_chunk(self, chunk_id: str) -> None:
        self._request("DELETE", f"/v1/chunks/{chunk_id}")

    # ========================================================================
    # RetrievalService
    # ========================================================================

    def search(
        self,
        topic_ids: list[str],
        *,
        query_text: Optional[str] = None,
        query_embedding: Optional[list[float]] = None,
        top_k: Optional[int] = None,
        follow_links: Optional[bool] = None,
        link_depth: Optional[int] = None,
        metadata_filter: Optional[dict[str, Any]] = None,
        exclude_document_ids: Optional[list[str]] = None,
    ) -> SearchResponse:
        body: dict[str, Any] = {"topic_ids": topic_ids}
        if query_text is not None:
            body["query_text"] = query_text
        if query_embedding is not None:
            body["query_embedding"] = query_embedding
        if top_k is not None:
            body["top_k"] = top_k
        if follow_links is not None:
            body["follow_links"] = follow_links
        if link_depth is not None:
            body["link_depth"] = link_depth
        if metadata_filter is not None:
            body["metadata_filter"] = metadata_filter
        if exclude_document_ids is not None:
            body["exclude_document_ids"] = exclude_document_ids
        data = self._request("POST", "/v1/search", json=body)
        return SearchResponse(
            results=[
                self._parse_search_result(r) for r in data.get("results", [])
            ]
        )

    def get_context(
        self,
        document_id: str,
        *,
        last_n: Optional[int] = None,
        since: Optional[str] = None,
        include_summaries: Optional[bool] = None,
    ) -> GetContextResponse:
        data = self._request(
            "GET",
            f"/v1/documents/{document_id}/context",
            params={
                "last_n": last_n,
                "since": since,
                "include_summaries": include_summaries,
            },
        )
        return GetContextResponse(
            chunks=[self._parse_chunk(c) for c in data.get("chunks", [])]
        )

    # ========================================================================
    # MemoryService
    # ========================================================================

    def get_memories(
        self,
        *,
        scopes: Optional[list[str]] = None,
    ) -> list[Memory]:
        """Retrieve memories, optionally filtered by scopes.

        If ``scopes`` is empty or None, returns memories across all scopes.
        """
        params: dict[str, Any] = {}
        if scopes:
            params["scopes"] = ",".join(scopes)
        data = self._request("GET", "/v1/memories", params=params)
        return [self._parse_memory(m) for m in data.get("memories", [])]

    def add_messages(
        self,
        scope: str,
        messages: list[dict[str, str]],
    ) -> AddMessagesResponse:
        """Send conversation messages for automatic fact extraction.

        Each item in ``messages`` should be a dict with ``role`` and
        ``content`` keys.  Returns job IDs for the extraction jobs.
        """
        body: dict[str, Any] = {"scope": scope, "messages": messages}
        data = self._request("POST", "/v1/memories:add-messages", json=body)
        return AddMessagesResponse(job_ids=data.get("job_ids", []))

    def add_memory(
        self,
        scope: str,
        content: str,
        *,
        subject: Optional[str] = None,
        predicate: Optional[str] = None,
        object: Optional[str] = None,
        metadata: Optional[dict[str, Any]] = None,
    ) -> AddMemoryResponse:
        """Queue a memory observation for processing.

        Returns an AddMemoryResponse containing a job_id. The memory
        maintenance worker handles deduplication asynchronously.
        """
        body: dict[str, Any] = {"scope": scope, "content": content}
        if subject is not None:
            body["subject"] = subject
        if predicate is not None:
            body["predicate"] = predicate
        if object is not None:
            body["object"] = object
        if metadata is not None:
            body["metadata"] = metadata
        data = self._request("POST", "/v1/memories", json=body)
        return AddMemoryResponse(job_id=data.get("job_id", ""))

    def update_memory(
        self,
        memory_id: str,
        *,
        content: Optional[str] = None,
        metadata: Optional[dict[str, Any]] = None,
    ) -> Memory:
        body: dict[str, Any] = {}
        if content is not None:
            body["content"] = content
        if metadata is not None:
            body["metadata"] = metadata
        return self._parse_memory(
            self._request("PATCH", f"/v1/memories/{memory_id}", json=body)
        )

    def delete_memory(self, memory_id: str) -> None:
        self._request("DELETE", f"/v1/memories/{memory_id}")

    def list_memories(
        self,
        scope: str,
        *,
        include_invalidated: Optional[bool] = None,
    ) -> ListMemoriesResponse:
        data = self._request(
            "GET",
            f"/v1/memories/{scope}/list",
            params={"include_invalidated": include_invalidated},
        )
        return ListMemoriesResponse(
            memories=[self._parse_memory(m) for m in data.get("memories", [])]
        )

    def list_scopes(self) -> ListScopesResponse:
        data = self._request("GET", "/v1/memories:scopes")
        return ListScopesResponse(scopes=data.get("scopes", []))

    # ========================================================================
    # LinkService
    # ========================================================================

    def create_link(
        self,
        source_chunk_id: str,
        target_chunk_id: str,
        *,
        link_type: Optional[str] = None,
        metadata: Optional[dict[str, Any]] = None,
    ) -> Link:
        body: dict[str, Any] = {
            "source_chunk_id": source_chunk_id,
            "target_chunk_id": target_chunk_id,
        }
        if link_type is not None:
            body["link_type"] = link_type
        if metadata is not None:
            body["metadata"] = metadata
        return self._parse_link(
            self._request("POST", "/v1/links", json=body)
        )

    def delete_link(self, link_id: str) -> None:
        self._request("DELETE", f"/v1/links/{link_id}")

    def list_links(
        self,
        chunk_id: str,
        *,
        include_backlinks: Optional[bool] = None,
    ) -> ListLinksResponse:
        data = self._request(
            "GET",
            f"/v1/chunks/{chunk_id}/links",
            params={"include_backlinks": include_backlinks},
        )
        return ListLinksResponse(
            links=[self._parse_link(l) for l in data.get("links", [])]
        )

    # ========================================================================
    # CompactionService
    # ========================================================================

    def compact(
        self,
        document_id: str,
        chunk_ids: list[str],
        summary_content: str,
        *,
        summary_embedding: Optional[list[float]] = None,
        summary_metadata: Optional[dict[str, Any]] = None,
    ) -> CompactResponse:
        body: dict[str, Any] = {
            "document_id": document_id,
            "chunk_ids": chunk_ids,
            "summary_content": summary_content,
        }
        if summary_embedding is not None:
            body["summary_embedding"] = summary_embedding
        if summary_metadata is not None:
            body["summary_metadata"] = summary_metadata
        data = self._request("POST", "/v1/compact", json=body)
        sc = data.get("summary_chunk")
        return CompactResponse(
            summary_chunk=self._parse_chunk(sc) if sc else None,
            compacted_count=data.get("compacted_count", 0),
        )

    def uncompact(self, summary_chunk_id: str) -> UncompactResponse:
        body = {"summary_chunk_id": summary_chunk_id}
        data = self._request("POST", "/v1/uncompact", json=body)
        return UncompactResponse(
            restored_chunks=[
                self._parse_chunk(c)
                for c in data.get("restored_chunks", [])
            ]
        )

    def request_compaction(
        self,
        document_id: str,
        *,
        chunk_ids: Optional[list[str]] = None,
    ) -> RequestCompactionResponse:
        body: dict[str, Any] = {"document_id": document_id}
        if chunk_ids is not None:
            body["chunk_ids"] = chunk_ids
        data = self._request("POST", "/v1/compact/request", json=body)
        return RequestCompactionResponse(
            job_id=data.get("job_id", "")
        )

    def get_compaction_history(
        self, document_id: str
    ) -> GetCompactionHistoryResponse:
        data = self._request(
            "GET", f"/v1/documents/{document_id}/compaction-history"
        )
        return GetCompactionHistoryResponse(
            records=[
                self._parse_compaction_record(r)
                for r in data.get("records", [])
            ]
        )

    # ========================================================================
    # JobService
    # ========================================================================

    def get_job(self, job_id: str) -> ProcessingJob:
        return self._parse_job(self._request("GET", f"/v1/jobs/{job_id}"))

    def list_jobs(
        self,
        *,
        topic_id: Optional[str] = None,
        document_id: Optional[str] = None,
        status: Optional[str] = None,
        page_size: Optional[int] = None,
        page_token: Optional[str] = None,
    ) -> ListJobsResponse:
        data = self._request(
            "GET",
            "/v1/jobs",
            params={
                "topic_id": topic_id,
                "document_id": document_id,
                "status": status,
                "page_size": page_size,
                "page_token": page_token,
            },
        )
        return ListJobsResponse(
            jobs=[self._parse_job(j) for j in data.get("jobs", [])],
            next_page_token=data.get("next_page_token", ""),
        )

    # ========================================================================
    # AdminService
    # ========================================================================

    def health(self) -> HealthResponse:
        data = self._request("GET", "/v1/health")
        return HealthResponse(
            status=data.get("status", ""),
            version=data.get("version", ""),
        )
