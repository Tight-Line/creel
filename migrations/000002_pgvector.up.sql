CREATE EXTENSION IF NOT EXISTS vector SCHEMA public;

CREATE TABLE chunk_embeddings (
    chunk_id   UUID PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    embedding  vector(1536) NOT NULL,
    metadata   JSONB NOT NULL DEFAULT '{}'
);
