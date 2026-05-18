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

type broadcastUserStore interface {
	ListActive(ctx context.Context, now time.Time) ([]broadcast.Broadcast, error)
	MarkRead(ctx context.Context, broadcastID, userID int64) error
	ListReadByUser(ctx context.Context, userID int64) ([]broadcast.BroadcastRead, error)
}

type userIDExtractor interface {
	UserID(r *http.Request) int64
}

type broadcastUserIDExtractor struct{}

func (broadcastUserIDExtractor) UserID(r *http.Request) int64 {
	if c := getUserClaims(r.Context()); c != nil {
		return c.UserID
	}
	return 0
}

type BroadcastUserHandler struct {
	store      broadcastUserStore
	userIDFunc func(r *http.Request) int64
}

func NewBroadcastUserHandler(store broadcastUserStore) *BroadcastUserHandler {
	return &BroadcastUserHandler{
		store:      store,
		userIDFunc: getUserIDFromClaims,
	}
}

func getUserIDFromClaims(r *http.Request) int64 {
	if c := getUserClaims(r.Context()); c != nil {
		return c.UserID
	}
	return 0
}

func (h *BroadcastUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "broadcast store unavailable"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/user/broadcasts")
	switch {
	case path == "" || path == "/":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, r)
			return
		}
		h.handleListActive(w, r)
	default:
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) != 2 || parts[1] != "read" {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
			return
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
			return
		}
		if r.Method != http.MethodPost {
			methodNotAllowed(w, r)
			return
		}
		h.handleMarkRead(w, r, id)
	}
}

func (h *BroadcastUserHandler) handleListActive(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	rows, err := h.store.ListActive(r.Context(), now)
	if err != nil {
		internalError(w, err)
		return
	}
	userID := h.userIDFunc(r)
	var readIDs []int64
	if userID > 0 {
		reads, err := h.store.ListReadByUser(r.Context(), userID)
		if err == nil {
			readIDs = make([]int64, len(reads))
			for i, rd := range reads {
				readIDs[i] = rd.BroadcastID
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object":   "list",
		"data":     rows,
		"read_ids": readIDs,
	})
}

type markReadRequest struct {
	UserID int64 `json:"user_id"`
}

func (h *BroadcastUserHandler) handleMarkRead(w http.ResponseWriter, r *http.Request, broadcastID int64) {
	userID := h.userIDFunc(r)
	if userID == 0 {
		var body markReadRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.UserID > 0 {
			userID = body.UserID
		}
	}
	if userID == 0 {
		badRequest(w, "user authentication required")
		return
	}
	if err := h.store.MarkRead(r.Context(), broadcastID, userID); err != nil {
		if errors.Is(err, broadcast.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "broadcast not found", "type": "not_found_error"}})
			return
		}
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
