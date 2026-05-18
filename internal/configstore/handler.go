package configstore

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler 提供配置版本管理的 HTTP 接口
type Handler struct {
	store *VersionedStore
}

// NewHandler 创建一个新的配置版本管理 Handler
func NewHandler(store *VersionedStore) *Handler {
	return &Handler{store: store}
}

// ServeHTTP 实现 http.Handler 接口，路由分发
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/config/versions":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error": map[string]any{"message": "method not allowed", "type": "method_not_allowed"},
			})
			return
		}
		h.getVersions(w, r)
	case "/api/config/rollback":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error": map[string]any{"message": "method not allowed", "type": "method_not_allowed"},
			})
			return
		}
		h.rollback(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"message": "not found", "type": "not_found"},
		})
	}
}

// getVersions GET /api/config/versions?entity_type=preset&entity_id=1&limit=20&offset=0
func (h *Handler) getVersions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	entityType := EntityType(q.Get("entity_type"))
	if entityType != EntityTypePreset && entityType != EntityTypeMask {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "entity_type must be 'preset' or 'mask'", "type": "validation_error"},
		})
		return
	}

	entityIDStr := q.Get("entity_id")
	if entityIDStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "entity_id is required", "type": "validation_error"},
		})
		return
	}
	entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
	if err != nil || entityID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "invalid entity_id", "type": "validation_error"},
		})
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	records := h.store.GetHistory(r.Context(), entityType, entityID, limit, offset)
	if records == nil {
		records = []VersionRecord{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":   records,
		"limit":  limit,
		"offset": offset,
	})
}

// rollback POST /api/config/rollback
// Body: {"version_id": 1, "actor_id": 123}
func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		VersionID int64 `json:"version_id"`
		ActorID   int64 `json:"actor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "invalid JSON", "type": "validation_error"},
		})
		return
	}
	if body.VersionID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "version_id is required", "type": "validation_error"},
		})
		return
	}
	if body.ActorID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "actor_id is required", "type": "validation_error"},
		})
		return
	}

	if err := h.store.Rollback(r.Context(), body.VersionID, body.ActorID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "rollback_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// writeJSON 辅助函数：写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
