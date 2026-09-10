package chat

import (
	"context"
	"time"
)

type Session struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Title      string    `json:"title"`
	Model      string    `json:"model"`
	Visibility string    `json:"visibility"`
	ShareHash  string    `json:"share_hash,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Message struct {
	ID        int64     `json:"id"`
	SessionID int64     `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Model     string    `json:"model,omitempty"`
	Tokens    int       `json:"tokens"`
	CreatedAt time.Time `json:"created_at"`
}

type Store interface {
	CreateSession(ctx context.Context, userID int64, title, model string) (*Session, error)
	GetSession(ctx context.Context, sessionID, userID int64) (*Session, error)
	ListSessions(ctx context.Context, userID int64, limit, offset int) ([]*Session, error)
	UpdateSessionTitle(ctx context.Context, sessionID, userID int64, title string) error
	DeleteSession(ctx context.Context, sessionID, userID int64) error
	CreateShareLink(ctx context.Context, sessionID, userID int64) (string, error)
	GetSessionByShareHash(ctx context.Context, hash string) (*Session, error)
	AddMessage(ctx context.Context, sessionID int64, role, content, model string, tokens int) (*Message, error)
	GetMessages(ctx context.Context, sessionID int64, limit, offset int) ([]*Message, error)
}
