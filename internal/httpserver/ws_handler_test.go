package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"llm-gateway/gateway/internal/chat"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/router"
)

func newWSTestServer(store *mockChatStore) *Server {
	cfg := config.Config{AdminAPIKey: "test-admin-key", JWTSecret: testJWTSecret}
	r := router.New("mock-provider", "mock-model")
	s := New(cfg, nil, nil, r, nil, nil, nil, nil, nil, nil, nil)
	s.chatStore = store
	s.providers = newMockRegistry()
	s.userStore = &noopUserStore{}
	return s
}

func wsURLFromServer(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http")
}

func TestWSChat_PingPong(t *testing.T) {
	store := &mockChatStore{}
	s := newWSTestServer(store)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := wsURLFromServer(ts.URL) + "/api/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+validChatToken())

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial error: %v, resp: %v", err, resp)
	}
	defer conn.Close()

	// 发送 ping
	if err := conn.WriteJSON(wsMessage{Type: "ping"}); err != nil {
		t.Fatalf("write ping error: %v", err)
	}

	// 读取 pong
	var respMsg wsResponse
	if err := conn.ReadJSON(&respMsg); err != nil {
		t.Fatalf("read pong error: %v", err)
	}
	if respMsg.Type != "pong" {
		t.Fatalf("expected type 'pong', got %q", respMsg.Type)
	}
}

func TestWSChat_ChatNewSession(t *testing.T) {
	store := &mockChatStore{}
	s := newWSTestServer(store)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := wsURLFromServer(ts.URL) + "/api/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+validChatToken())

	conn, dialResp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial error: %v, resp: %v", err, dialResp)
	}
	defer conn.Close()

	// 发送 chat 消息（无 session，触发创建新会话）
	if err := conn.WriteJSON(wsMessage{Type: "chat", Content: "Hello"}); err != nil {
		t.Fatalf("write chat error: %v", err)
	}

	// 读取 session_created
	var sessionResp wsResponse
	if err := conn.ReadJSON(&sessionResp); err != nil {
		t.Fatalf("read session_created error: %v", err)
	}
	if sessionResp.Type != "session_created" {
		t.Fatalf("expected type 'session_created', got %q", sessionResp.Type)
	}
	if sessionResp.SessionID == 0 {
		t.Fatal("expected non-zero session_id")
	}

	// 读取 done
	var doneResp wsResponse
	if err := conn.ReadJSON(&doneResp); err != nil {
		t.Fatalf("read done error: %v", err)
	}
	if doneResp.Type != "done" {
		t.Fatalf("expected type 'done', got %q (error=%s)", doneResp.Type, doneResp.Error)
	}
	if doneResp.SessionID != sessionResp.SessionID {
		t.Fatalf("session_id mismatch: created=%d, done=%d", sessionResp.SessionID, doneResp.SessionID)
	}
	if doneResp.MessageID == 0 {
		t.Fatal("expected non-zero message_id in done response")
	}

	// 验证 store 中保存了消息
	if len(store.messages) != 2 {
		t.Fatalf("expected 2 messages (user + assistant), got %d", len(store.messages))
	}
	if store.messages[0].Role != "user" || store.messages[0].Content != "Hello" {
		t.Fatalf("first message should be user 'Hello', got role=%q content=%q", store.messages[0].Role, store.messages[0].Content)
	}
	if store.messages[1].Role != "assistant" {
		t.Fatalf("second message should be assistant, got role=%q", store.messages[1].Role)
	}
}

func TestWSChat_ChatWithExistingSession(t *testing.T) {
	store := &mockChatStore{}
	// 预先创建一个 session
	store.sessions = append(store.sessions, &chat.Session{
		ID:         100,
		UserID:     1,
		Title:      "Existing Chat",
		Model:      "gpt-4o-mini",
		Visibility: "private",
	})

	s := newWSTestServer(store)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := wsURLFromServer(ts.URL) + "/api/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+validChatToken())

	conn, dialResp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial error: %v, resp: %v", err, dialResp)
	}
	defer conn.Close()

	// 发送 chat 消息，指定已有 session
	if err := conn.WriteJSON(wsMessage{Type: "chat", Content: "Follow up", Session: 100}); err != nil {
		t.Fatalf("write chat error: %v", err)
	}

	// 读取 done（不应收到 session_created）
	var doneResp wsResponse
	if err := conn.ReadJSON(&doneResp); err != nil {
		t.Fatalf("read done error: %v", err)
	}
	if doneResp.Type != "done" {
		t.Fatalf("expected type 'done', got %q (error=%s)", doneResp.Type, doneResp.Error)
	}
	if doneResp.SessionID != 100 {
		t.Fatalf("expected session_id 100, got %d", doneResp.SessionID)
	}
	if doneResp.MessageID == 0 {
		t.Fatal("expected non-zero message_id")
	}
}

func TestWSChat_Unauthenticated(t *testing.T) {
	store := &mockChatStore{}
	s := newWSTestServer(store)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := wsURLFromServer(ts.URL) + "/api/ws/chat"

	// 不携带 Authorization header
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected error on unauthenticated ws dial, got nil")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 status, got %d", resp.StatusCode)
	}
}

func TestWSChat_EmptyContent(t *testing.T) {
	store := &mockChatStore{}
	s := newWSTestServer(store)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := wsURLFromServer(ts.URL) + "/api/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+validChatToken())

	conn, dialResp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial error: %v, resp: %v", err, dialResp)
	}
	defer conn.Close()

	// 发送空 content 的 chat 消息
	if err := conn.WriteJSON(wsMessage{Type: "chat", Content: "   "}); err != nil {
		t.Fatalf("write chat error: %v", err)
	}

	// 读取 error 响应
	var errResp wsResponse
	if err := conn.ReadJSON(&errResp); err != nil {
		t.Fatalf("read error response error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("expected type 'error', got %q", errResp.Type)
	}
	if !strings.Contains(errResp.Error, "content is required") {
		t.Fatalf("expected 'content is required' error, got %q", errResp.Error)
	}
}

func TestWSChat_InvalidJSON(t *testing.T) {
	store := &mockChatStore{}
	s := newWSTestServer(store)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := wsURLFromServer(ts.URL) + "/api/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+validChatToken())

	conn, dialResp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial error: %v, resp: %v", err, dialResp)
	}
	defer conn.Close()

	// 发送无效 JSON
	if err := conn.WriteMessage(websocket.TextMessage, []byte("{invalid json")); err != nil {
		t.Fatalf("write invalid json error: %v", err)
	}

	// 读取 error 响应
	var errResp wsResponse
	if err := conn.ReadJSON(&errResp); err != nil {
		t.Fatalf("read error response error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("expected type 'error', got %q", errResp.Type)
	}
	if !strings.Contains(errResp.Error, "invalid JSON") {
		t.Fatalf("expected 'invalid JSON' error, got %q", errResp.Error)
	}
}

func TestWSChat_UnknownMessageType(t *testing.T) {
	store := &mockChatStore{}
	s := newWSTestServer(store)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := wsURLFromServer(ts.URL) + "/api/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+validChatToken())

	conn, dialResp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial error: %v, resp: %v", err, dialResp)
	}
	defer conn.Close()

	// 发送未知类型
	if err := conn.WriteJSON(wsMessage{Type: "unknown_type"}); err != nil {
		t.Fatalf("write error: %v", err)
	}

	// 读取 error 响应
	var errResp wsResponse
	if err := conn.ReadJSON(&errResp); err != nil {
		t.Fatalf("read error response error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("expected type 'error', got %q", errResp.Type)
	}
	if !strings.Contains(errResp.Error, "unknown message type") {
		t.Fatalf("expected 'unknown message type' error, got %q", errResp.Error)
	}
}

func TestWSChat_SessionNotFound(t *testing.T) {
	store := &mockChatStore{}
	s := newWSTestServer(store)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := wsURLFromServer(ts.URL) + "/api/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+validChatToken())

	conn, dialResp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial error: %v, resp: %v", err, dialResp)
	}
	defer conn.Close()

	// 请求不存在的 session
	if err := conn.WriteJSON(wsMessage{Type: "chat", Content: "Hello", Session: 9999}); err != nil {
		t.Fatalf("write chat error: %v", err)
	}

	// 读取 error 响应
	var errResp wsResponse
	if err := conn.ReadJSON(&errResp); err != nil {
		t.Fatalf("read error response error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("expected type 'error', got %q", errResp.Type)
	}
	if !strings.Contains(errResp.Error, "session not found") {
		t.Fatalf("expected 'session not found' error, got %q", errResp.Error)
	}
}

