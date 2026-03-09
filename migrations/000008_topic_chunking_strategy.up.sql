-- Add chunking_strategy JSONB column to topics.
-- NULL means use server defaults (chunk_size=2048 chars, chunk_overlap=200 chars).
ALTER TABLE topics ADD COLUMN chunking_strategy JSONB;
