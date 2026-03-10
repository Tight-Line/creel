-- Vector backend configs (type + connection parameters)
CREATE TABLE vector_backend_configs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    backend    TEXT NOT NULL,  -- e.g. 'pgvector', 'qdrant', 'weaviate'
    config     JSONB NOT NULL DEFAULT '{}',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_vector_backend_configs_default
    ON vector_backend_configs (is_default) WHERE is_default = true;

-- Bind vector backend config to topics
ALTER TABLE topics
    ADD COLUMN vector_backend_config_id UUID REFERENCES vector_backend_configs(id) ON DELETE SET NULL;
