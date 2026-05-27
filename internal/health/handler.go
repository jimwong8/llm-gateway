package health

import (
	"encoding/json"
	"net/http"
	"time"
)

// Handler 健康检查 HTTP handler
type Handler struct {
	checker *HealthChecker
}

// NewHandler 创建健康检查 handler
func NewHandler(checker *HealthChecker) *Handler {
	return &Handler{checker: checker}
}

// Healthz 返回基础健康状态（轻量级，适合 k8s liveness probe）
// GET /healthz
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	status := h.checker.Status()
	httpStatus := http.StatusOK
	if status == StatusUnhealthy {
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// HealthzDetailed 返回详细健康报告
// GET /healthz/detailed
func (h *Handler) HealthzDetailed(w http.ResponseWriter, r *http.Request) {
	report := h.checker.Report()
	httpStatus := http.StatusOK
	if report.Status == StatusUnhealthy {
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(report)
}
