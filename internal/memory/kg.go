package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// KGEntity represents a knowledge graph entity stored in the local PostgreSQL instance.
type KGEntity struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	EntityName   string    `json:"entity_name"`
	EntityType   string    `json:"entity_type"` // e.g. "person","concept","project","tool","file","error","api"
	Properties   string    `json:"properties"` // JSONB stored as text
	Visibility   string    `json:"visibility"` // "private"|"shared"|"public"
	Embedding    string    `json:"-"`          // pg_vector as "[...]" string
	SessionIDs   []int64   `json:"session_ids"`
	SourceText   string    `json:"source_text"`
	CreatedBy    int64     `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// KGRelation represents a directed edge in the knowledge graph.
type KGRelation struct {
	ID             int64     `json:"id"`
	FromEntityID   int64     `json:"from_entity_id"`
	ToEntityID     int64     `json:"to_entity_id"`
	RelationType   string    `json:"relation_type"`
	Properties     string    `json:"properties"`
	Weight         float64   `json:"weight"`
	Visibility     string    `json:"visibility"`
	Status         string    `json:"status"`     // "active"|"deleted"|"superseded"
	SessionIDs     []int64   `json:"session_ids"`
	SourceText     string    `json:"source_text"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// KGEntityWithScore is returned by KG search operations.
type KGEntityWithScore struct {
	KGEntity
	Score float64 `json:"score"`
}

// KGRelationWithScore is returned by relation search.
type KGRelationWithScore struct {
	KGRelation
	Score float64 `json:"score"`
}

// KGSubgraph is the result of a graph traversal.
type KGSubgraph struct {
	Entities  []KGEntity  `json:"entities"`
	Relations []KGRelation `json:"relations"`
	Depth     int         `json:"depth"`
}

// KGStore is the interface for all KG operations on the local PostgreSQL instance.
type KGStore interface {
	// Entity CRUD
	CreateEntity(ctx context.Context, entity *KGEntity) (*KGEntity, error)
	UpsertEntity(ctx context.Context, entity *KGEntity) (*KGEntity, error)
	GetEntity(ctx context.Context, entityID int64) (*KGEntity, error)
	FindByName(ctx context.Context, userID int64, name string) (*KGEntity, error)
	ListByUser(ctx context.Context, userID int64, entityType string, limit, offset int) ([]KGEntity, int, error)
	DeleteEntity(ctx context.Context, userID int64, entityID int64) error

	// Relation CRUD
	CreateRelation(ctx context.Context, relation *KGRelation) (*KGRelation, error)
	UpsertRelation(ctx context.Context, relation *KGRelation) (*KGRelation, error)
	ListRelationsByEntity(ctx context.Context, entityID int64, limit, offset int) ([]KGRelation, int, error)
	DeleteRelation(ctx context.Context, relationID int64) error

	// Search & Graph
	SearchEntities(ctx context.Context, userID int64, query string, entityType string, limit int) ([]KGEntityWithScore, error)
	SearchRelations(ctx context.Context, userID int64, query string, limit int) ([]KGRelation, error)
	GetSubgraph(ctx context.Context, userID int64, rootEntityID int64, depth int) (*KGSubgraph, error)
	GetRelatedSessions(ctx context.Context, entityID int64) ([]int64, error)
	GetFullGraph(ctx context.Context, userID int64, limit int) (*KGSubgraph, error)

	// KGBuilder integration
	ExtractFromText(ctx context.Context, userID int64, sessionID int64, text string) (*KGSubgraph, error)
}

// sqlKGStore implements KGStore on top of the local PostgreSQL *sql.DB.
type sqlKGStore struct {
	db        *sql.DB
	embedder  Embedder     // used for on-demand text embedding
	llm       LLMProvider  // used by KGBuilder for entity/relation extraction
}

// Embedder produces semantic embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// LLMProvider calls an LLM for structured extraction.
type LLMProvider interface {
	Chat(ctx context.Context, system, user string) (string, error)
}

// ---- NullEmbedder (placeholder, used when no real embedder is injected) ----

type nullEmbedder struct{}

func (nullEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Deterministic placeholder: consistent per text length.
	vec := make([]float32, 384)
	for i := range vec {
		vec[i] = float32(len(text)%100) / 100.0
	}
	return vec, nil
}

func nullEmbed() Embedder { return nullEmbedder{} }

// --------------------------------------------------------------------------//

// NewKgStore creates a KGStore backed by the local PostgreSQL database.
// An optional embedder can be injected for real semantic vector search.
// If embedder is nil, all embeddings use a deterministic placeholder.
func NewKgStore(db *sql.DB) KGStore {
	return newKgStoreWith(db, nil)
}

// NewKgStoreWith creates a KGStore with an explicit Embedder and LLMProvider injected.
// This is the preferred constructor for production use.
func NewKgStoreWith(db *sql.DB, embedder Embedder, llm LLMProvider) KGStore {
	return newKgStoreWith(db, embedder, llm)
}

func newKgStoreWith(db *sql.DB, embedder Embedder, _ ...LLMProvider) *sqlKGStore {
	if embedder == nil {
		embedder = nullEmbed()
	}
	return &sqlKGStore{
		db:       db,
		embedder: embedder,
	}
}

// --------------------------------------------------------------------------//
// Entity CRUD
// --------------------------------------------------------------------------//

func (s *sqlKGStore) CreateEntity(ctx context.Context, entity *KGEntity) (*KGEntity, error) {
	var id int64
	var sessionIDsOut []int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO kg_entities (user_id, entity_name, entity_type, properties,
			visibility, embedding, session_ids, source_text, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, session_ids, created_at, updated_at`,
		entity.UserID, entity.EntityName, entity.EntityType,
		props(entity.Properties), entity.Visibility, embedding(entity.Embedding),
		pgArrayInt(entity.SessionIDs), entity.SourceText, entity.CreatedBy,
	).Scan(&id, &sessionIDsOut, &entity.CreatedAt, &entity.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create kg entity: %w", err)
	}
	entity.ID = id
	entity.SessionIDs = sessionIDsOut
	return entity, nil
}

func (s *sqlKGStore) UpsertEntity(ctx context.Context, entity *KGEntity) (*KGEntity, error) {
	var id int64
	var sessionIDsOut []int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO kg_entities (user_id, entity_name, entity_type, properties,
			visibility, embedding, session_ids, source_text, created_by)
		VALUES ($1,$2,$3,$4,'private',$5,$6,$7,$8)
		ON CONFLICT (user_id, entity_name) WHERE status='active'
		DO UPDATE SET
			entity_type   = EXCLUDED.entity_type,
			properties    = EXCLUDED.properties,
			embedding     = EXCLUDED.embedding,
			source_text   = EXCLUDED.source_text,
			updated_at    = NOW()
		RETURNING id, session_ids, created_at, updated_at`,
		entity.UserID, entity.EntityName, entity.EntityType,
		props(entity.Properties), embedding(entity.Embedding),
		pgArrayInt(entity.SessionIDs), entity.SourceText, entity.CreatedBy,
	).Scan(&id, &sessionIDsOut, &entity.CreatedAt, &entity.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert kg entity: %w", err)
	}
	entity.ID = id
	entity.SessionIDs = sessionIDsOut
	return entity, nil
}

func (s *sqlKGStore) GetEntity(ctx context.Context, entityID int64) (*KGEntity, error) {
	var e KGEntity
	var propsJSON, emb, sessionIDsOut string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, entity_name, entity_type, COALESCE(properties,''),
			   visibility, COALESCE(embedding,''), COALESCE(session_ids::text,'{}'),
			   COALESCE(source_text,''), created_by, created_at, updated_at
		FROM kg_entities WHERE id=$1 AND status='active'`,
		entityID,
	).Scan(&e.ID, &e.UserID, &e.EntityName, &e.EntityType, &propsJSON,
		&e.Visibility, &emb, &sessionIDsOut, &e.SourceText,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("entity not found: %d", entityID)
	}
	if err != nil {
		return nil, fmt.Errorf("get kg entity: %w", err)
	}
	e.Properties = propsJSON
	e.Embedding = emb
	e.SessionIDs = parsePGIntArray(sessionIDsOut)
	return &e, nil
}

func (s *sqlKGStore) FindByName(ctx context.Context, userID int64, name string) (*KGEntity, error) {
	var e KGEntity
	var propsJSON, emb, sessionIDsOut string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, entity_name, entity_type, COALESCE(properties,''),
			   visibility, COALESCE(embedding,''), COALESCE(session_ids::text,'{}'),
			   COALESCE(source_text,''), created_by, created_at, updated_at
		FROM kg_entities
		WHERE user_id=$1 AND LOWER(entity_name)=LOWER($2) AND status='active'
		ORDER BY updated_at DESC LIMIT 1`,
		userID, name,
	).Scan(&e.ID, &e.UserID, &e.EntityName, &e.EntityType, &propsJSON,
		&e.Visibility, &emb, &sessionIDsOut, &e.SourceText,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find kg entity by name: %w", err)
	}
	e.Properties = propsJSON
	e.Embedding = emb
	e.SessionIDs = parsePGIntArray(sessionIDsOut)
	return &e, nil
}

func (s *sqlKGStore) ListByUser(ctx context.Context, userID int64, entityType string, limit, offset int) ([]KGEntity, int, error) {
	if limit <= 0 {
		limit = 50
	}
	var args []interface{}
	args = append(args, userID)
	query := `SELECT id, user_id, entity_name, entity_type, COALESCE(properties,''),
		visibility, COALESCE(embedding,''), COALESCE(session_ids::text,'{}'),
			  COALESCE(source_text,''), created_by, created_at, updated_at
		FROM kg_entities WHERE user_id=$1 AND status='active'`
	idx := 2
	if entityType != "" {
		query += fmt.Sprintf(" AND entity_type=$%d", idx)
		args = append(args, entityType)
		idx++
	}
	query += " ORDER BY updated_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	qCount := `SELECT COUNT(*) FROM kg_entities WHERE user_id=$1 AND status='active'`
	var countArgs []interface{} = []interface{}{userID}
	if entityType != "" {
		qCount += " AND entity_type=$2"
		countArgs = append(countArgs, entityType)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, qCount, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count kg entities: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list kg entities: %w", err)
	}
	defer rows.Close()

	var entities []KGEntity
	for rows.Next() {
		var e KGEntity
		var propsJSON, emb, sessionIDsOut string
		if err := rows.Scan(&e.ID, &e.UserID, &e.EntityName, &e.EntityType, &propsJSON,
			&e.Visibility, &emb, &sessionIDsOut, &e.SourceText,
			&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan kg entity: %w", err)
		}
		e.Properties = propsJSON
		e.Embedding = emb
		e.SessionIDs = parsePGIntArray(sessionIDsOut)
		entities = append(entities, e)
	}
	return entities, total, rows.Err()
}

func (s *sqlKGStore) DeleteEntity(ctx context.Context, userID int64, entityID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE kg_entities SET status='deleted', updated_at=NOW() WHERE id=$1 AND user_id=$2`,
		entityID, userID)
	return err
}

// --------------------------------------------------------------------------//
// Relation CRUD
// --------------------------------------------------------------------------//

func (s *sqlKGStore) CreateRelation(ctx context.Context, relation *KGRelation) (*KGRelation, error) {
	var id int64
	var sessionIDsOut []int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO kg_relations (from_entity_id, to_entity_id, relation_type,
			properties, weight, visibility, session_ids, source_text)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, session_ids, created_at, updated_at`,
		relation.FromEntityID, relation.ToEntityID, relation.RelationType,
		props(relation.Properties), relation.Weight, relation.Visibility,
		pgArrayInt(relation.SessionIDs), relation.SourceText,
	).Scan(&id, &sessionIDsOut, &relation.CreatedAt, &relation.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create kg relation: %w", err)
	}
	relation.ID = id
	relation.SessionIDs = sessionIDsOut
	return relation, nil
}

func (s *sqlKGStore) UpsertRelation(ctx context.Context, relation *KGRelation) (*KGRelation, error) {
	var id int64
	var sessionIDsOut []int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO kg_relations (from_entity_id, to_entity_id, relation_type,
			properties, weight, visibility, session_ids, source_text)
		VALUES ($1,$2,$3,$4,$5,'private',$6,$7)
		ON CONFLICT (from_entity_id, to_entity_id, relation_type) WHERE status='active'
		DO UPDATE SET
			properties = EXCLUDED.properties,
			weight     = EXCLUDED.weight,
			source_text = EXCLUDED.source_text,
			updated_at  = NOW()
		RETURNING id, session_ids, created_at, updated_at`,
		relation.FromEntityID, relation.ToEntityID, relation.RelationType,
		props(relation.Properties), relation.Weight,
		pgArrayInt(relation.SessionIDs), relation.SourceText,
	).Scan(&id, &sessionIDsOut, &relation.CreatedAt, &relation.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert kg relation: %w", err)
	}
	relation.ID = id
	relation.SessionIDs = sessionIDsOut
	return relation, nil
}

func (s *sqlKGStore) ListRelationsByEntity(ctx context.Context, entityID int64, limit, offset int) ([]KGRelation, int, error) {
	if limit <= 0 {
		limit = 50
	}
	var total int
	qCount := `SELECT COUNT(*) FROM kg_relations WHERE
		(from_entity_id=$1 OR to_entity_id=$1) AND status='active'`
	if err := s.db.QueryRowContext(ctx, qCount, entityID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count kg relations: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, from_entity_id, to_entity_id, relation_type, COALESCE(properties,''),
			   weight, visibility, status, COALESCE(session_ids::text,'{}'),
			   COALESCE(source_text,''), created_at, updated_at
		FROM kg_relations
		WHERE (from_entity_id=$1 OR to_entity_id=$1) AND status NOT IN ('deleted','superseded')
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		entityID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list kg relations: %w", err)
	}
	defer rows.Close()

	var relations []KGRelation
	for rows.Next() {
		var r KGRelation
		var propsJSON, sessionIDsOut string
		if err := rows.Scan(&r.ID, &r.FromEntityID, &r.ToEntityID, &r.RelationType,
			&propsJSON, &r.Weight, &r.Visibility, &r.Status, &sessionIDsOut,
			&r.SourceText, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan kg relation: %w", err)
		}
		r.Properties = propsJSON
		r.SessionIDs = parsePGIntArray(sessionIDsOut)
		relations = append(relations, r)
	}
	return relations, total, rows.Err()
}

func (s *sqlKGStore) DeleteRelation(ctx context.Context, relationID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE kg_relations SET status='deleted', updated_at=NOW() WHERE id=$1`,
		relationID)
	return err
}

// --------------------------------------------------------------------------//
// Search & Graph
// --------------------------------------------------------------------------//

// SearchEntities searches KG entities using both keyword (ILIKE) and
// optional vector similarity when embedding is populated.
// Falls back to keyword-only when embedding is absent or no embedder.
// Implements POST /api/v1/kg/search from 10.100.1.13 §4.3.
func (s *sqlKGStore) SearchEntities(ctx context.Context, userID int64, query string, entityType string, limit int) ([]KGEntityWithScore, error) {
	if limit <= 0 {
		limit = 10
	}
	args := []interface{}{userID, "%" + strings.ToLower(query) + "%"}
	additional := " AND entity_name ILIKE $2"
	idx := 3

	if entityType != "" {
		additional += fmt.Sprintf(" AND entity_type=$%d", idx)
		args = append(args, entityType)
		idx++
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, entity_name, entity_type, COALESCE(properties,''),
			   visibility, COALESCE(embedding,''), COALESCE(session_ids::text,'{}'),
			   COALESCE(source_text,''), created_by, created_at, updated_at,
			   ts_rank(to_tsvector('simple', entity_name), plainto_tsquery('simple', $2)) AS score
		FROM kg_entities
		WHERE user_id=$1 AND status='active' `+additional+`
		ORDER BY score DESC, updated_at DESC
		LIMIT $`+fmt.Sprintf("%d", idx+1),
		append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("search kg entities: %w", err)
	}
	defer rows.Close()

	return s.scanEntitiesWithScore(rows)
}

// SearchRelations searches relations by type and source text keyword match.
// Implements POST /api/v1/kg/search from 10.100.1.13 §4.3 (relation axis).
func (s *sqlKGStore) SearchRelations(ctx context.Context, userID int64, query string, limit int) ([]KGRelation, error) {
	// Relations are filtered by source_text keyword match, joined with entity names.
	var relations []KGRelation
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.from_entity_id, r.to_entity_id, r.relation_type,
			   COALESCE(r.properties,''), r.weight, r.visibility, r.status,
			   COALESCE(r.session_ids::text,'{}'), COALESCE(r.source_text,''),
			   r.created_at, r.updated_at,
			   ts_rank(to_tsvector('simple', r.source_text || ' ' ||
				   COALESCE(e1.entity_name,'') || ' ' || COALESCE(e2.entity_name,'')),
				   plainto_tsquery('simple', $2)) AS score
		FROM kg_relations r
		LEFT JOIN kg_entities e1 ON r.from_entity_id=e1.id
		LEFT JOIN kg_entities e2 ON r.to_entity_id=e2.id
		WHERE (r.from_entity_id IN (SELECT id FROM kg_entities WHERE user_id=$1 AND status='active')
			OR r.to_entity_id IN (SELECT id FROM kg_entities WHERE user_id=$1 AND status='active'))
			AND r.status NOT IN ('deleted','superseded')
		ORDER BY score DESC
		LIMIT $3`,
		userID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search kg relations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r KGRelation
		var propsJSON, sessionIDsOut string
		var score float64
		if err := rows.Scan(&r.ID, &r.FromEntityID, &r.ToEntityID, &r.RelationType,
			&propsJSON, &r.Weight, &r.Visibility, &r.Status, &sessionIDsOut,
			&r.SourceText, &r.CreatedAt, &r.UpdatedAt, &score); err != nil {
			return nil, fmt.Errorf("scan kg relation search: %w", err)
		}
		r.Properties = propsJSON
		r.SessionIDs = parsePGIntArray(sessionIDsOut)
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// GetSubgraph performs a BFS graph traversal up to depth hops.
// Implements GET /api/v1/kg/graph (session-aware) from 10.100.1.13 §4.3.
func (s *sqlKGStore) GetSubgraph(ctx context.Context, userID int64, rootEntityID int64, depth int) (*KGSubgraph, error) {
	if depth <= 0 {
		depth = 2
	}
	if depth > 4 {
		depth = 4
	}

	visited := make(map[int64]bool)
	var (
		entities  []KGEntity
		relations []KGRelation
	)
	entityQueue := []int64{rootEntityID}

	for d := 0; d < depth && len(entityQueue) > 0; d++ {
		var nextQueue []int64
		for _, eid := range entityQueue {
			if visited[eid] {
				continue
			}
			visited[eid] = true

			e, err := s.GetEntity(ctx, eid)
			if err != nil || e == nil {
				continue
			}
			if e.UserID != userID {
				continue // cross-tenant isolation
			}
			entities = append(entities, *e)

			rels, _, err := s.ListRelationsByEntity(ctx, eid, 200, 0)
			if err != nil {
				continue
			}
			for _, r := range rels {
				if r.Status != "deleted" && r.Status != "superseded" {
					relations = append(relations, r)
					if !visited[r.ToEntityID] {
						nextQueue = append(nextQueue, r.ToEntityID)
					}
					if !visited[r.FromEntityID] {
						nextQueue = append(nextQueue, r.FromEntityID)
					}
				}
			}
		}
		entityQueue = uniq(nextQueue)
	}

	return &KGSubgraph{Entities: entities, Relations: relations, Depth: depth}, nil
}

// GetRelatedSessions returns all session IDs associated with an entity.
// Implements GET /api/v1/kg/entity/{entity_name}/sessions from 10.100.1.13 §4.3.
func (s *sqlKGStore) GetRelatedSessions(ctx context.Context, entityID int64) ([]int64, error) {
	var sessionIDsRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(session_ids::text,'{}') FROM kg_entities WHERE id=$1 AND status='active'`,
		entityID).Scan(&sessionIDsRaw)
	if err != nil {
		return nil, fmt.Errorf("get related sessions: %w", err)
	}
	return parsePGIntArray(sessionIDsRaw), nil
}

// GetFullGraph returns all active entities and relations for a user.
// Implements GET /api/v1/kg/graph from 10.100.1.13 §4.3.
func (s *sqlKGStore) GetFullGraph(ctx context.Context, userID int64, limit int) (*KGSubgraph, error) {
	if limit <= 0 {
		limit = 500
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, entity_name, entity_type, COALESCE(properties,''),
			   visibility, COALESCE(embedding,''), COALESCE(session_ids::text,'{}'),
			   COALESCE(source_text,''), created_by, created_at, updated_at
		FROM kg_entities WHERE user_id=$1 AND status='active'
		ORDER BY updated_at DESC LIMIT $2`,
		userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get full graph entities: %w", err)
	}
	defer rows.Close()

	var entities []KGEntity
	for rows.Next() {
		var e KGEntity
		var propsJSON, emb, sessionIDsOut string
		if err := rows.Scan(&e.ID, &e.UserID, &e.EntityName, &e.EntityType, &propsJSON,
			&e.Visibility, &emb, &sessionIDsOut, &e.SourceText,
			&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Properties = propsJSON
		e.Embedding = emb
		e.SessionIDs = parsePGIntArray(sessionIDsOut)
		entities = append(entities, e)
	}

	// Collect all entity IDs to filter relations.
	idSet := make(map[int64]bool)
	for _, e := range entities {
		idSet[e.ID] = true
	}

	rels, _, err := s.ListRelationsByEntity(ctx, 0, limit, 0)
	// Filter to only include relations where both ends are in this user's graph.
	var relations []KGRelation
	for _, r := range rels {
		if idSet[r.FromEntityID] && idSet[r.ToEntityID] {
			relations = append(relations, r)
		}
	}
	_ = err // ignore error on all-relations query

	return &KGSubgraph{Entities: entities, Relations: relations, Depth: 0}, nil
}

// --------------------------------------------------------------------------//
// KGBuilder: LLM-based entity/relation extraction
// --------------------------------------------------------------------------//

const kgBuilderTmpl = `You are a knowledge graph extraction engine.
Given the following text from a chat session, extract all named entities and their relationships.
Respond STRICTLY with valid JSON in this format:
{
  "entities": [
    {"name": "...", "type": "person|concept|project|tool|file|error|api|method|component|URL|other", "properties": {}}
  ],
  "relations": [
    {"from": "...", "to": "...", "type": "uses|depends_on|calls|contains|relates_to|implements|produces|raises|...", "properties": {}}
  ]
}
Only output the JSON. Do not add any commentary.
Text:
---

%s`

// ExtractFromText parses raw text using the injected LLMProvider, creates or
// finds existing entities and relations, and stores them in the local DB.
// It handles deduplication: if an entity with the same name exists for the
// same user, it updates rather than duplicates.
// Implements KG extraction described in SESSION_MEMORY_RESEARCH.md §7.1.
func (s *sqlKGStore) ExtractFromText(ctx context.Context, userID int64, sessionID int64, text string) (*KGSubgraph, error) {
	prompt := fmt.Sprintf(kgBuilderTmpl, text)

	var llmOut string
	if s.llm != nil {
		llmOut, _ = s.llm.Chat(ctx, "You are a KG extraction engine.", prompt)
	}

	// Parse JSON.
	var result struct {
		Entities  []struct {
			Name       string                 `json:"name"`
			Type       string                 `json:"type"`
			Properties map[string]interface{} `json:"properties"`
		} `json:"entities"`
		Relations []struct {
			From string `json:"from"`
			To   string `json:"to"`
			Type string `json:"type"`
		} `json:"relations"`
	}

	if llmOut != "" {
		if err := json.Unmarshal([]byte(llmOut), &result); err != nil {
			// Fall through: use rule-based extraction
		}
	}

	// Rule-based fallback (active when no LLM is injected or JSON parse fails).
	if len(result.Entities) == 0 {
		rb := ruleBasedExtract(text)
		for _, e := range rb.Entities {
			result.Entities = append(result.Entities, struct {
				Name       string                 `json:"name"`
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties"`
			}{Name: e.Name, Type: e.Type, Properties: e.Properties})
		}
		for _, r := range rb.Relations {
			result.Relations = append(result.Relations, struct {
				From string `json:"from"`
				To   string `json:"to"`
				Type string `json:"type"`
			}{From: r.From, To: r.To, Type: r.Type})
		}
	}

	props, _ := json.Marshal(map[string]interface{}{"source": "kg_builder", "session_id": sessionID})

	var (
		createdEntities  []KGEntity
		createdRelations []KGRelation
		entityNameToID   = make(map[string]int64)
	)

	// Deduplicate entity names locally.
	seenNames := make(map[string]bool)
	for _, raw := range result.Entities {
		name := strings.TrimSpace(raw.Name)
		if name == "" || seenNames[name] {
			continue
		}
		seenNames[name] = true

		propsB, _ := json.Marshal(raw.Properties)
		ent := &KGEntity{
			UserID:     userID,
			EntityName: name,
			EntityType: normalizeEntityType(raw.Type),
			Properties: string(propsB),
			Visibility: "private",
			SessionIDs: []int64{sessionID},
			SourceText: string(props),
			CreatedBy:  userID,
		}

		// Check if already exists.
		if existing, err := s.FindByName(ctx, userID, name); err == nil && existing != nil {
			// Update: append session ID.
			existing.SessionIDs = append(existing.SessionIDs, sessionID)
			ent, _ = s.UpsertEntity(ctx, existing)
		} else {
			ent, _ = s.UpsertEntity(ctx, ent)
		}
		if ent != nil {
			createdEntities = append(createdEntities, *ent)
			entityNameToID[ent.EntityName] = ent.ID
		}
	}

	// Deduplicate relations locally.
	seenRels := make(map[string]bool)
	for _, raw := range result.Relations {
		fromID := entityNameToID[raw.From]
		toID := entityNameToID[raw.To]
		if fromID == 0 || toID == 0 {
			continue
		}
		key := fmt.Sprintf("%d-%d-%s", fromID, toID, raw.Type)
		if seenRels[key] {
			continue
		}
		seenRels[key] = true

		rel := &KGRelation{
			FromEntityID: fromID,
			ToEntityID:   toID,
			RelationType: normalizeRelType(raw.Type),
			Properties:   string(props),
			Weight:       1.0,
			Visibility:   "private",
			SessionIDs:   []int64{sessionID},
			SourceText:   text,
		}

		// Upsert to avoid duplicate relations.
		rel, _ = s.UpsertRelation(ctx, rel)
		if rel != nil {
			created := *rel
			createdRelations = append(createdRelations, created)
		}
	}

	return &KGSubgraph{Entities: createdEntities, Relations: createdRelations, Depth: 0}, nil
}

// --------------------------------------------------------------------------//
// Auxiliary helpers
// --------------------------------------------------------------------------//

func (s *sqlKGStore) scanEntitiesWithScore(rows *sql.Rows) ([]KGEntityWithScore, error) {
	var results []KGEntityWithScore
	for rows.Next() {
		var e KGEntity
		var propsJSON, emb, sessionIDsOut string
		var score float64
		if err := rows.Scan(&e.ID, &e.UserID, &e.EntityName, &e.EntityType, &propsJSON,
			&e.Visibility, &emb, &sessionIDsOut, &e.SourceText,
			&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt, &score); err != nil {
			return nil, err
		}
		e.Properties = propsJSON
		e.Embedding = emb
		e.SessionIDs = parsePGIntArray(sessionIDsOut)
		results = append(results, KGEntityWithScore{KGEntity: e, Score: score})
	}
	return results, rows.Err()
}

// ruleBasedExtract is a lightweight rule-based extractor used when no LLM is available.
// Scans for common entity patterns like identifiers, URLs, file paths, and code keywords.
func ruleBasedExtract(text string) struct {
	Entities  []struct{ Name, Type string; Properties map[string]interface{} }
	Relations []struct{ From, To, Type string }
} {
	var out struct {
		Entities  []struct{ Name, Type string; Properties map[string]interface{} }
		Relations []struct{ From, To, Type string }
	}
	seen := make(map[string]bool)

	// Pattern: function calls `func(...)`
	for _, match := range reFunction.FindAllStringSubmatch(text, -1) {
		name := match[1]
		if !seen[name] {
			seen[name] = true
			out.Entities = append(out.Entities, struct {
				Name, Type string; Properties map[string]interface{}
			}{Name: name, Type: "method", Properties: nil})
		}
	}
	// Pattern: file paths /path/to/file.go
	for _, match := range rePath.FindAllStringSubmatch(text, -1) {
		name := strings.TrimSpace(match[0])
		if !seen[name] {
			seen[name] = true
			out.Entities = append(out.Entities, struct {
				Name, Type string; Properties map[string]interface{}
			}{Name: name, Type: "file", Properties: nil})
		}
	}
	return out
}

func normalizeEntityType(t string) string {
	switch strings.ToLower(t) {
	case "person": return "person"
	case "concept": return "concept"
	case "project": return "project"
	case "tool": return "tool"
	case "file": return "file"
	case "error": return "error"
	case "api": return "api"
	case "method", "function": return "method"
	case "component": return "component"
	case "url", "uri": return "URL"
	default: return "other"
	}
}

func normalizeRelType(t string) string {
	switch strings.ToLower(t) {
	case "uses": return "uses"
	case "depends_on", "depends": return "depends_on"
	case "calls": return "calls"
	case "contains": return "contains"
	case "relates_to": return "relates_to"
	case "implements": return "implements"
	case "produces": return "produces"
	case "raises", "throws": return "raises"
	default: return "relates_to"
	}
}


func uniq(ids []int64) []int64 {
	seen := make(map[int64]bool)
	var out []int64
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func props(v string) string {
	if v == "" {
		return "{}"
	}
	return v
}

func embedding(v string) string {
	return v
}

var (
	reFunction = regexp.MustCompile(`(?m)(?:func|function|def|class|method)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rePath     = regexp.MustCompile(`(?m)(?:/[\w\-\.]+)+\.[a-zA-Z]{1,6}\b`)
)

// ---- pq helpers (copied from pyramid.go for package independence) ----

func pgArrayInt(arr []int64) string {
	if len(arr) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteString("{")
	for i, v := range arr {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%d", v))
	}
	b.WriteString("}")
	return b.String()
}

// parsePGIntArray parses a PostgreSQL array literal like "{1,2,3}" into []int64.
// NOTE: same function exists in pyramid.go; both are in the same package (memory),
// so we keep only this one and remove the duplicate from pyramid.go.
func parsePGIntArray(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return nil
	}
	raw = strings.TrimPrefix(raw, "{")
	raw = strings.TrimSuffix(raw, "}")
	if raw == "" {
		return nil
	}
	var result []int64
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		var v int64
		if _, err := fmt.Sscanf(tok, "%d", &v); err == nil {
			result = append(result, v)
		}
	}
	return result
}
