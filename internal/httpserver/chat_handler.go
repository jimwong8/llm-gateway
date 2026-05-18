package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"llm-gateway/gateway/internal/chat"
	"llm-gateway/gateway/internal/providers"
)

type chatStore interface {
	CreateSession(ctx context.Context, userID int64, title, model string) (*chat.Session, error)
	GetSession(ctx context.Context, sessionID, userID int64) (*chat.Session, error)
	ListSessions(ctx context.Context, userID int64, limit, offset int) ([]*chat.Session, error)
	UpdateSessionTitle(ctx context.Context, sessionID, userID int64, title string) error
	DeleteSession(ctx context.Context, sessionID, userID int64) error
	CreateShareLink(ctx context.Context, sessionID, userID int64) (string, error)
	GetSessionByShareHash(ctx context.Context, hash string) (*chat.Session, error)
	AddMessage(ctx context.Context, sessionID int64, role, content, model string, tokens int) (*chat.Message, error)
	GetMessages(ctx context.Context, sessionID int64, limit, offset int) ([]*chat.Message, error)
}

func (s *Server) WithChatStore(store chatStore) *Server {
	s.chatStore = store
	return s
}

func (s *Server) mountChatRoutes(mux *http.ServeMux) {
	if s.chatStore == nil {
		return
	}

	mux.HandleFunc("/api/chat/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.requireUser(s.chatListSessions)(w, r)
		case http.MethodPost:
			s.requireUser(s.chatCreateSession)(w, r)
		default:
			methodNotAllowed(w, r)
		}
	})

	mux.HandleFunc("/api/chat/sessions/", s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/chat/sessions/")
		if path == "" {
			methodNotAllowed(w, r)
			return
		}

		if strings.HasSuffix(path, "/messages:stream") {
			s.chatStreamMessages(w, r)
			return
		}
		if strings.HasSuffix(path, "/share") {
			s.chatCreateShareLink(w, r)
			return
		}

		parts := strings.SplitN(path, "/", 2)
		sidStr := parts[0]
		sessionID, err := strconv.ParseInt(sidStr, 10, 64)
		if err != nil || sessionID <= 0 {
			badRequest(w, "invalid session id")
			return
		}

		claims := getUserClaims(r.Context())
		if claims == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
			return
		}

		switch r.Method {
		case http.MethodGet:
			s.chatGetSession(w, r, sessionID, claims.UserID)
		case http.MethodPut:
			s.chatUpdateSessionTitle(w, r, sessionID, claims.UserID)
		case http.MethodDelete:
			s.chatDeleteSession(w, r, sessionID, claims.UserID)
		default:
			methodNotAllowed(w, r)
		}
	}))

	mux.HandleFunc("/api/chat/share/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, r)
			return
		}
		hash := strings.TrimPrefix(r.URL.Path, "/api/chat/share/")
		s.chatGetSharedSession(w, r, hash)
	})
}

func (s *Server) chatCreateSession(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	var body struct {
		Title string `json:"title"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Model = strings.TrimSpace(body.Model)
	if body.Title == "" {
		body.Title = "New Chat"
	}
	if body.Model == "" {
		body.Model = "gpt-4o-mini"
	}

	session, err := s.chatStore.CreateSession(r.Context(), claims.UserID, body.Title, body.Model)
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) chatListSessions(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	limit := parseLimit(r, 50)
	offset := parseOffset(r)

	sessions, err := s.chatStore.ListSessions(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		internalError(w, err)
		return
	}
	if sessions == nil {
		sessions = []*chat.Session{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": sessions})
}

func (s *Server) chatGetSession(w http.ResponseWriter, r *http.Request, sessionID, userID int64) {
	session, err := s.chatStore.GetSession(r.Context(), sessionID, userID)
	if err != nil {
		internalError(w, err)
		return
	}

	messages, err := s.chatStore.GetMessages(r.Context(), sessionID, 100, 0)
	if err != nil {
		internalError(w, err)
		return
	}
	if messages == nil {
		messages = []*chat.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session":  session,
		"messages": messages,
	})
}

func (s *Server) chatUpdateSessionTitle(w http.ResponseWriter, r *http.Request, sessionID, userID int64) {
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		badRequest(w, "title is required")
		return
	}

	if err := s.chatStore.UpdateSessionTitle(r.Context(), sessionID, userID, body.Title); err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) chatDeleteSession(w http.ResponseWriter, r *http.Request, sessionID, userID int64) {
	if err := s.chatStore.DeleteSession(r.Context(), sessionID, userID); err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) chatCreateShareLink(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	sessionID, err := parseSessionIDFromPath(r.URL.Path, "/api/chat/sessions/", "/share")
	if err != nil {
		badRequest(w, "invalid session id")
		return
	}

	hash, err := s.chatStore.CreateShareLink(r.Context(), sessionID, claims.UserID)
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"share_hash": hash})
}

func (s *Server) chatGetSharedSession(w http.ResponseWriter, r *http.Request, hash string) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		badRequest(w, "share hash is required")
		return
	}

	session, err := s.chatStore.GetSessionByShareHash(r.Context(), hash)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "shared session not found", "type": "not_found_error"}})
		return
	}

	messages, err := s.chatStore.GetMessages(r.Context(), session.ID, 100, 0)
	if err != nil {
		internalError(w, err)
		return
	}
	if messages == nil {
		messages = []*chat.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session":  session,
		"messages": messages,
	})
}

func (s *Server) chatStreamMessages(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	sessionID, err := parseSessionIDFromPath(r.URL.Path, "/api/chat/sessions/", "/messages:stream")
	if err != nil {
		badRequest(w, "invalid session id")
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.Content = strings.TrimSpace(body.Content)
	if body.Content == "" {
		badRequest(w, "content is required")
		return
	}

	session, err := s.chatStore.GetSession(r.Context(), sessionID, claims.UserID)
	if err != nil {
		internalError(w, err)
		return
	}

	_, err = s.chatStore.AddMessage(r.Context(), sessionID, "user", body.Content, session.Model, 0)
	if err != nil {
		internalError(w, err)
		return
	}

	messages, err := s.chatStore.GetMessages(r.Context(), sessionID, 100, 0)
	if err != nil {
		internalError(w, err)
		return
	}

	providerMsgs := make([]providers.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		providerMsgs = append(providerMsgs, providers.ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	req := providers.ChatCompletionRequest{
		Model:    session.Model,
		Messages: providerMsgs,
	}

	decision := s.router.Decide(req)

	resp, err := s.providers.ChatCompletion(r.Context(), decision.Provider, req)
	if err != nil {
		sendSSEError(w, "provider_error", err.Error())
		return
	}

	assistantContent := ""
	if len(resp.Choices) > 0 {
		assistantContent = resp.Choices[0].Message.Content
	}

	assistantMsg, err := s.chatStore.AddMessage(r.Context(), sessionID, "assistant", assistantContent, resp.Model, resp.Usage.CompletionTokens)
	if err != nil {
		slog.Warn("failed to store assistant message", "err", err)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	words := strings.Fields(assistantContent)
	for i, word := range words {
		chunk := word
		if i < len(words)-1 {
			chunk += " "
		}

		data, _ := json.Marshal(map[string]any{
			"type":    "chunk",
			"content": chunk,
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	finalData, _ := json.Marshal(map[string]any{
		"type":          "done",
		"message_id":    assistantMsg.ID,
		"content":       assistantContent,
		"model":         resp.Model,
		"tokens":        resp.Usage.CompletionTokens,
		"prompt_tokens": resp.Usage.PromptTokens,
	})
	fmt.Fprintf(w, "data: %s\n\n", finalData)
	flusher.Flush()
}

func sendSSEError(w http.ResponseWriter, code, message string) {
	data, _ := json.Marshal(map[string]any{
		"type":    "error",
		"code":    code,
		"message": message,
	})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
}

func parseSessionIDFromPath(path, prefix, suffix string) (int64, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, suffix)
	trimmed = strings.TrimSuffix(trimmed, "/")
	return strconv.ParseInt(trimmed, 10, 64)
}
