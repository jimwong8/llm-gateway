package abtest

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler 封装 A/B 测试的 HTTP 处理逻辑
type Handler struct {
	service *Service
}

// NewHandler 创建 A/B 测试 HTTP 处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// ServeHTTP 实现 http.Handler 接口，按路径和方法分发
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	prefix := "/api/abtest/experiments"
	path := strings.TrimPrefix(r.URL.Path, prefix)

	switch {
	case path == "" || path == "/":
		switch r.Method {
		case http.MethodPost:
			h.createExperiment(w, r)
		case http.MethodGet:
			h.listExperiments(w, r)
		default:
			methodNotAllowed(w, r)
		}
	case strings.HasSuffix(path, "/results"):
		experimentName := strings.TrimSuffix(path, "/results")
		experimentName = strings.TrimPrefix(experimentName, "/")
		if r.Method == http.MethodGet {
			h.getResults(w, r, experimentName)
		} else {
			methodNotAllowed(w, r)
		}
	case strings.HasSuffix(path, "/assign"):
		experimentName := strings.TrimSuffix(path, "/assign")
		experimentName = strings.TrimPrefix(experimentName, "/")
		if r.Method == http.MethodPost {
			h.assignUser(w, r, experimentName)
		} else {
			methodNotAllowed(w, r)
		}
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"message": "not found",
				"type":    "not_found_error",
			},
		})
	}
}

// --- 请求/响应结构 ---

type createExperimentRequest struct {
	Name     string   `json:"name"`
	Variants []Variant `json:"variants"`
}

// --- Handlers ---

func (h *Handler) createExperiment(w http.ResponseWriter, r *http.Request) {
	var body createExperimentRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "invalid JSON", "type": "invalid_request_error"},
		})
		return
	}

	if err := h.service.CreateExperiment(body.Name, body.Variants); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "experiment created",
		"name":    body.Name,
	})
}

func (h *Handler) listExperiments(w http.ResponseWriter, r *http.Request) {
	experiments := h.service.ListExperiments()
	writeJSON(w, http.StatusOK, map[string]any{
		"experiments": experiments,
	})
}

func (h *Handler) getResults(w http.ResponseWriter, r *http.Request, experimentName string) {
	results, err := h.service.GetResults(experimentName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "not_found_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) assignUser(w http.ResponseWriter, r *http.Request, experimentName string) {
	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "invalid JSON", "type": "invalid_request_error"},
		})
		return
	}

	variant, err := h.service.AssignUser(body.UserID, experimentName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"experiment": experimentName,
		"user_id":    body.UserID,
		"variant":    variant,
	})
}

// --- 辅助函数（与 server.go 中同名函数独立，避免循环依赖） ---

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
		"error": map[string]any{
			"message": "method not allowed",
			"type":    "method_not_allowed",
			"method":  r.Method,
		},
	})
}
