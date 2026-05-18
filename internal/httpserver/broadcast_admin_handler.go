package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/broadcast"
)

type broadcastAdminStore interface {
	Create(ctx context.Context, input broadcast.BroadcastInput, createdBy string) (*broadcast.Broadcast, error)
	List(ctx context.Context) ([]broadcast.Broadcast, error)
	GetByID(ctx context.Context, id int64) (*broadcast.Broadcast, error)
	Update(ctx context.Context, id int64, input broadcast.BroadcastInput) (*broadcast.Broadcast, error)
	Delete(ctx context.Context, id int64) error
}

type BroadcastAdminHandler struct {
	store broadcastAdminStore
}

func NewBroadcastAdminHandler(store broadcastAdminStore) *BroadcastAdminHandler {
	return &BroadcastAdminHandler{store: store}
}

func (h *BroadcastAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "broadcast store unavailable"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/admin/broadcasts")
	switch {
	case path == "" || path == "/":
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			methodNotAllowed(w, r)
		}
	default:
		idStr := strings.TrimPrefix(path, "/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
			return
		}
		switch r.Method {
		case http.MethodPut:
			h.handleUpdate(w, r, id)
		case http.MethodDelete:
			h.handleDelete(w, r, id)
		default:
			methodNotAllowed(w, r)
		}
	}
}

func (h *BroadcastAdminHandler) handleList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.List(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
}

type createBroadcastRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Type    string `json:"type"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}

func (h *BroadcastAdminHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body createBroadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Content = strings.TrimSpace(body.Content)
	if body.Title == "" || body.Content == "" {
		badRequest(w, "title and content are required")
		return
	}
	bType := broadcast.BroadcastType(strings.TrimSpace(body.Type))
	if bType == "" {
		bType = broadcast.TypeInfo
	}
	startAt, err := time.Parse(time.RFC3339, body.StartAt)
	if err != nil {
		badRequest(w, "start_at must be RFC3339 format")
		return
	}
	endAt, err := time.Parse(time.RFC3339, body.EndAt)
	if err != nil {
		badRequest(w, "end_at must be RFC3339 format")
		return
	}
	createdBy := strings.TrimSpace(r.Header.Get("X-Admin-Key"))
	if createdBy == "" {
		createdBy = "admin"
	}
	row, err := h.store.Create(r.Context(), broadcast.BroadcastInput{
		Title:   body.Title,
		Content: body.Content,
		Type:    bType,
		StartAt: startAt,
		EndAt:   endAt,
	}, createdBy)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *BroadcastAdminHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id int64) {
	var body createBroadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Content = strings.TrimSpace(body.Content)
	if body.Title == "" || body.Content == "" {
		badRequest(w, "title and content are required")
		return
	}
	bType := broadcast.BroadcastType(strings.TrimSpace(body.Type))
	if bType == "" {
		bType = broadcast.TypeInfo
	}
	startAt, err := time.Parse(time.RFC3339, body.StartAt)
	if err != nil {
		badRequest(w, "start_at must be RFC3339 format")
		return
	}
	endAt, err := time.Parse(time.RFC3339, body.EndAt)
	if err != nil {
		badRequest(w, "end_at must be RFC3339 format")
		return
	}
	row, err := h.store.Update(r.Context(), id, broadcast.BroadcastInput{
		Title:   body.Title,
		Content: body.Content,
		Type:    bType,
		StartAt: startAt,
		EndAt:   endAt,
	})
	if err != nil {
		if errors.Is(err, broadcast.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "broadcast not found", "type": "not_found_error"}})
			return
		}
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *BroadcastAdminHandler) handleDelete(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.store.Delete(r.Context(), id); err != nil {
		if errors.Is(err, broadcast.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "broadcast not found", "type": "not_found_error"}})
			return
		}
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

