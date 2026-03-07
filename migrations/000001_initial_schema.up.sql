CREATE EXTENSION IF NOT EXISTS "uuid-ossp" SCHEMA public;

CREATE TABLE topics (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE topic_grants (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    topic_id    UUID NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    principal   TEXT NOT NULL,
    permission  TEXT NOT NULL CHECK (permission IN ('read', 'write', 'admin')),
    granted_by  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (topic_id, principal)
);

CREATE TABLE documents (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    topic_id    UUID NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    doc_type    TEXT NOT NULL DEFAULT 'reference',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (topic_id, slug)
);

CREATE TABLE chunks (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id   UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    sequence      INT NOT NULL,
    content       TEXT NOT NULL,
    embedding_id  TEXT,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'compacted')),
    compacted_by  UUID REFERENCES chunks(id),
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chunks_document_id ON chunks(document_id);
CREATE INDEX idx_chunks_status ON chunks(status);

CREATE TABLE links (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_chunk  UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    target_chunk  UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    link_type     TEXT NOT NULL DEFAULT 'manual' CHECK (link_type IN ('manual', 'auto', 'compaction_transfer')),
    created_by    TEXT NOT NULL,
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_links_source ON links(source_chunk);
CREATE INDEX idx_links_target ON links(target_chunk);

CREATE TABLE system_accounts (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL UNIQUE,
    description   TEXT NOT NULL DEFAULT '',
    principal     TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE system_account_keys (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id        UUID NOT NULL REFERENCES system_accounts(id) ON DELETE CASCADE,
    key_hash          TEXT NOT NULL UNIQUE,
    key_prefix        TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'grace_period', 'revoked')),
    grace_expires_at  TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at        TIMESTAMPTZ
);
