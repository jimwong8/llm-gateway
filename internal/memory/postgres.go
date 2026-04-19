package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"llm-gateway/gateway/internal/cache"
	"llm-gateway/gateway/internal/providers"
)

type Item struct {
	Content string
	Role    string
}

type Conversation struct {
	ID        int64
	TenantID  string
	UserID    string
	SessionID string
	Status    string
	LastSeq   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Message struct {
	Seq        int64
	Role       string
	Content    string
	TokenCount *int
	CreatedAt  time.Time
}

type SessionSummary struct {
	TenantID         string
	UserID           string
	SessionID        string
	CurrentGoal      string
	CompletedItems   []string
	OpenItems        []string
	KeyDecisions     []string
	Blockers         []string
	SourceMessageSeq int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type UserPreference struct {
	TenantID   string
	UserID     string
	Key        string
	Value      string
	SourceText string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ProjectFact struct {
	ID               int64
	TenantID         string
	UserID           string
	Key              string
	Value            string
	SourceText       string
	Status           string
	SupersededBy     *int64
	SourceMessageSeq int64
	LastVerifiedAt   time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type SearchResult struct {
	ConversationID int64
	SessionID      string
	MessageID      int64
	Seq            int64
	Snippet        string
}

type messageToAppend struct {
	Role       string
	Content    string
	TokenCount *int
}

type conversationCache interface {
	CacheConversationMeta(ctx context.Context, conversationID string, meta cache.ConversationMeta) error
	GetConversationMeta(ctx context.Context, conversationID string) (*cache.ConversationMeta, bool, error)
	CacheRecentMessages(ctx context.Context, conversationID string, messages []cache.RecentMessage, maxItems int64) error
	GetRecentMessages(ctx context.Context, conversationID string, limit int64) ([]cache.RecentMessage, error)
	InvalidateConversationCache(ctx context.Context, conversationID string) error
}

type Store struct {
	db    *sql.DB
	cache conversationCache
}

func NewStore(dsn string, rc *cache.RedisCache) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, cache: rc}
	if err := s.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS session_memories (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_session_memories_tenant_user_session_created_at ON session_memories (tenant_id, user_id, session_id, created_at DESC);

CREATE TABLE IF NOT EXISTS conversations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    session_id TEXT NOT NULL,
    status TEXT,
    last_seq BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_session_id ON conversations (session_id);
CREATE INDEX IF NOT EXISTS idx_conversations_tenant_user_session ON conversations (tenant_id, user_id, session_id);

CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    conversation_id BIGINT REFERENCES conversations(id),
    seq BIGINT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
    token_count INTEGER,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_messages_session_seq UNIQUE (session_id, seq)
);
ALTER TABLE messages ADD COLUMN IF NOT EXISTS search_vector tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_messages_conversation_seq ON messages (conversation_id, seq);
CREATE INDEX IF NOT EXISTS idx_messages_session_created_at ON messages (session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_search_vector ON messages USING GIN (search_vector);

CREATE TABLE IF NOT EXISTS session_summaries (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    session_id TEXT NOT NULL,
    current_goal TEXT NOT NULL DEFAULT '',
    completed_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    open_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    key_decisions JSONB NOT NULL DEFAULT '[]'::jsonb,
    blockers JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_message_seq BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_session_summaries_session_id ON session_summaries (session_id);
CREATE INDEX IF NOT EXISTS idx_session_summaries_tenant_user ON session_summaries (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_session_summaries_updated_at ON session_summaries (updated_at DESC);

CREATE TABLE IF NOT EXISTS user_preferences (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    preference_key TEXT NOT NULL,
    preference_value TEXT NOT NULL,
    source_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_preferences_tenant_user_key ON user_preferences (COALESCE(tenant_id, ''), COALESCE(user_id, ''), preference_key);
CREATE INDEX IF NOT EXISTS idx_user_preferences_tenant_user_updated_at ON user_preferences (tenant_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS project_facts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    fact_key TEXT NOT NULL,
    fact_value TEXT NOT NULL,
    source_text TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    superseded_by BIGINT REFERENCES project_facts(id),
    source_message_seq BIGINT NOT NULL DEFAULT 0,
    last_verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE project_facts ADD COLUMN IF NOT EXISTS status TEXT;
ALTER TABLE project_facts ALTER COLUMN status SET DEFAULT 'active';
UPDATE project_facts SET status = 'active' WHERE status IS NULL OR TRIM(status) = '';
ALTER TABLE project_facts ALTER COLUMN status SET NOT NULL;
ALTER TABLE project_facts ADD COLUMN IF NOT EXISTS superseded_by BIGINT REFERENCES project_facts(id);
ALTER TABLE project_facts ADD COLUMN IF NOT EXISTS source_message_seq BIGINT;
ALTER TABLE project_facts ALTER COLUMN source_message_seq SET DEFAULT 0;
UPDATE project_facts SET source_message_seq = 0 WHERE source_message_seq IS NULL;
ALTER TABLE project_facts ALTER COLUMN source_message_seq SET NOT NULL;
ALTER TABLE project_facts ADD COLUMN IF NOT EXISTS last_verified_at TIMESTAMPTZ;
ALTER TABLE project_facts ALTER COLUMN last_verified_at SET DEFAULT NOW();
UPDATE project_facts SET last_verified_at = COALESCE(last_verified_at, updated_at, created_at, NOW());
ALTER TABLE project_facts ALTER COLUMN last_verified_at SET NOT NULL;
DROP INDEX IF EXISTS idx_project_facts_tenant_user_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_project_facts_active_tenant_user_key ON project_facts (COALESCE(tenant_id, ''), COALESCE(user_id, ''), fact_key) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_project_facts_tenant_user_key_status ON project_facts (tenant_id, user_id, fact_key, status);
`)


	return err
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) AppendMessage(ctx context.Context, tenantID, userID, sessionID, role, content string, tokenCount *int) error {
	return s.appendMessagesTx(ctx, tenantID, userID, sessionID, []messageToAppend{{
		Role:       role,
		Content:    content,
		TokenCount: tokenCount,
	}})
}

func (s *Store) AppendFromRequest(ctx context.Context, req providers.ChatCompletionRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil
	}
	msgs := make([]messageToAppend, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}
		msgs = append(msgs, messageToAppend{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return s.appendMessagesTx(ctx, req.TenantID, req.UserID, req.SessionID, msgs)
}

func (s *Store) appendMessagesTx(ctx context.Context, tenantID, userID, sessionID string, messages []messageToAppend) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	normalized := make([]messageToAppend, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		normalized = append(normalized, messageToAppend{
			Role:       msg.Role,
			Content:    content,
			TokenCount: msg.TokenCount,
		})
	}
	if len(normalized) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	batchSize := int64(len(normalized))
	var conversationID int64
	var lastSeq int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO conversations (tenant_id, user_id, session_id, status, last_seq, created_at, updated_at)
VALUES ($1, $2, $3, 'active', $4, NOW(), NOW())
ON CONFLICT (session_id) DO UPDATE SET
	tenant_id = COALESCE(EXCLUDED.tenant_id, conversations.tenant_id),
	user_id = COALESCE(EXCLUDED.user_id, conversations.user_id),
	last_seq = conversations.last_seq + EXCLUDED.last_seq,
	updated_at = NOW()
RETURNING id, last_seq
`, tenantID, userID, sessionID, batchSize).Scan(&conversationID, &lastSeq); err != nil {
		return err
	}

	startSeq := lastSeq - batchSize + 1
	for i, msg := range normalized {
		seq := startSeq + int64(i)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO messages (session_id, conversation_id, seq, role, content, token_count)
VALUES ($1, $2, $3, $4, $5, $6)
`, sessionID, conversationID, seq, msg.Role, msg.Content, msg.TokenCount); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO session_memories (tenant_id, user_id, session_id, role, content)
VALUES ($1, $2, $3, $4, $5)
`, tenantID, userID, sessionID, msg.Role, trim(msg.Content)); err != nil {
			return err
		}
	}

	if err := s.insertBusinessAuditInTx(ctx, tx, tenantID, userID, sessionID, len(normalized)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.updateConversationCacheAsync(ctx, conversationID, lastSeq, normalized, startSeq)
	return nil
}

func (s *Store) updateConversationCacheAsync(ctx context.Context, conversationID int64, lastSeq int64, normalized []messageToAppend, startSeq int64) {
	if s.cache == nil {
		return
	}
	if len(normalized) == 0 {
		return
	}

	recent := make([]cache.RecentMessage, 0, len(normalized))
	for i, msg := range normalized {
		recent = append(recent, cache.RecentMessage{
			Seq:     startSeq + int64(i),
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	conversationKey := strconv.FormatInt(conversationID, 10)

	go func() {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		if err := s.cache.CacheConversationMeta(cacheCtx, conversationKey, cache.ConversationMeta{
			LastSeq:   lastSeq,
			UpdatedAt: time.Now().UTC(),
		}); err != nil {
			log.Printf("memory cache meta update failed conversation_id=%s: %v", conversationKey, err)
		}
		if err := s.cache.CacheRecentMessages(cacheCtx, conversationKey, recent, 50); err != nil {
			log.Printf("memory cache recent update failed conversation_id=%s: %v", conversationKey, err)
		}
	}()
}

func (s *Store) insertBusinessAuditInTx(ctx context.Context, tx *sql.Tx, tenantID, actorID, sessionID string, messageCount int) error {
	var tableExists bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('public.business_audit_logs') IS NOT NULL`).Scan(&tableExists); err != nil {
		return err
	}
	if !tableExists {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO business_audit_logs (tenant_id, action, target_type, target_id, actor_id)
VALUES ($1, $2, $3, $4, $5)
`, tenantID, fmt.Sprintf("append_messages:%d", messageCount), "conversation", sessionID, actorID)
	return err
}

func (s *Store) insertBusinessAuditActionInTx(ctx context.Context, tx *sql.Tx, tenantID, action, targetType, targetID, actorID string) error {
	var tableExists bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('public.business_audit_logs') IS NOT NULL`).Scan(&tableExists); err != nil {
		return err
	}
	if !tableExists {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO business_audit_logs (tenant_id, action, target_type, target_id, actor_id)
VALUES ($1, $2, $3, $4, $5)
`, tenantID, action, targetType, targetID, actorID)
	return err
}

func (s *Store) Recent(ctx context.Context, tenantID, userID, sessionID string, limit int) ([]Item, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 3
	}

	conversation, err := s.GetConversation(ctx, tenantID, userID, sessionID)
	if err != nil {
		return nil, err
	}

	cacheMiss := false
	if s.cache != nil && conversation != nil {
		recentMsgs, cacheErr := s.cache.GetRecentMessages(ctx, strconv.FormatInt(conversation.ID, 10), int64(limit))
		if cacheErr != nil {
			log.Printf("memory cache recent read failed conversation_id=%d: %v", conversation.ID, cacheErr)
			cacheMiss = true
		} else if len(recentMsgs) > 0 {
			out := make([]Item, 0, len(recentMsgs))
			for i := len(recentMsgs) - 1; i >= 0; i-- {
				out = append(out, Item{Role: recentMsgs[i].Role, Content: recentMsgs[i].Content})
			}
			return out, nil
		} else {
			cacheMiss = true
		}
	}

	rows, err := s.db.QueryContext(ctx, `SELECT seq, role, content FROM messages WHERE session_id = $1 AND deleted_at IS NULL ORDER BY seq DESC LIMIT $2`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Item
	pgRecent := make([]cache.RecentMessage, 0, limit)
	for rows.Next() {
		var seq int64
		var item Item
		if err := rows.Scan(&seq, &item.Role, &item.Content); err != nil {
			return nil, err
		}
		out = append(out, item)
		pgRecent = append(pgRecent, cache.RecentMessage{
			Seq:     seq,
			Role:    item.Role,
			Content: item.Content,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) > 0 {
		if cacheMiss && s.cache != nil && conversation != nil {
			s.refillRecentMessagesCacheAsync(ctx, conversation.ID, pgRecent)
		}
		return out, nil
	}

	fallbackRows, err := s.db.QueryContext(ctx, `SELECT role, content FROM session_memories WHERE COALESCE(tenant_id, '') = COALESCE($1, '') AND COALESCE(user_id, '') = COALESCE($2, '') AND session_id = $3 ORDER BY created_at DESC LIMIT $4`, tenantID, userID, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer fallbackRows.Close()
	for fallbackRows.Next() {
		var item Item
		if err := fallbackRows.Scan(&item.Role, &item.Content); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, fallbackRows.Err()
}

func (s *Store) GetConversation(ctx context.Context, tenantID, userID, sessionID string) (*Conversation, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	var c Conversation
	var tenant sql.NullString
	var user sql.NullString
	var status sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT id, tenant_id, user_id, session_id, status, last_seq, created_at, updated_at
FROM conversations
WHERE COALESCE(tenant_id, '') = COALESCE($1, '')
  AND COALESCE(user_id, '') = COALESCE($2, '')
  AND session_id = $3
  AND COALESCE(status, 'active') <> 'deleted'
`, tenantID, userID, sessionID).Scan(
		&c.ID,
		&tenant,
		&user,
		&c.SessionID,
		&status,
		&c.LastSeq,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if tenant.Valid {
		c.TenantID = tenant.String
	}
	if user.Valid {
		c.UserID = user.String
	}
	if status.Valid {
		c.Status = status.String
	}

	conversationKey := strconv.FormatInt(c.ID, 10)
	cacheMiss := false
	if s.cache != nil {
		meta, hit, cacheErr := s.cache.GetConversationMeta(ctx, conversationKey)
		if cacheErr != nil {
			log.Printf("memory cache meta read failed conversation_id=%s: %v", conversationKey, cacheErr)
			cacheMiss = true
		} else if hit {
			c.LastSeq = meta.LastSeq
			if !meta.UpdatedAt.IsZero() {
				c.UpdatedAt = meta.UpdatedAt
			}
		} else {
			cacheMiss = true
		}
	}

	if cacheMiss {
		s.refillConversationMetaCacheAsync(ctx, c.ID, c.LastSeq, c.UpdatedAt)
	}
	return &c, nil
}

func (s *Store) refillConversationMetaCacheAsync(ctx context.Context, conversationID int64, lastSeq int64, updatedAt time.Time) {
	if s.cache == nil {
		return
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	conversationKey := strconv.FormatInt(conversationID, 10)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if err := s.cache.CacheConversationMeta(cacheCtx, conversationKey, cache.ConversationMeta{
			LastSeq:   lastSeq,
			UpdatedAt: updatedAt.UTC(),
		}); err != nil {
			log.Printf("memory cache meta refill failed conversation_id=%s: %v", conversationKey, err)
		}
	}()
}

func (s *Store) refillRecentMessagesCacheAsync(ctx context.Context, conversationID int64, recent []cache.RecentMessage) {
	if s.cache == nil {
		return
	}
	if len(recent) == 0 {
		return
	}
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}
	conversationKey := strconv.FormatInt(conversationID, 10)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if err := s.cache.CacheRecentMessages(cacheCtx, conversationKey, recent, 50); err != nil {
			log.Printf("memory cache recent refill failed conversation_id=%s: %v", conversationKey, err)
		}
	}()
}

func (s *Store) GetMessages(ctx context.Context, sessionID string, cursorSeq int64, limit int, direction string) ([]Message, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	dir := strings.ToLower(strings.TrimSpace(direction))
	if dir != "backward" {
		dir = "forward"
	}

	query := `SELECT seq, role, content, token_count, created_at FROM messages WHERE session_id = $1 AND deleted_at IS NULL`
	args := []any{sessionID}
	argPos := 2

	if cursorSeq > 0 {
		if dir == "backward" {
			query += fmt.Sprintf(" AND seq < $%d", argPos)
		} else {
			query += fmt.Sprintf(" AND seq > $%d", argPos)
		}
		args = append(args, cursorSeq)
		argPos++
	}

	if dir == "backward" {
		query += " ORDER BY seq DESC"
	} else {
		query += " ORDER BY seq ASC"
	}
	query += fmt.Sprintf(" LIMIT $%d", argPos)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Message, 0, limit)
	for rows.Next() {
		var msg Message
		var token sql.NullInt64
		if err := rows.Scan(&msg.Seq, &msg.Role, &msg.Content, &token, &msg.CreatedAt); err != nil {
			return nil, err
		}
		if token.Valid {
			t := int(token.Int64)
			msg.TokenCount = &t
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if dir == "backward" {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}

	return out, nil
}

func (s *Store) GetMessagesAroundAnchor(ctx context.Context, sessionID string, anchorSeq int64, limit int) ([]Message, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	if anchorSeq <= 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	beforeLimit := limit / 2
	afterLimit := limit - beforeLimit - 1
	if afterLimit < 0 {
		afterLimit = 0
	}

	rows, err := s.db.QueryContext(ctx, `
WITH before_msgs AS (
	SELECT seq, role, content, token_count, created_at
	FROM messages
	WHERE session_id = $1 AND seq < $2 AND deleted_at IS NULL
	ORDER BY seq DESC
	LIMIT $3
),
anchor_msg AS (
	SELECT seq, role, content, token_count, created_at
	FROM messages
	WHERE session_id = $1 AND seq = $2 AND deleted_at IS NULL
),
after_msgs AS (
	SELECT seq, role, content, token_count, created_at
	FROM messages
	WHERE session_id = $1 AND seq > $2 AND deleted_at IS NULL
	ORDER BY seq ASC
	LIMIT $4
)
SELECT seq, role, content, token_count, created_at
FROM (
	SELECT seq, role, content, token_count, created_at FROM before_msgs
	UNION ALL
	SELECT seq, role, content, token_count, created_at FROM anchor_msg
	UNION ALL
	SELECT seq, role, content, token_count, created_at FROM after_msgs
) around
ORDER BY seq ASC
`, sessionID, anchorSeq, beforeLimit, afterLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Message, 0, limit)
	for rows.Next() {
		var msg Message
		var token sql.NullInt64
		if err := rows.Scan(&msg.Seq, &msg.Role, &msg.Content, &token, &msg.CreatedAt); err != nil {
			return nil, err
		}
		if token.Valid {
			t := int(token.Int64)
			msg.TokenCount = &t
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) GetSessionSummary(ctx context.Context, tenantID, userID, sessionID string) (*SessionSummary, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}

	var summary SessionSummary
	var tenant sql.NullString
	var user sql.NullString
	var completedRaw []byte
	var openRaw []byte
	var decisionsRaw []byte
	var blockersRaw []byte

	err := s.db.QueryRowContext(ctx, `
SELECT tenant_id, user_id, session_id, current_goal, completed_items, open_items, key_decisions, blockers, source_message_seq, created_at, updated_at
FROM session_summaries
WHERE COALESCE(tenant_id, '') = COALESCE($1, '')
  AND COALESCE(user_id, '') = COALESCE($2, '')
  AND session_id = $3
`, tenantID, userID, sessionID).Scan(
		&tenant,
		&user,
		&summary.SessionID,
		&summary.CurrentGoal,
		&completedRaw,
		&openRaw,
		&decisionsRaw,
		&blockersRaw,
		&summary.SourceMessageSeq,
		&summary.CreatedAt,
		&summary.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if tenant.Valid {
		summary.TenantID = tenant.String
	}
	if user.Valid {
		summary.UserID = user.String
	}

	if summary.CompletedItems, err = decodeJSONStringArray(completedRaw); err != nil {
		return nil, err
	}
	if summary.OpenItems, err = decodeJSONStringArray(openRaw); err != nil {
		return nil, err
	}
	if summary.KeyDecisions, err = decodeJSONStringArray(decisionsRaw); err != nil {
		return nil, err
	}
	if summary.Blockers, err = decodeJSONStringArray(blockersRaw); err != nil {
		return nil, err
	}
	if summary.SourceMessageSeq < 0 {
		summary.SourceMessageSeq = 0
	}

	return &summary, nil
}

func (s *Store) UpsertSessionSummary(ctx context.Context, summary SessionSummary) error {
	if strings.TrimSpace(summary.SessionID) == "" {
		return nil
	}
	if summary.SourceMessageSeq < 0 {
		summary.SourceMessageSeq = 0
	}

	completedJSON, err := encodeJSONStringArray(summary.CompletedItems)
	if err != nil {
		return err
	}
	openJSON, err := encodeJSONStringArray(summary.OpenItems)
	if err != nil {
		return err
	}
	decisionsJSON, err := encodeJSONStringArray(summary.KeyDecisions)
	if err != nil {
		return err
	}
	blockersJSON, err := encodeJSONStringArray(summary.Blockers)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO session_summaries (tenant_id, user_id, session_id, current_goal, completed_items, open_items, key_decisions, blockers, source_message_seq, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9, NOW(), NOW())
ON CONFLICT (session_id) DO UPDATE SET
	tenant_id = COALESCE(EXCLUDED.tenant_id, session_summaries.tenant_id),
	user_id = COALESCE(EXCLUDED.user_id, session_summaries.user_id),
	current_goal = EXCLUDED.current_goal,
	completed_items = EXCLUDED.completed_items,
	open_items = EXCLUDED.open_items,
	key_decisions = EXCLUDED.key_decisions,
	blockers = EXCLUDED.blockers,
	source_message_seq = EXCLUDED.source_message_seq,
	updated_at = NOW()
`, summary.TenantID, summary.UserID, summary.SessionID, summary.CurrentGoal, completedJSON, openJSON, decisionsJSON, blockersJSON, summary.SourceMessageSeq)
	return err
}

func (s *Store) GetMessagesAfterSeq(ctx context.Context, sessionID string, afterSeq int64, limit int) ([]Message, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT seq, role, content, token_count, created_at
FROM messages
WHERE session_id = $1
  AND seq > $2
  AND deleted_at IS NULL
ORDER BY seq ASC
LIMIT $3
`, sessionID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Message, 0, limit)
	for rows.Next() {
		var msg Message
		var token sql.NullInt64
		if err := rows.Scan(&msg.Seq, &msg.Role, &msg.Content, &token, &msg.CreatedAt); err != nil {
			return nil, err
		}
		if token.Valid {
			t := int(token.Int64)
			msg.TokenCount = &t
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) RefreshSessionSummary(ctx context.Context, tenantID, userID, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}

	summary, err := s.GetSessionSummary(ctx, tenantID, userID, sessionID)
	if err != nil {
		return err
	}
	if summary == nil {
		summary = &SessionSummary{TenantID: tenantID, UserID: userID, SessionID: sessionID}
	}
	if summary.SourceMessageSeq < 0 {
		summary.SourceMessageSeq = 0
	}

	summary.TenantID = tenantID
	summary.UserID = userID
	summary.SessionID = sessionID

	const batchSize = 200
	cursorSeq := summary.SourceMessageSeq
	maxSeenSeq := summary.SourceMessageSeq
	for {
		messages, err := s.GetMessagesAfterSeq(ctx, sessionID, cursorSeq, batchSize)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			break
		}

		applySessionSummaryRules(summary, messages)
		lastSeq := messages[len(messages)-1].Seq
		if lastSeq > maxSeenSeq {
			maxSeenSeq = lastSeq
		}
		cursorSeq = lastSeq
		if len(messages) < batchSize {
			break
		}
	}

	// Phase L: summary 刷新时避免继续保留/强化已被 superseded 的项目事实引用。
	beforeDecisionCount := len(summary.KeyDecisions)
	beforeBlockerCount := len(summary.Blockers)
	if err := s.pruneSupersededProjectFactMentions(ctx, tenantID, userID, summary); err != nil {
		return err
	}
	summaryPruned := len(summary.KeyDecisions) != beforeDecisionCount || len(summary.Blockers) != beforeBlockerCount

	if maxSeenSeq <= summary.SourceMessageSeq && !summaryPruned {
		return nil
	}
	if maxSeenSeq > summary.SourceMessageSeq {
		summary.SourceMessageSeq = maxSeenSeq
	}
	return s.UpsertSessionSummary(ctx, *summary)
}

func applySessionSummaryRules(summary *SessionSummary, messages []Message) {
	for _, msg := range messages {
		content := normalizeSummaryText(msg.Content)
		if content == "" {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(msg.Role))

		if role == "user" {
			if goal, ok := extractStrongGoal(content); ok {
				summary.CurrentGoal = goal
			}
		}
		if item, ok := extractCompletedItem(content); ok {
			summary.CompletedItems = addUniqueSummaryItem(summary.CompletedItems, item, 20)
			summary.OpenItems = removeSummaryItem(summary.OpenItems, item)
			summary.Blockers = removeSummaryItem(summary.Blockers, item)
		}
		if item, ok := extractOpenItem(content); ok {
			summary.OpenItems = addUniqueSummaryItem(summary.OpenItems, item, 20)
		}
		if item, ok := extractKeyDecision(content); ok {
			summary.KeyDecisions = addUniqueSummaryItem(summary.KeyDecisions, item, 20)
		}
		if item, ok := extractBlocker(content); ok {
			summary.Blockers = addUniqueSummaryItem(summary.Blockers, item, 20)
		}
		if item, ok := extractResolvedBlocker(content); ok {
			summary.Blockers = removeSummaryItem(summary.Blockers, item)
		}
	}
}

func extractStrongGoal(content string) (string, bool) {
	markers := []string{"current goal:", "goal:", "objective:", "当前目标：", "目标：", "本次目标："}
	return extractByLeadingMarkers(content, markers)
}

func extractCompletedItem(content string) (string, bool) {
	markers := []string{"completed:", "done:", "finished:", "已完成：", "完成：", "完成了：", "搞定："}
	return extractByContainingMarkers(content, markers)
}

func extractOpenItem(content string) (string, bool) {
	markers := []string{"todo:", "next:", "open item:", "open:", "待办：", "下一步：", "计划："}
	return extractByContainingMarkers(content, markers)
}

func extractKeyDecision(content string) (string, bool) {
	markers := []string{"decision:", "we decided to", "decided to", "最终决定", "决定：", "确定采用", "结论："}
	return extractByContainingMarkers(content, markers)
}

func extractBlocker(content string) (string, bool) {
	markers := []string{"blocker:", "blocked by", "stuck on", "卡住：", "阻塞：", "受阻于", "无法继续："}
	return extractByContainingMarkers(content, markers)
}

func extractResolvedBlocker(content string) (string, bool) {
	markers := []string{"blocker resolved:", "unblocked:", "resolved blocker:", "阻塞已解决：", "已解除阻塞：", "问题已解决："}
	return extractByContainingMarkers(content, markers)
}

func extractByLeadingMarkers(content string, markers []string) (string, bool) {
	normalized := strings.TrimSpace(content)
	if normalized == "" {
		return "", false
	}
	lower := strings.ToLower(normalized)
	for _, marker := range markers {
		markerLower := strings.ToLower(strings.TrimSpace(marker))
		if markerLower == "" {
			continue
		}
		if strings.HasPrefix(lower, markerLower) {
			idx := len(marker)
			if idx > len(normalized) {
				idx = len(normalized)
			}
			item := normalizeSignalItem(normalized[idx:])
			if item != "" {
				return item, true
			}
		}
	}
	return "", false
}

func extractByContainingMarkers(content string, markers []string) (string, bool) {
	normalized := strings.TrimSpace(content)
	if normalized == "" {
		return "", false
	}
	lower := strings.ToLower(normalized)
	for _, marker := range markers {
		markerLower := strings.ToLower(strings.TrimSpace(marker))
		if markerLower == "" {
			continue
		}
		idx := strings.Index(lower, markerLower)
		if idx < 0 {
			continue
		}
		start := idx + len(markerLower)
		if start > len(normalized) {
			start = len(normalized)
		}
		item := normalizeSignalItem(normalized[start:])
		if item != "" {
			return item, true
		}
	}
	return "", false
}

func normalizeSummaryText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return strings.Join(strings.Fields(content), " ")
}

func normalizeSignalItem(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimLeft(raw, " :-：，,。;；")
	if raw == "" {
		return ""
	}
	if idx := strings.IndexAny(raw, "\n。；;!！?"); idx >= 0 {
		raw = raw[:idx]
	}
	raw = trim(raw)
	if len([]rune(raw)) < 3 {
		return ""
	}
	return raw
}

func addUniqueSummaryItem(items []string, item string, maxItems int) []string {
	item = trim(item)
	if item == "" {
		return items
	}
	target := canonicalSummaryItem(item)
	for _, existing := range items {
		if canonicalSummaryItem(existing) == target {
			return items
		}
	}
	items = append(items, item)
	if maxItems > 0 && len(items) > maxItems {
		items = items[len(items)-maxItems:]
	}
	return items
}

func removeSummaryItem(items []string, item string) []string {
	item = trim(item)
	if item == "" || len(items) == 0 {
		return items
	}
	target := canonicalSummaryItem(item)
	out := make([]string, 0, len(items))
	for _, existing := range items {
		if canonicalSummaryItem(existing) == target {
			continue
		}
		out = append(out, existing)
	}
	return out
}

func canonicalSummaryItem(item string) string {
	item = strings.ToLower(strings.TrimSpace(item))
	return strings.Join(strings.Fields(item), " ")
}

func pruneSummaryItemsReferencingFactValues(items []string, factValues []string) []string {
	if len(items) == 0 || len(factValues) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		itemText := strings.ToLower(strings.TrimSpace(item))
		if itemText == "" {
			continue
		}
		matched := false
		for _, value := range factValues {
			if value == "" {
				continue
			}
			if strings.Contains(itemText, value) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *Store) pruneSupersededProjectFactMentions(ctx context.Context, tenantID, userID string, summary *SessionSummary) error {
	if summary == nil {
		return nil
	}
	allFacts, err := s.getProjectFacts(ctx, tenantID, userID, true)
	if err != nil {
		return err
	}
	if len(allFacts) == 0 {
		return nil
	}

	supersededValues := make([]string, 0, len(allFacts))
	for _, fact := range allFacts {
		if strings.ToLower(strings.TrimSpace(fact.Status)) != "superseded" {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(fact.Value))
		if value == "" {
			continue
		}
		supersededValues = append(supersededValues, value)
	}
	if len(supersededValues) == 0 {
		return nil
	}

	summary.KeyDecisions = pruneSummaryItemsReferencingFactValues(summary.KeyDecisions, supersededValues)
	summary.Blockers = pruneSummaryItemsReferencingFactValues(summary.Blockers, supersededValues)
	return nil
}

func (s *Store) SearchMessages(ctx context.Context, tenantID, query string, limit, offset int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT
	m.conversation_id,
	m.session_id,
	m.id,
	m.seq,
	ts_headline('english', m.content, plainto_tsquery('english', $2)) AS snippet
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE COALESCE(c.tenant_id, '') = COALESCE($1, '')
  AND COALESCE(c.status, 'active') <> 'deleted'
  AND m.deleted_at IS NULL
  AND m.search_vector @@ plainto_tsquery('english', $2)
ORDER BY m.created_at DESC, m.id DESC
LIMIT $3 OFFSET $4
`, tenantID, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SearchResult, 0, limit)
	for rows.Next() {
		var item SearchResult
		if err := rows.Scan(&item.ConversationID, &item.SessionID, &item.MessageID, &item.Seq, &item.Snippet); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func InjectMemory(req providers.ChatCompletionRequest, items []Item) providers.ChatCompletionRequest {
	if len(items) == 0 {
		return req
	}
	lines := make([]string, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		lines = append(lines, fmt.Sprintf("- %s: %s", items[i].Role, items[i].Content))
	}
	memoryMessage := providers.ChatMessage{Role: "system", Content: "Session memory:\n" + strings.Join(lines, "\n")}
	req.Messages = append([]providers.ChatMessage{memoryMessage}, req.Messages...)
	return req
}

func FormatSessionSummary(summary *SessionSummary) string {
	if summary == nil {
		return ""
	}

	sections := []string{"[Session Summary]"}
	if goal := strings.TrimSpace(summary.CurrentGoal); goal != "" {
		sections = append(sections, "Current Goal:\n"+goal)
	} else {
		sections = append(sections, "Current Goal:\n- (none)")
	}
	sections = append(sections, formatSummaryListSection("Completed Items", summary.CompletedItems))
	sections = append(sections, formatSummaryListSection("Open Items", summary.OpenItems))
	sections = append(sections, formatSummaryListSection("Key Decisions", summary.KeyDecisions))
	sections = append(sections, formatSummaryListSection("Blockers", summary.Blockers))
	sections = append(sections, fmt.Sprintf("Source Message Seq:\n- %d", summary.SourceMessageSeq))

	return strings.Join(sections, "\n\n")
}

func (s *Store) GetUserPreferences(ctx context.Context, tenantID, userID string) ([]UserPreference, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT tenant_id, user_id, preference_key, preference_value, source_text, created_at, updated_at
FROM user_preferences
WHERE COALESCE(tenant_id, '') = COALESCE($1, '')
  AND COALESCE(user_id, '') = COALESCE($2, '')
ORDER BY preference_key ASC
`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserPreference, 0, 8)
	for rows.Next() {
		var pref UserPreference
		var tenant sql.NullString
		var user sql.NullString
		if err := rows.Scan(&tenant, &user, &pref.Key, &pref.Value, &pref.SourceText, &pref.CreatedAt, &pref.UpdatedAt); err != nil {
			return nil, err
		}
		if tenant.Valid {
			pref.TenantID = tenant.String
		}
		if user.Valid {
			pref.UserID = user.String
		}
		pref.Key = strings.TrimSpace(pref.Key)
		pref.Value = strings.TrimSpace(pref.Value)
		pref.SourceText = strings.TrimSpace(pref.SourceText)
		if pref.Key == "" || pref.Value == "" {
			continue
		}
		out = append(out, pref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpsertUserPreference(ctx context.Context, pref UserPreference) error {
	pref.Key = strings.TrimSpace(strings.ToLower(pref.Key))
	pref.Value = trim(pref.Value)
	pref.SourceText = trim(pref.SourceText)
	if pref.Key == "" || pref.Value == "" {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO user_preferences (tenant_id, user_id, preference_key, preference_value, source_text, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT ((COALESCE(tenant_id, '')), (COALESCE(user_id, '')), preference_key) DO UPDATE SET
	preference_value = EXCLUDED.preference_value,
	source_text = EXCLUDED.source_text,
	updated_at = NOW()
`, pref.TenantID, pref.UserID, pref.Key, pref.Value, pref.SourceText)
	return err
}

func FormatUserPreferences(prefs []UserPreference) string {
	if len(prefs) == 0 {
		return ""
	}

	sections := []string{"[User Preferences]", "Long-term explicit user preferences (highest priority after system rules):"}
	for _, pref := range prefs {
		key := strings.TrimSpace(pref.Key)
		value := strings.TrimSpace(pref.Value)
		if key == "" || value == "" {
			continue
		}
		line := fmt.Sprintf("- %s: %s", key, value)
		if source := strings.TrimSpace(pref.SourceText); source != "" {
			line += fmt.Sprintf(" (source: %q)", source)
		}
		sections = append(sections, line)
	}
	if len(sections) <= 2 {
		return ""
	}
	return strings.Join(sections, "\n")
}

func (s *Store) GetProjectFacts(ctx context.Context, tenantID, userID string) ([]ProjectFact, error) {
	return s.getProjectFacts(ctx, tenantID, userID, false)
}

func (s *Store) getProjectFacts(ctx context.Context, tenantID, userID string, includeSuperseded bool) ([]ProjectFact, error) {
	query := `
SELECT id, tenant_id, user_id, fact_key, fact_value, source_text, status, superseded_by, source_message_seq, last_verified_at, created_at, updated_at
FROM project_facts
WHERE COALESCE(tenant_id, '') = COALESCE($1, '')
  AND COALESCE(user_id, '') = COALESCE($2, '')`
	if !includeSuperseded {
		query += "\n  AND status = 'active'"
	}
	query += "\nORDER BY fact_key ASC, updated_at DESC"

	rows, err := s.db.QueryContext(ctx, query, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProjectFact, 0, 8)
	for rows.Next() {
		var fact ProjectFact
		var tenant sql.NullString
		var user sql.NullString
		var status sql.NullString
		var supersededBy sql.NullInt64
		if err := rows.Scan(
			&fact.ID,
			&tenant,
			&user,
			&fact.Key,
			&fact.Value,
			&fact.SourceText,
			&status,
			&supersededBy,
			&fact.SourceMessageSeq,
			&fact.LastVerifiedAt,
			&fact.CreatedAt,
			&fact.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if tenant.Valid {
			fact.TenantID = tenant.String
		}
		if user.Valid {
			fact.UserID = user.String
		}
		if status.Valid {
			fact.Status = strings.TrimSpace(strings.ToLower(status.String))
		}
		if fact.Status == "" {
			fact.Status = "active"
		}
		if supersededBy.Valid {
			v := supersededBy.Int64
			fact.SupersededBy = &v
		}
		if fact.SourceMessageSeq < 0 {
			fact.SourceMessageSeq = 0
		}
		fact.Key = strings.TrimSpace(fact.Key)
		fact.Value = strings.TrimSpace(fact.Value)
		fact.SourceText = strings.TrimSpace(fact.SourceText)
		if fact.Key == "" || fact.Value == "" {
			continue
		}
		out = append(out, fact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpsertProjectFact(ctx context.Context, fact ProjectFact) error {
	fact.Key = strings.TrimSpace(strings.ToLower(fact.Key))
	fact.Value = trim(fact.Value)
	fact.SourceText = trim(fact.SourceText)
	if fact.Key == "" || fact.Value == "" {
		return nil
	}
	if fact.SourceMessageSeq < 0 {
		fact.SourceMessageSeq = 0
	}
	verifiedAt := fact.LastVerifiedAt
	if verifiedAt.IsZero() {
		verifiedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var oldActiveID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
SELECT id
FROM project_facts
WHERE COALESCE(tenant_id, '') = COALESCE($1, '')
  AND COALESCE(user_id, '') = COALESCE($2, '')
  AND fact_key = $3
  AND status = 'active'
ORDER BY id DESC
LIMIT 1
FOR UPDATE
`, fact.TenantID, fact.UserID, fact.Key).Scan(&oldActiveID); err != nil && err != sql.ErrNoRows {
		return err
	}

	if oldActiveID.Valid {
		if _, err := tx.ExecContext(ctx, `
UPDATE project_facts
SET status = 'superseded', updated_at = NOW()
WHERE id = $1
`, oldActiveID.Int64); err != nil {
			return err
		}
	}

	var newFactID int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO project_facts (tenant_id, user_id, fact_key, fact_value, source_text, status, source_message_seq, last_verified_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 'active', $6, $7, NOW(), NOW())
RETURNING id
`, fact.TenantID, fact.UserID, fact.Key, fact.Value, fact.SourceText, fact.SourceMessageSeq, verifiedAt).Scan(&newFactID); err != nil {
		return err
	}

	if oldActiveID.Valid {
		if _, err := tx.ExecContext(ctx, `
UPDATE project_facts
SET superseded_by = $2, updated_at = NOW()
WHERE id = $1
`, oldActiveID.Int64, newFactID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func FormatProjectFacts(facts []ProjectFact) string {
	if len(facts) == 0 {
		return ""
	}

	sections := []string{"[Project Facts]", "Stable confirmed architecture/workflow facts. Treat these as project constraints; do not override unless user explicitly updates them:"}
	for _, fact := range facts {
		key := strings.TrimSpace(fact.Key)
		value := strings.TrimSpace(fact.Value)
		if key == "" || value == "" {
			continue
		}
		line := fmt.Sprintf("- %s: %s", key, value)
		if source := strings.TrimSpace(fact.SourceText); source != "" {
			line += fmt.Sprintf(" (source: %q)", source)
		}
		sections = append(sections, line)
	}
	if len(sections) <= 2 {
		return ""
	}
	return strings.Join(sections, "\n")
}

func (s *Store) DeleteConversation(ctx context.Context, tenantID, sessionID, actorID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var conversationID int64
	if err := tx.QueryRowContext(ctx, `
UPDATE conversations
SET status = 'deleted', updated_at = NOW()
WHERE COALESCE(tenant_id, '') = COALESCE($1, '')
  AND session_id = $2
RETURNING id
`, tenantID, sessionID).Scan(&conversationID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET deleted_at = NOW()
WHERE conversation_id = $1 AND deleted_at IS NULL
`, conversationID); err != nil {
		return err
	}

	if err := s.insertBusinessAuditActionInTx(ctx, tx, tenantID, "delete_conversation", "conversation", sessionID, actorID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.invalidateConversationCacheAsync(ctx, conversationID)
	return nil
}

func (s *Store) DeleteMessage(ctx context.Context, tenantID, sessionID string, seq int64, actorID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if seq <= 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var conversationID int64
	var messageID int64
	if err := tx.QueryRowContext(ctx, `
UPDATE messages
SET deleted_at = NOW()
WHERE session_id = $1 AND seq = $2 AND deleted_at IS NULL
RETURNING conversation_id, id
`, sessionID, seq).Scan(&conversationID, &messageID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	if err := s.insertBusinessAuditActionInTx(ctx, tx, tenantID, "delete_message", "message", strconv.FormatInt(messageID, 10), actorID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.invalidateConversationCacheAsync(ctx, conversationID)
	return nil
}

func (s *Store) ExportConversation(ctx context.Context, tenantID, sessionID, actorID string) ([]Message, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
SELECT seq, role, content, token_count, created_at
FROM messages
WHERE session_id = $1 AND deleted_at IS NULL
ORDER BY seq ASC
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Message, 0, 32)
	for rows.Next() {
		var msg Message
		var token sql.NullInt64
		if err := rows.Scan(&msg.Seq, &msg.Role, &msg.Content, &token, &msg.CreatedAt); err != nil {
			return nil, err
		}
		if token.Valid {
			t := int(token.Int64)
			msg.TokenCount = &t
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := s.insertBusinessAuditActionInTx(ctx, tx, tenantID, "export_conversation", "conversation", sessionID, actorID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) invalidateConversationCacheAsync(ctx context.Context, conversationID int64) {
	if s.cache == nil {
		return
	}
	conversationKey := strconv.FormatInt(conversationID, 10)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if err := s.cache.InvalidateConversationCache(cacheCtx, conversationKey); err != nil {
			log.Printf("memory cache invalidate failed conversation_id=%s: %v", conversationKey, err)
		}
	}()
}

func decodeJSONStringArray(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}

func encodeJSONStringArray(items []string) (string, error) {
	if items == nil {
		items = []string{}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatSummaryListSection(title string, items []string) string {
	lines := []string{title + ":"}
	if len(items) == 0 {
		lines = append(lines, "- (none)")
		return strings.Join(lines, "\n")
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		lines = append(lines, "- "+item)
	}
	if len(lines) == 1 {
		lines = append(lines, "- (none)")
	}
	return strings.Join(lines, "\n")
}

func trim(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

