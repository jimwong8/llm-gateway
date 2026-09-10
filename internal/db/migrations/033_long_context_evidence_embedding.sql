-- Add optional embedding storage to long_context_evidence for hybrid retrieval.
-- Stored as JSONB so it works without the pgvector extension.
ALTER TABLE long_context_evidence ADD COLUMN IF NOT EXISTS embedding JSONB;
