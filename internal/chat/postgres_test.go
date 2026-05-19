package chat

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func chatTestPostgresDSN(t *testing.T) string {
	t.Helper()
	if dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	t.Skip("skip integration test: set POSTGRES_DSN")
	return ""
}

func TestChatStore_CreateSession(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	session, err := store.CreateSession(ctx, userID, "Test Chat", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.ID <= 0 {
		t.Fatal("expected session ID > 0")
	}
	if session.Title != "Test Chat" {
		t.Fatalf("expected title 'Test Chat', got %q", session.Title)
	}
	if session.Model != "gpt-4o-mini" {
		t.Fatalf("expected model 'gpt-4o-mini', got %q", session.Model)
	}
	if session.UserID != userID {
		t.Fatalf("expected user_id %d, got %d", userID, session.UserID)
	}
	if session.Visibility != "private" {
		t.Fatalf("expected visibility 'private', got %q", session.Visibility)
	}
	if session.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}
	if session.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero updated_at")
	}
}

func TestChatStore_GetSession(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	created, err := store.CreateSession(ctx, userID, "Get Test", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	got, err := store.GetSession(ctx, created.ID, userID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected id %d, got %d", created.ID, got.ID)
	}
	if got.Title != "Get Test" {
		t.Fatalf("expected title 'Get Test', got %q", got.Title)
	}
}

func TestChatStore_GetSession_WrongUser(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	created, err := store.CreateSession(ctx, 1, "Wrong User Test", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	_, err = store.GetSession(ctx, created.ID, 999)
	if err == nil {
		t.Fatal("expected error for wrong user, got nil")
	}
}

func TestChatStore_ListSessions(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	for i := 0; i < 3; i++ {
		_, err := store.CreateSession(ctx, userID, "List Test", "gpt-4o-mini")
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
	}

	sessions, err := store.ListSessions(ctx, userID, 10, 0)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) < 3 {
		t.Fatalf("expected at least 3 sessions, got %d", len(sessions))
	}
	for _, s := range sessions {
		if s.UserID != userID {
			t.Fatalf("expected user_id %d, got %d", userID, s.UserID)
		}
	}
}

func TestChatStore_UpdateSessionTitle(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	created, err := store.CreateSession(ctx, userID, "Old Title", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	err = store.UpdateSessionTitle(ctx, created.ID, userID, "New Title")
	if err != nil {
		t.Fatalf("UpdateSessionTitle() error = %v", err)
	}

	got, err := store.GetSession(ctx, created.ID, userID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got.Title != "New Title" {
		t.Fatalf("expected title 'New Title', got %q", got.Title)
	}
}

func TestChatStore_DeleteSession(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	created, err := store.CreateSession(ctx, userID, "Delete Test", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	err = store.DeleteSession(ctx, created.ID, userID)
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	_, err = store.GetSession(ctx, created.ID, userID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestChatStore_CreateShareLink(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	created, err := store.CreateSession(ctx, userID, "Share Test", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	hash, err := store.CreateShareLink(ctx, created.ID, userID)
	if err != nil {
		t.Fatalf("CreateShareLink() error = %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty share hash")
	}
}

func TestChatStore_GetSessionByShareHash(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	created, err := store.CreateSession(ctx, userID, "Share Test 2", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	hash, err := store.CreateShareLink(ctx, created.ID, userID)
	if err != nil {
		t.Fatalf("CreateShareLink() error = %v", err)
	}

	got, err := store.GetSessionByShareHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetSessionByShareHash() error = %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected id %d, got %d", created.ID, got.ID)
	}
}

func TestChatStore_AddMessage(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	session, err := store.CreateSession(ctx, userID, "Message Test", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	msg, err := store.AddMessage(ctx, session.ID, "user", "Hello!", "gpt-4o-mini", 10)
	if err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}
	if msg.ID <= 0 {
		t.Fatal("expected message ID > 0")
	}
	if msg.Role != "user" {
		t.Fatalf("expected role 'user', got %q", msg.Role)
	}
	if msg.Content != "Hello!" {
		t.Fatalf("expected content 'Hello!', got %q", msg.Content)
	}
	if msg.Model != "gpt-4o-mini" {
		t.Fatalf("expected model 'gpt-4o-mini', got %q", msg.Model)
	}
	if msg.Tokens != 10 {
		t.Fatalf("expected tokens 10, got %d", msg.Tokens)
	}
	if msg.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}
}

func TestChatStore_GetMessages(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	session, err := store.CreateSession(ctx, userID, "Get Messages", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	_, err = store.AddMessage(ctx, session.ID, "user", "Hi", "gpt-4o-mini", 5)
	if err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}
	_, err = store.AddMessage(ctx, session.ID, "assistant", "Hello!", "gpt-4o-mini", 20)
	if err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}

	messages, err := store.GetMessages(ctx, session.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}
}

func TestChatStore_AddMessage_DeletedSession(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)
	session, err := store.CreateSession(ctx, userID, "Delete Cascade", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	_, err = store.AddMessage(ctx, session.ID, "user", "Hi", "gpt-4o-mini", 5)
	if err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}

	err = store.DeleteSession(ctx, session.ID, userID)
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	messages, err := store.GetMessages(ctx, session.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages() error = %v (expected not found after cascade)", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages after session delete, got %d", len(messages))
	}
}

func TestChatStore_ListSessions_Ordered(t *testing.T) {
	dsn := chatTestPostgresDSN(t)
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	userID := int64(1)

	s1, _ := store.CreateSession(ctx, userID, "First", "gpt-4o-mini")
	time.Sleep(5 * time.Millisecond)
	s2, _ := store.CreateSession(ctx, userID, "Second", "gpt-4o-mini")

	sessions, err := store.ListSessions(ctx, userID, 10, 0)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	found1 := false
	found2 := false
	for _, s := range sessions {
		if s.ID == s1.ID {
			found1 = true
		}
		if s.ID == s2.ID {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatal("expected both sessions in list")
	}

	if len(sessions) >= 2 {
		if sessions[0].UpdatedAt.Before(sessions[1].UpdatedAt) {
			t.Fatal("expected sessions ordered by updated_at DESC")
		}
	}
}
