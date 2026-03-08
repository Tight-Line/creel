-- API key configs (standalone; referenced by llm_configs and embedding_configs)
CREATE TABLE api_key_configs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL UNIQUE,
    provider      TEXT NOT NULL,
    encrypted_key BYTEA NOT NULL,
    key_nonce     BYTEA NOT NULL,
    is_default    BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_api_key_configs_default ON api_key_configs (is_default) WHERE is_default = true;

-- LLM configs (references api_key_config)
CREATE TABLE llm_configs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL UNIQUE,
    provider          TEXT NOT NULL,
    model             TEXT NOT NULL,
    parameters        JSONB NOT NULL DEFAULT '{}',
    api_key_config_id UUID NOT NULL REFERENCES api_key_configs(id),
    is_default        BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_llm_configs_default ON llm_configs (is_default) WHERE is_default = true;

-- Embedding configs (references api_key_config)
CREATE TABLE embedding_configs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL UNIQUE,
    provider          TEXT NOT NULL,
    model             TEXT NOT NULL,
    dimensions        INT NOT NULL,
    api_key_config_id UUID NOT NULL REFERENCES api_key_configs(id),
    is_default        BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_embedding_configs_default ON embedding_configs (is_default) WHERE is_default = true;

-- Extraction prompt configs (standalone)
CREATE TABLE extraction_prompt_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    prompt      TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_extraction_prompt_configs_default
    ON extraction_prompt_configs (is_default) WHERE is_default = true;

-- Bind configs to topics
ALTER TABLE topics
    ADD COLUMN llm_config_id               UUID REFERENCES llm_configs(id) ON DELETE SET NULL,
    ADD COLUMN embedding_config_id         UUID REFERENCES embedding_configs(id) ON DELETE SET NULL,
    ADD COLUMN extraction_prompt_config_id UUID REFERENCES extraction_prompt_configs(id) ON DELETE SET NULL;
