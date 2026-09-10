# Streaming Chat MVP Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executions-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a user-facing streaming chat interface to llm-gateway with SSE streaming, multi-model selection, conversation history, and chat sharing.

**Architecture:**
- New `internal/chat/` package: session management, message storage
- New `internal/httpserver/chat_handler.go`: REST API for chat sessions + SSE streaming endpoint
- New `web/admin/src/pages/user/ChatPage.tsx`: main chat UI
- New `web/admin/src/components/chat/`: reusable chat components
- DB migration: `chat_sessions` + `chat_messages` tables

**Tech Stack:** Go 1.22+, PostgreSQL, SSE (Server-Sent Events), React 18, TypeScript

---

## 1. DB Migration

### Task 1: Create chat_sessions and chat_messages tables

**Files:**
- Create: `internal/db/migrations/015_chat_sessions.sql`
- Create: `internal/db/migrations/016_chat_messages.sql`

```sql
-- 015_chat_sessions.sql
CREATE TABLE IF NOT EXISTS chat_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT 'New Chat',
    model VARCHAR(128) NOT NULL DEFAULT 'gpt-4o-mini',
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',
    share_hash VARCHAR(64) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_id ON chat_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_share_hash ON chat_sessions (share_hash);

-- 016_chat_messages.sql
CREATE TABLE IF NOT EXISTS chat_messages (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    model VARCHAR(128),
    tokens INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages (session_id);
```

---

## 2. Chat Domain (internal/chat/)

### Task 2: Chat session/message models and repository

**Files:**
- Create: `internal/chat/models.go`
- Create: `internal/chat/postgres.go`
- Create: `internal/chat/postgres_test.go`

Models:
```go
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
```

Repository interface:
```go
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
```

TDD: write tests first, then implement.

---

## 3. Chat HTTP Handlers

### Task 3: Chat handler with SSE streaming

**Files:**
- Create: `internal/httpserver/chat_handler.go`
- Create: `internal/httpserver/chat_handler_test.go`
- Modify: `internal/httpserver/server.go`

Routes (all require JWT auth):
- `POST /api/chat/sessions` — create new session
- `GET /api/chat/sessions` — list user's sessions
- `GET /api/chat/sessions/{id}` — get session with messages
- `PUT /api/chat/sessions/{id}` — update session title
- `DELETE /api/chat/sessions/{id}` — delete session
- `POST /api/chat/sessions/{id}/messages:stream` — send message, receive SSE stream
- `POST /api/chat/sessions/{id}/share` — create share link
- `GET /api/chat/share/{hash}` — view shared session (public, no auth required)

SSE streaming:
- Use `text/event-stream` content type
- Each chunk: `data: {JSON}\n\n`
- Final chunk: `data: [DONE]\n\n`
- On error: `event: error\ndata: {JSON}\n\n`

Integration with existing chat completions:
- The SSE handler internally calls the same provider routing as `/v1/chat/completions`
- But wraps it in SSE format and stores messages

---

## 4. Frontend Chat UI

### Task 4: Chat page and components

**Files:**
- Create: `web/admin/src/pages/user/ChatPage.tsx`
- Create: `web/admin/src/components/chat/MessageList.tsx`
- Create: `web/admin/src/components/chat/ChatInput.tsx`
- Create: `web/admin/src/components/chat/ModelSelector.tsx`
- Create: `web/admin/src/components/chat/SessionSidebar.tsx`
- Create: `web/admin/src/hooks/useSseChat.ts`
- Create: `web/admin/src/lib/api/chat.ts`
- Create: `web/admin/src/types/chat.ts`
- Modify: `web/admin/src/router.tsx`
- Modify: `web/admin/src/components/layout/Sidebar.tsx`

ChatPage layout:
```
┌─────────────────────────────────────────────────────┐
│ Topbar: Model selector + New Chat button            │
├──────────────┬──────────────────────────────────────┤
│  Session     │  Message list (scrollable)           │
│  Sidebar     │  ┌────────────────────────────────┐  │
│  - Session 1 │  │ User: Hello                     │  │
│  - Session 2 │  │ AI: Hi there! How can I help?   │  │
│  - ...       │  │                                 │  │
│              │  │ [Streaming text...]             │  │
│              │  └────────────────────────────────┘  │
│              │  ┌────────────────────────────────┐  │
│              │  │ ChatInput                      │  │
│              │  └────────────────────────────────┘  │
└──────────────┴──────────────────────────────────────┘
```

useSseChat hook:
- Manages SSE connection
- Accumulates streaming chunks
- Handles errors and reconnection
- Stores messages in React state

---

## 5. Verification

- [ ] Backend tests pass
- [ ] Frontend builds
- [ ] Chat session CRUD works
- [ ] SSE streaming delivers chunks
- [ ] Messages stored in DB
- [ ] Share link works
