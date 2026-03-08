ALTER TABLE topics
    DROP COLUMN IF EXISTS extraction_prompt_config_id,
    DROP COLUMN IF EXISTS embedding_config_id,
    DROP COLUMN IF EXISTS llm_config_id;

DROP TABLE IF EXISTS extraction_prompt_configs;
DROP TABLE IF EXISTS embedding_configs;
DROP TABLE IF EXISTS llm_configs;
DROP TABLE IF EXISTS api_key_configs;
