ALTER TABLE topics DROP COLUMN IF EXISTS vector_backend_config_id;
DROP TABLE IF EXISTS vector_backend_configs;
