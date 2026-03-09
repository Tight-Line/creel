CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    principal TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'default',
    content TEXT NOT NULL,
    embedding_id TEXT,
    subject TEXT,
    predicate TEXT,
    object TEXT,
    source_chunk_id UUID REFERENCES chunks(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'active',
    invalidated_at TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_memories_principal_scope ON memories(principal, scope);
CREATE INDEX idx_memories_status ON memories(status);
CREATE INDEX idx_memories_embedding_id ON memories(embedding_id);
