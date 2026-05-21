package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/health"
)

// adminRuntimeRecentRoutes 返回最近路由记录列表
func (s *Server) adminRuntimeRecentRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	usageRows, err := s.admin.RecentUsage(ctx, limit)
	if err != nil {
		internalError(w, err)
		return
	}

	type RouteEntry struct {
		RequestID     string  `json:"request_id"`
		TenantID      string  `json:"tenant_id"`
		Model         string  `json:"model"`
		Provider      string  `json:"provider"`
		TotalTokens   int     `json:"total_tokens"`
		EstimatedCost float64 `json:"estimated_cost"`
		CreatedAt     string  `json:"created_at"`
	}

	routes := make([]RouteEntry, 0, len(usageRows))
	for _, row := range usageRows {
		routes = append(routes, RouteEntry{
			RequestID:     row.RequestID,
			TenantID:      row.TenantID,
			Model:         row.Model,
			Provider:      row.Provider,
			TotalTokens:   row.TotalTokens,
			EstimatedCost: row.EstimatedCost,
			CreatedAt:     row.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   routes,
	})
}

// adminRuntimeRouteTrace 返回单条路由追踪详情
func (s *Server) adminRuntimeRouteTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	if requestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "request_id is required"})
		return
	}
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// 获取最近审计条目并按 request_id 过滤
	auditRows, err := s.admin.RecentAudit(ctx, 100)
	if err != nil {
		internalError(w, err)
		return
	}

	type TraceEntry struct {
		RequestID     string `json:"request_id"`
		RouteTask     string `json:"route_task"`
		RouteModel    string `json:"route_model"`
		RouteProvider string `json:"route_provider"`
		CacheStatus   string `json:"cache_status"`
		CreatedAt     string `json:"created_at"`
	}

	var traces []TraceEntry
	for _, row := range auditRows {
		if row.RequestID == requestID {
			traces = append(traces, TraceEntry{
				RequestID:     row.RequestID,
				RouteTask:     row.RouteTask,
				RouteModel:    row.RouteModel,
				RouteProvider: row.RouteProvider,
				CacheStatus:   row.CacheStatus,
				CreatedAt:     row.CreatedAt,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": requestID,
		"object":     "trace",
		"data":       traces,
	})
}

// adminChannelsCircuitStatus 返回各渠道熔断状态
func (s *Server) adminChannelsCircuitStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	type CircuitInfo struct {
		ChannelID    string `json:"channel_id"`
		ChannelName  string `json:"channel_name"`
		Provider     string `json:"provider"`
		Status       string `json:"status"` // "closed" | "open" | "half-open" | "unknown"
		LatencyMS    int64  `json:"latency_ms"`
		LastChecked  string `json:"last_checked"`
		ErrorMessage string `json:"error_message,omitempty"`
	}

	result := make([]CircuitInfo, 0)

	// 从 health checker 获取整体健康状态和延迟信息
	var healthStatus health.Status
	var healthLatency int64
	var healthCheckedAt time.Time

	if s.healthChecker != nil {
		report := s.healthChecker.Report()
		healthCheckedAt = report.Timestamp
		if check, ok := report.Checks["providers"]; ok {
			healthStatus = check.Status
			healthLatency = check.LatencyMS
		}
		if healthCheckedAt.IsZero() {
			healthCheckedAt = time.Now().UTC()
		}
	}

	if s.admin != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		channels, err := s.admin.ListChannels(ctx)
		if err == nil {
			for _, ch := range channels {
				// 根据 channel 自身状态映射熔断状态
				status := "unknown"
				switch strings.ToLower(ch.Status) {
				case "active":
					status = "closed"
				case "error":
					status = "open"
				case "inactive", "degraded":
					status = "half-open"
				default:
					// 如果 channel 状态不明确，使用 health checker 的整体状态
					if healthStatus == health.StatusHealthy {
						status = "closed"
					} else if healthStatus == health.StatusUnhealthy {
						status = "open"
					} else if healthStatus == health.StatusDegraded {
						status = "half-open"
					}
				}

				lat := int64(ch.LatencyMs)
				if lat <= 0 && healthLatency > 0 {
					lat = healthLatency
				}

				result = append(result, CircuitInfo{
					ChannelID:   ch.ID,
					ChannelName: ch.Name,
					Provider:    ch.Provider,
					Status:      status,
					LatencyMS:   lat,
					LastChecked: healthCheckedAt.Format(time.RFC3339),
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   result,
	})
}

// adminChannelsCircuitHistory 返回熔断状态历史
func (s *Server) adminChannelsCircuitHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	type CircuitHistoryEntry struct {
		ChannelID   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		Provider    string `json:"provider"`
		FromStatus  string `json:"from_status"`
		ToStatus    string `json:"to_status"`
		ChangedAt   string `json:"changed_at"`
		Reason      string `json:"reason,omitempty"`
	}

	// 当前返回当前状态作为单条历史记录
	// 生产环境应查询 circuit_breaker_events 表
	result := make([]CircuitHistoryEntry, 0)

	if s.admin != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		channels, err := s.admin.ListChannels(ctx)
		if err == nil {
			for _, ch := range channels {
				fromStatus := "unknown"
				toStatus := "closed"
				switch strings.ToLower(ch.Status) {
				case "error":
					toStatus = "open"
				case "inactive", "degraded":
					toStatus = "half-open"
				}
				result = append(result, CircuitHistoryEntry{
					ChannelID:   ch.ID,
					ChannelName: ch.Name,
					Provider:    ch.Provider,
					FromStatus:  fromStatus,
					ToStatus:    toStatus,
					ChangedAt:   ch.UpdatedAt,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   result,
	})
}
