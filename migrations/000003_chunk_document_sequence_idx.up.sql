-- Composite index for GetContext queries: filter by document + status, sort by sequence.
-- Covers both the "all chunks" (ORDER BY sequence ASC) and "last N" (ORDER BY sequence DESC LIMIT N) paths.
CREATE INDEX IF NOT EXISTS idx_chunks_document_status_sequence ON chunks(document_id, status, sequence);
