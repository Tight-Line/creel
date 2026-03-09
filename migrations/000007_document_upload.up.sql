-- Add status column to documents.
ALTER TABLE documents ADD COLUMN status TEXT NOT NULL DEFAULT 'ready';

-- Separate table for raw content and extracted text to keep documents table lean.
CREATE TABLE document_content (
    document_id UUID PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
    raw_content BYTEA,
    content_type TEXT NOT NULL DEFAULT '',
    extracted_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
