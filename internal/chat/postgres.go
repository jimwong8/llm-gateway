package chat

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type PostgresStore struct {
	db *sql.DB
}

func NewStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("chat: open db: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) CreateSession(ctx context.Context, userID int64, title, model string) (*Session, error) {
	var session Session
	err := s.db.QueryRowContext(ctx, `
INSERT INTO chat_sessions (user_id, title, model)
VALUES ($1, $2, $3)
RETURNING id, user_id, title, model, visibility, COALESCE(share_hash, ''), created_at, updated_at`,
		userID, title, model,
	).Scan(&session.ID, &session.UserID, &session.Title, &session.Model,
		&session.Visibility, &session.ShareHash, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("chat: create session: %w", err)
	}
	return &session, nil
}

func (s *PostgresStore) GetSession(ctx context.Context, sessionID, userID int64) (*Session, error) {
	var session Session
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, title, model, visibility, COALESCE(share_hash, ''), created_at, updated_at
FROM chat_sessions
WHERE id = $1 AND user_id = $2`,
		sessionID, userID,
	).Scan(&session.ID, &session.UserID, &session.Title, &session.Model,
		&session.Visibility, &session.ShareHash, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("chat: get session: %w", err)
	}
	return &session, nil
}

func (s *PostgresStore) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]*Session, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, title, model, visibility, COALESCE(share_hash, ''), created_at, updated_at
FROM chat_sessions
WHERE user_id = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		if err := rows.Scan(&session.ID, &session.UserID, &session.Title, &session.Model,
			&session.Visibility, &session.ShareHash, &session.CreatedAt, &session.UpdatedAt); err != nil {
			return nil, fmt.Errorf("chat: scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}
	return sessions, rows.Err()
}

func (s *PostgresStore) UpdateSessionTitle(ctx context.Context, sessionID, userID int64, title string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE chat_sessions
SET title = $1, updated_at = NOW()
WHERE id = $2 AND user_id = $3`,
		title, sessionID, userID,
	)
	if err != nil {
		return fmt.Errorf("chat: update session title: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) DeleteSession(ctx context.Context, sessionID, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM chat_sessions WHERE id = $1 AND user_id = $2`, sessionID, userID)
	if err != nil {
		return fmt.Errorf("chat: delete session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) CreateShareLink(ctx context.Context, sessionID, userID int64) (string, error) {
	hash := newShareHash()
	res, err := s.db.ExecContext(ctx, `
UPDATE chat_sessions
SET share_hash = $1, visibility = 'shared', updated_at = NOW()
WHERE id = $2 AND user_id = $3`,
		hash, sessionID, userID,
	)
	if err != nil {
		return "", fmt.Errorf("chat: create share link: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", sql.ErrNoRows
	}
	return hash, nil
}

func (s *PostgresStore) GetSessionByShareHash(ctx context.Context, hash string) (*Session, error) {
	var session Session
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, title, model, visibility, COALESCE(share_hash, ''), created_at, updated_at
FROM chat_sessions
WHERE share_hash = $1`,
		hash,
	).Scan(&session.ID, &session.UserID, &session.Title, &session.Model,
		&session.Visibility, &session.ShareHash, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("chat: get session by share hash: %w", err)
	}
	return &session, nil
}

func (s *PostgresStore) AddMessage(ctx context.Context, sessionID int64, role, content, model string, tokens int) (*Message, error) {
	var msg Message
	err := s.db.QueryRowContext(ctx, `
INSERT INTO chat_messages (session_id, role, content, model, tokens)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, session_id, role, content, COALESCE(model, ''), tokens, created_at`,
		sessionID, role, content, model, tokens,
	).Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.Model, &msg.Tokens, &msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("chat: add message: %w", err)
	}

	_, _ = s.db.ExecContext(ctx, `UPDATE chat_sessions SET updated_at = NOW() WHERE id = $1`, sessionID)
	return &msg, nil
}

func (s *PostgresStore) GetMessages(ctx context.Context, sessionID int64, limit, offset int) ([]*Message, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, role, content, COALESCE(model, ''), tokens, created_at
FROM chat_messages
WHERE session_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3`,
		sessionID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: get messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.Model, &msg.Tokens, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: scan message: %w", err)
		}
		messages = append(messages, &msg)
	}
	return messages, rows.Err()
}

func newShareHash() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
