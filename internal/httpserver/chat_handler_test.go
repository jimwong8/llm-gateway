package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llm-gateway/gateway/internal/auth"
	"llm-gateway/gateway/internal/chat"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
)

const testJWTSecret = "test-jwt-secret-at-least-32-characters!!!"

func validChatToken() string {
	t, err := auth.GenerateToken(1, "test@example.com", "user", testJWTSecret, 24*time.Hour)
	if err != nil {
		panic(err)
	}
	return t
}

func newMockRegistry() *providers.Registry {
	mock := providers.NewMockProvider("mock-provider", "mock-model")
	return providers.NewRegistry(config.Config{}, mock, mock)
}

type mockChatStore struct {
	sessions []*chat.Session
	messages []*chat.Message
	nextID   int64
}

func (m *mockChatStore) CreateSession(_ context.Context, userID int64, title, model string) (*chat.Session, error) {
	m.nextID++
	s := &chat.Session{ID: m.nextID, UserID: userID, Title: title, Model: model, Visibility: "private"}
	m.sessions = append(m.sessions, s)
	return s, nil
}

func (m *mockChatStore) GetSession(_ context.Context, sessionID, userID int64) (*chat.Session, error) {
	for _, s := range m.sessions {
		if s.ID == sessionID && s.UserID == userID {
			return s, nil
		}
	}
	return nil, errNotFound
}

func (m *mockChatStore) ListSessions(_ context.Context, userID int64, limit, offset int) ([]*chat.Session, error) {
	var out []*chat.Session
	for _, s := range m.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *mockChatStore) UpdateSessionTitle(_ context.Context, sessionID, userID int64, title string) error {
	for _, s := range m.sessions {
		if s.ID == sessionID && s.UserID == userID {
			s.Title = title
			return nil
		}
	}
	return errNotFound
}

func (m *mockChatStore) DeleteSession(_ context.Context, sessionID, userID int64) error {
	for i, s := range m.sessions {
		if s.ID == sessionID && s.UserID == userID {
			m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
			return nil
		}
	}
	return errNotFound
}

func (m *mockChatStore) CreateShareLink(_ context.Context, sessionID, userID int64) (string, error) {
	for _, s := range m.sessions {
		if s.ID == sessionID && s.UserID == userID {
			s.ShareHash = "testhash123"
			s.Visibility = "shared"
			return "testhash123", nil
		}
	}
	return "", errNotFound
}

func (m *mockChatStore) GetSessionByShareHash(_ context.Context, hash string) (*chat.Session, error) {
	for _, s := range m.sessions {
		if s.ShareHash == hash {
			return s, nil
		}
	}
	return nil, errNotFound
}

func (m *mockChatStore) AddMessage(_ context.Context, sessionID int64, role, content, model string, tokens int) (*chat.Message, error) {
	m.nextID++
	msg := &chat.Message{ID: m.nextID, SessionID: sessionID, Role: role, Content: content, Model: model, Tokens: tokens}
	m.messages = append(m.messages, msg)
	return msg, nil
}

func (m *mockChatStore) GetMessages(_ context.Context, sessionID int64, limit, offset int) ([]*chat.Message, error) {
	var out []*chat.Message
	for _, msg := range m.messages {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return out, nil
}



var errNotFound = &notFoundErr{}

type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "not found" }

type noopUserStore struct{}

func (n *noopUserStore) CreateUser(ctx context.Context, email, username, passwordHash string) (*auth.User, error) {
	return &auth.User{ID: 1, Email: email, Username: username}, nil
}
func (n *noopUserStore) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	return &auth.User{ID: 1, Email: email}, nil
}
func (n *noopUserStore) GetUserByID(ctx context.Context, id int64) (*auth.User, error) {
	return &auth.User{ID: id, Email: "test@example.com"}, nil
}
func (n *noopUserStore) CreateAPIKey(ctx context.Context, userID int64, keyPrefix, keyHash, name string) (*auth.APIKey, error) {
	return &auth.APIKey{ID: 1, UserID: userID, KeyPrefix: keyPrefix}, nil
}
func (n *noopUserStore) ListAPIKeys(ctx context.Context, userID int64) ([]auth.APIKey, error) {
	return nil, nil
}
func (n *noopUserStore) RevokeAPIKey(ctx context.Context, userID, keyID int64) error {
	return nil
}
func (n *noopUserStore) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*auth.APIKey, error) {
	return nil, nil
}
func (n *noopUserStore) GetAPIKeyByID(ctx context.Context, keyID int64) (*auth.APIKey, error) {
	return nil, nil
}
func (n *noopUserStore) UpdateAPIKeyLastUsed(ctx context.Context, keyID int64) error {
	return nil
}

func newChatTestServer(store *mockChatStore) *Server {
	cfg := config.Config{AdminAPIKey: "test-admin-key", JWTSecret: testJWTSecret}
	r := router.New("mock-provider", "mock-model")
	s := New(cfg, nil, nil, r, nil, nil, nil, nil, nil, nil, nil)
	s.chatStore = store
	s.providers = newMockRegistry()
	s.userStore = &noopUserStore{}
	return s
}

func chatAuthHeader() string {
	return "Bearer " + validChatToken()
}

func TestChatHandler_CreateSession(t *testing.T) {
	store := &mockChatStore{}
	s := newChatTestServer(store)
	rr := httptest.NewRecorder()

	body := `{"title":"New Chat","model":"gpt-4o-mini"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", chatAuthHeader())

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json decode error: %v", err)
	}
	if resp["title"] != "New Chat" {
		t.Fatalf("expected title 'New Chat', got %v", resp["title"])
	}
	if resp["model"] != "gpt-4o-mini" {
		t.Fatalf("expected model 'gpt-4o-mini', got %v", resp["model"])
	}
}

func TestChatHandler_CreateSession_Unauthenticated(t *testing.T) {
	store := &mockChatStore{}
	s := newChatTestServer(store)
	rr := httptest.NewRecorder()

	body := `{"title":"New Chat","model":"gpt-4o-mini"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions", bytes.NewReader([]byte(body)))

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestChatHandler_ListSessions(t *testing.T) {
	store := &mockChatStore{}
	s := newChatTestServer(store)

	store.sessions = append(store.sessions,
		&chat.Session{ID: 1, UserID: 1, Title: "Chat 1", Model: "gpt-4o-mini"},
		&chat.Session{ID: 2, UserID: 1, Title: "Chat 2", Model: "gpt-4o-mini"},
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	req.Header.Set("Authorization", chatAuthHeader())

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json decode error: %v", err)
	}
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("expected 'data' array, got %T", resp["data"])
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(data))
	}
}

func TestChatHandler_GetSession(t *testing.T) {
	store := &mockChatStore{}
	s := newChatTestServer(store)
	store.sessions = append(store.sessions, &chat.Session{ID: 1, UserID: 1, Title: "My Chat", Model: "gpt-4o-mini"})
	store.messages = append(store.messages,
		&chat.Message{ID: 1, SessionID: 1, Role: "user", Content: "Hi", Model: "gpt-4o-mini", Tokens: 5},
		&chat.Message{ID: 2, SessionID: 1, Role: "assistant", Content: "Hello!", Model: "gpt-4o-mini", Tokens: 20},
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/1", nil)
	req.Header.Set("Authorization", chatAuthHeader())

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json decode error: %v", err)
	}
	sess, ok := resp["session"].(map[string]any)
	if !ok {
		t.Fatalf("expected session object, got %v", resp["session"])
	}
	if sess["title"] != "My Chat" {
		t.Fatalf("expected title 'My Chat', got %v", sess["title"])
	}
}

func TestChatHandler_UpdateSessionTitle(t *testing.T) {
	store := &mockChatStore{}
	s := newChatTestServer(store)
	store.sessions = append(store.sessions, &chat.Session{ID: 1, UserID: 1, Title: "Old Title", Model: "gpt-4o-mini"})

	body := `{"title":"Updated Title"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/chat/sessions/1", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", chatAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
	if store.sessions[0].Title != "Updated Title" {
		t.Fatalf("expected title 'Updated Title', got %q", store.sessions[0].Title)
	}
}

func TestChatHandler_DeleteSession(t *testing.T) {
	store := &mockChatStore{}
	s := newChatTestServer(store)
	store.sessions = append(store.sessions, &chat.Session{ID: 1, UserID: 1, Title: "Delete Me", Model: "gpt-4o-mini"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/chat/sessions/1", nil)
	req.Header.Set("Authorization", chatAuthHeader())

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
	if len(store.sessions) != 0 {
		t.Fatalf("expected 0 sessions after delete, got %d", len(store.sessions))
	}
}

func TestChatHandler_CreateShareLink(t *testing.T) {
	store := &mockChatStore{}
	s := newChatTestServer(store)
	store.sessions = append(store.sessions, &chat.Session{ID: 1, UserID: 1, Title: "Share Me", Model: "gpt-4o-mini"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/1/share", nil)
	req.Header.Set("Authorization", chatAuthHeader())

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json decode error: %v", err)
	}
	if resp["share_hash"] != "testhash123" {
		t.Fatalf("expected share_hash 'testhash123', got %v", resp["share_hash"])
	}
}

func TestChatHandler_GetSharedSession_Public(t *testing.T) {
	store := &mockChatStore{}
	store.sessions = append(store.sessions, &chat.Session{ID: 1, UserID: 1, Title: "Shared", Model: "gpt-4o-mini", ShareHash: "pubhash", Visibility: "shared"})
	store.messages = append(store.messages,
		&chat.Message{ID: 1, SessionID: 1, Role: "user", Content: "Hi", Model: "gpt-4o-mini", Tokens: 5},
	)
	s := newChatTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/share/pubhash", nil)

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestChatHandler_StreamMessages(t *testing.T) {
	store := &mockChatStore{}
	s := newChatTestServer(store)
	store.sessions = append(store.sessions, &chat.Session{ID: 1, UserID: 1, Title: "Stream Test", Model: "gpt-4o-mini"})

	body := `{"content":"Hello world"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/1/messages:stream", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", chatAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", ct)
	}

	bodyOut := rr.Body.String()
	if !strings.Contains(bodyOut, `"type":"done"`) {
		t.Fatalf("expected done event in SSE response, got: %s", bodyOut)
	}

	if len(store.messages) != 2 {
		t.Fatalf("expected 2 messages (user + assistant), got %d", len(store.messages))
	}
}
