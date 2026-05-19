package webhook

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler 提供 webhook 订阅的 HTTP 接口
type Handler struct {
	registry *WebhookRegistry
}

// NewHandler 创建 webhook handler
func NewHandler(registry *WebhookRegistry) *Handler {
	return &Handler{registry: registry}
}

// ServeHTTP 实现 http.Handler 接口
//
// 路由：
//
//	GET/POST /api/webhooks        → 列出/创建订阅
//	DELETE    /api/webhooks/{id}  → 删除订阅
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/webhooks")
	path = strings.TrimSpace(path)

	if path == "" || path == "/" {
		switch r.Method {
		case http.MethodGet:
			h.list(w, r)
		case http.MethodPost:
			h.create(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error": map[string]any{
					"message": "method not allowed",
					"type":    "method_not_allowed",
				},
			})
		}
		return
	}

	// /api/webhooks/{id}
	id := strings.TrimPrefix(path, "/")
	switch r.Method {
	case http.MethodDelete:
		h.delete(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{
				"message": "method not allowed",
				"type":    "method_not_allowed",
			},
		})
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	webhooks := h.registry.List()
	if webhooks == nil {
		webhooks = []Webhook{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": webhooks})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL     string   `json:"url"`
		Events  []string `json:"events"`
		Secret  string   `json:"secret"`
		Enabled bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "invalid JSON", "type": "invalid_request_error"},
		})
		return
	}
	body.URL = strings.TrimSpace(body.URL)
	if body.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "url is required", "type": "invalid_request_error"},
		})
		return
	}
	if len(body.Events) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "events is required", "type": "invalid_request_error"},
		})
		return
	}

	wh := Webhook{
		URL:     body.URL,
		Events:  body.Events,
		Secret:  body.Secret,
		Enabled: body.Enabled,
	}
	id := h.registry.Register(wh)
	wh.ID = id

	writeJSON(w, http.StatusCreated, wh)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, id string) {
	if h.registry.Unregister(id) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{
		"error": map[string]any{
			"message": "webhook not found",
			"type":    "not_found_error",
		},
	})
}

// writeJSON 是 webhook handler 的内部 JSON 响应辅助函数
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
