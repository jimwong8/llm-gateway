package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"llm-gateway/gateway/internal/providers"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type wsMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Session int64  `json:"session,omitempty"`
}

type wsResponse struct {
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	SessionID int64  `json:"session_id,omitempty"`
	MessageID int64  `json:"message_id,omitempty"`
	Model     string `json:"model,omitempty"`
	Tokens    int    `json:"tokens,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *Server) handleWSChat(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"message": "not authenticated", "type": "authentication_error"},
		})
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("websocket upgrade failed", "err", err)
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	ctx := r.Context()
	var mu sync.Mutex

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("websocket read error", "err", err)
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			mu.Lock()
			conn.WriteJSON(wsResponse{Type: "error", Error: "invalid JSON"})
			mu.Unlock()
			continue
		}

		switch msg.Type {
		case "ping":
			mu.Lock()
			conn.WriteJSON(wsResponse{Type: "pong"})
			mu.Unlock()

		case "chat":
			s.handleWSChatMessage(ctx, conn, &mu, claims.UserID, msg)

		default:
			mu.Lock()
			conn.WriteJSON(wsResponse{Type: "error", Error: "unknown message type: " + msg.Type})
			mu.Unlock()
		}
	}
}

func (s *Server) handleWSChatMessage(ctx context.Context, conn *websocket.Conn, mu *sync.Mutex, userID int64, msg wsMessage) {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		mu.Lock()
		conn.WriteJSON(wsResponse{Type: "error", Error: "content is required"})
		mu.Unlock()
		return
	}

	var sessionID int64

	if msg.Session > 0 {
		// 使用已有 session
		session, err := s.chatStore.GetSession(ctx, msg.Session, userID)
		if err != nil {
			mu.Lock()
			conn.WriteJSON(wsResponse{Type: "error", Error: "session not found"})
			mu.Unlock()
			return
		}
		sessionID = session.ID
	} else {
		// 创建新 session
		session, err := s.chatStore.CreateSession(ctx, userID, "WebSocket Chat", "gpt-4o-mini")
		if err != nil {
			mu.Lock()
			conn.WriteJSON(wsResponse{Type: "error", Error: "failed to create session"})
			mu.Unlock()
			return
		}
		sessionID = session.ID
		mu.Lock()
		conn.WriteJSON(wsResponse{Type: "session_created", SessionID: sessionID})
		mu.Unlock()
	}

	// 保存 user 消息
	_, err := s.chatStore.AddMessage(ctx, sessionID, "user", content, "", 0)
	if err != nil {
		mu.Lock()
		conn.WriteJSON(wsResponse{Type: "error", Error: "failed to save message"})
		mu.Unlock()
		return
	}

	// 获取历史消息
	messages, err := s.chatStore.GetMessages(ctx, sessionID, 100, 0)
	if err != nil {
		mu.Lock()
		conn.WriteJSON(wsResponse{Type: "error", Error: "failed to get messages"})
		mu.Unlock()
		return
	}

	// 获取 session 信息以拿到 model
	session, err := s.chatStore.GetSession(ctx, sessionID, userID)
	if err != nil {
		mu.Lock()
		conn.WriteJSON(wsResponse{Type: "error", Error: "failed to get session"})
		mu.Unlock()
		return
	}

	providerMsgs := make([]providers.ChatMessage, 0, len(messages))
	for _, m := range messages {
		providerMsgs = append(providerMsgs, providers.ChatMessage{Role: m.Role, Content: m.Content})
	}

	req := providers.ChatCompletionRequest{
		Model:    session.Model,
		Messages: providerMsgs,
	}

	decision := s.router.Decide(req)

	resp, err := s.providers.ChatCompletion(ctx, decision.Provider, req)
	if err != nil {
		mu.Lock()
		conn.WriteJSON(wsResponse{Type: "error", Error: "provider error: " + err.Error()})
		mu.Unlock()
		return
	}

	assistantContent := ""
	if len(resp.Choices) > 0 {
		assistantContent = resp.Choices[0].Message.Content
	}

	assistantMsg, err := s.chatStore.AddMessage(ctx, sessionID, "assistant", assistantContent, resp.Model, resp.Usage.CompletionTokens)
	if err != nil {
		slog.Warn("failed to store assistant message", "err", err)
	}

	mu.Lock()
	conn.WriteJSON(wsResponse{
		Type:      "done",
		Content:   assistantContent,
		SessionID: sessionID,
		MessageID: assistantMsg.ID,
		Model:     resp.Model,
		Tokens:    resp.Usage.CompletionTokens,
	})
	mu.Unlock()
}

// handleWSChatHTTP 是 http.HandlerFunc 包装，用于路由注册
func (s *Server) handleWSChatHTTP(w http.ResponseWriter, r *http.Request) {
	s.handleWSChat(w, r)
}
