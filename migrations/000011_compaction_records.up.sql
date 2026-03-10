CREATE TABLE IF NOT EXISTS compaction_records (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    summary_chunk_id  UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    source_chunk_ids  UUID[] NOT NULL,
    document_id       UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    created_by        TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_compaction_records_document_id ON compaction_records(document_id);
CREATE INDEX IF NOT EXISTS idx_compaction_records_summary_chunk_id ON compaction_records(summary_chunk_id);
