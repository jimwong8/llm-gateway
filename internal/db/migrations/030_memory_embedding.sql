-- Add vector extension and embedding column to memory_atoms for hybrid search.
-- This migration must run after 026_memory_pyramid.sql.

CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE memory_atoms ADD COLUMN IF NOT EXISTS embedding vector(384);
CREATE INDEX IF NOT EXISTS idx_memory_atoms_embedding ON memory_atoms USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
