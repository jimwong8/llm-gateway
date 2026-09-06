-- Knowledge Graph (KG) tables for the local memory module.
-- Implements 10.100.1.13 KG API capabilities (§4.3 SESSION_MEMORY_RESEARCH.md).
-- These tables mirror the kg_entity/kg_relation storage model from the reference system.

CREATE TABLE IF NOT EXISTS kg_entities (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    entity_name TEXT NOT NULL,
    entity_type TEXT NOT NULL DEFAULT 'other',
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    visibility TEXT NOT NULL DEFAULT 'private',
    embedding VECTOR(384),
    session_ids BIGINT[] NOT NULL DEFAULT '{}',
    source_text TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_kg_entities_user_name
    ON kg_entities (user_id, entity_name) WHERE status='active';

CREATE INDEX IF NOT EXISTS idx_kg_entities_user_updated
    ON kg_entities (user_id, updated_at DESC) WHERE status='active';

CREATE INDEX IF NOT EXISTS idx_kg_entities_type
    ON kg_entities (entity_type) WHERE status='active';

CREATE INDEX IF NOT EXISTS idx_kg_entities_embedding
    ON kg_entities USING ivfflat (embedding vector_cosine_ops)
    WHERE embedding IS NOT NULL AND status='active';

CREATE TABLE IF NOT EXISTS kg_relations (
    id BIGSERIAL PRIMARY KEY,
    from_entity_id BIGINT NOT NULL REFERENCES kg_entities(id) ON DELETE CASCADE,
    to_entity_id BIGINT NOT NULL REFERENCES kg_entities(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL DEFAULT 'relates_to',
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    visibility TEXT NOT NULL DEFAULT 'private',
    session_ids BIGINT[] NOT NULL DEFAULT '{}',
    source_text TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_kg_relations_unique
    ON kg_relations (from_entity_id, to_entity_id, relation_type) WHERE status='active';

CREATE INDEX IF NOT EXISTS idx_kg_relations_from
    ON kg_relations (from_entity_id) WHERE status NOT IN ('deleted','superseded');

CREATE INDEX IF NOT EXISTS idx_kg_relations_to
    ON kg_relations (to_entity_id) WHERE status NOT IN ('deleted','superseded');

CREATE INDEX IF NOT EXISTS idx_kg_relations_type
    ON kg_relations (relation_type) WHERE status NOT IN ('deleted','superseded');

CREATE INDEX IF NOT EXISTS idx_kg_relations_session_ids
    ON kg_relations USING gin (session_ids) WHERE session_ids IS NOT NULL;
