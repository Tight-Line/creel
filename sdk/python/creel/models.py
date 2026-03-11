"""Dataclass models for Creel API responses."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Optional


# -- Topics ------------------------------------------------------------------

@dataclass
class ChunkingStrategy:
    chunk_size: int = 0
    chunk_overlap: int = 0


@dataclass
class Topic:
    id: str = ""
    slug: str = ""
    name: str = ""
    description: str = ""
    owner: str = ""
    created_at: str = ""
    updated_at: str = ""
    llm_config_id: Optional[str] = None
    embedding_config_id: Optional[str] = None
    extraction_prompt_config_id: Optional[str] = None
    chunking_strategy: Optional[ChunkingStrategy] = None


@dataclass
class TopicGrant:
    id: str = ""
    topic_id: str = ""
    principal: str = ""
    permission: str = ""
    granted_by: str = ""
    created_at: str = ""


@dataclass
class ListTopicsResponse:
    topics: list[Topic] = field(default_factory=list)
    next_page_token: str = ""


@dataclass
class ListGrantsResponse:
    grants: list[TopicGrant] = field(default_factory=list)


# -- Documents ---------------------------------------------------------------

@dataclass
class Document:
    id: str = ""
    topic_id: str = ""
    slug: str = ""
    name: str = ""
    doc_type: str = ""
    metadata: Optional[dict[str, Any]] = None
    created_at: str = ""
    updated_at: str = ""
    url: str = ""
    author: str = ""
    published_at: str = ""
    status: str = ""


@dataclass
class ListDocumentsResponse:
    documents: list[Document] = field(default_factory=list)
    next_page_token: str = ""


@dataclass
class UploadDocumentResponse:
    document: Optional[Document] = None
    job_id: str = ""


# -- Chunks ------------------------------------------------------------------

@dataclass
class Chunk:
    id: str = ""
    document_id: str = ""
    sequence: int = 0
    content: str = ""
    embedding_id: str = ""
    embedding_model: str = ""
    status: str = ""
    compacted_by: str = ""
    metadata: Optional[dict[str, Any]] = None
    created_at: str = ""


@dataclass
class IngestChunksResponse:
    chunks: list[Chunk] = field(default_factory=list)


# -- Retrieval ---------------------------------------------------------------

@dataclass
class DocumentCitation:
    id: str = ""
    slug: str = ""
    name: str = ""
    url: str = ""
    author: str = ""
    published_at: str = ""
    metadata: Optional[dict[str, Any]] = None


@dataclass
class Link:
    id: str = ""
    source_chunk_id: str = ""
    target_chunk_id: str = ""
    link_type: str = ""
    created_by: str = ""
    metadata: Optional[dict[str, Any]] = None
    created_at: str = ""


@dataclass
class SearchResult:
    chunk: Optional[Chunk] = None
    document_id: str = ""
    topic_id: str = ""
    score: float = 0.0
    via_link: Optional[Link] = None
    document_citation: Optional[DocumentCitation] = None


@dataclass
class SearchResponse:
    results: list[SearchResult] = field(default_factory=list)


@dataclass
class GetContextResponse:
    chunks: list[Chunk] = field(default_factory=list)


# -- Memory ------------------------------------------------------------------

@dataclass
class Memory:
    id: str = ""
    principal: str = ""
    scope: str = ""
    content: str = ""
    subject: str = ""
    predicate: str = ""
    object: str = ""
    source_chunk_id: str = ""
    status: str = ""
    invalidated_at: str = ""
    metadata: Optional[dict[str, Any]] = None
    created_at: str = ""
    updated_at: str = ""


@dataclass
class AddMemoryResponse:
    job_id: str = ""


@dataclass
class ListMemoriesResponse:
    memories: list[Memory] = field(default_factory=list)


@dataclass
class ListScopesResponse:
    scopes: list[str] = field(default_factory=list)


@dataclass
class AddMessagesResponse:
    job_ids: list[str] = field(default_factory=list)


# -- Links -------------------------------------------------------------------

@dataclass
class ListLinksResponse:
    links: list[Link] = field(default_factory=list)


# -- Compaction --------------------------------------------------------------

@dataclass
class CompactResponse:
    summary_chunk: Optional[Chunk] = None
    compacted_count: int = 0


@dataclass
class UncompactResponse:
    restored_chunks: list[Chunk] = field(default_factory=list)


@dataclass
class RequestCompactionResponse:
    job_id: str = ""


@dataclass
class CompactionRecord:
    id: str = ""
    summary_chunk_id: str = ""
    source_chunk_ids: list[str] = field(default_factory=list)
    document_id: str = ""
    created_by: str = ""
    created_at: str = ""


@dataclass
class GetCompactionHistoryResponse:
    records: list[CompactionRecord] = field(default_factory=list)


# -- Jobs --------------------------------------------------------------------

@dataclass
class ProcessingJob:
    id: str = ""
    document_id: str = ""
    job_type: str = ""
    status: str = ""
    progress: Optional[dict[str, Any]] = None
    error: str = ""
    started_at: str = ""
    completed_at: str = ""
    created_at: str = ""


@dataclass
class ListJobsResponse:
    jobs: list[ProcessingJob] = field(default_factory=list)
    next_page_token: str = ""


# -- Admin -------------------------------------------------------------------

@dataclass
class HealthResponse:
    status: str = ""
    version: str = ""
