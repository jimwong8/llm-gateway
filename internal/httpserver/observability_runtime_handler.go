package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/quota"
)

// ObservabilityHandler serves /admin/observability/* endpoints.
type ObservabilityHandler struct {
	billing *billing.Store
	quota   *quota.Limiter
}

// NewObservabilityHandler creates a new ObservabilityHandler.
func NewObservabilityHandler(billing *billing.Store, quota *quota.Limiter) *ObservabilityHandler {
	return &ObservabilityHandler{billing: billing, quota: quota}
}

// ServeHTTP dispatches observability requests by path suffix.
func (h *ObservabilityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/observability")
	path = strings.TrimSuffix(path, "/")

	switch path {
	case "/latency":
		h.handleLatency(w, r)
	case "/error-rate":
		h.handleErrorRate(w, r)
	case "/summary":
		h.handleSummary(w, r)
	case "/cache":
		h.handleCache(w, r)
	case "/providers":
		h.handleProviders(w, r)
	case "/hotspots":
		h.handleHotspots(w, r)
	case "/quota":
		h.handleQuota(w, r)
	case "/quota/trends":
		h.handleQuotaTrends(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
	}
}

func (h *ObservabilityHandler) handleLatency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
			days = parsed
		}
	}
	data := make([]map[string]interface{}, days)
	now := time.Now().UTC()
	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -days+i+1)
		p50 := 120 + (i*7)%80
		p95 := 350 + (i*15)%200
		p99 := 800 + (i*30)%500
		data[i] = map[string]interface{}{
			"date": date.Format("01/02"),
			"p50":  p50,
			"p95":  p95,
			"p99":  p99,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (h *ObservabilityHandler) handleErrorRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
			days = parsed
		}
	}
	data := make([]map[string]interface{}, days)
	now := time.Now().UTC()
	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -days+i+1)
		errorRate := 0.5 + float64(i%5)*0.3
		totalRequests := 20000 + int64(i)*3000
		errorRequests := int64(float64(totalRequests) * errorRate / 100)
		data[i] = map[string]interface{}{
			"date":          date.Format("01/02"),
			"errorRate":     errorRate,
			"totalRequests": totalRequests,
			"errorRequests": errorRequests,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (h *ObservabilityHandler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.billing == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "billing store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	row, err := h.billing.Summary(ctx, parseBillingFilter(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *ObservabilityHandler) handleCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.billing == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "billing store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := h.billing.CacheBreakdown(ctx, parseBillingFilter(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
}

func (h *ObservabilityHandler) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.billing == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "billing store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := h.billing.ProviderBreakdown(ctx, parseBillingFilter(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
}

func (h *ObservabilityHandler) handleHotspots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.billing == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "billing store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := h.billing.Hotspots(ctx, parseBillingFilter(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *ObservabilityHandler) handleQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.quota == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "quota limiter unavailable"})
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	row, err := h.quota.Summary(ctx, tenantID)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *ObservabilityHandler) handleQuotaTrends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.quota == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "quota limiter unavailable"})
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	windowMinutes := 5
	if q := strings.TrimSpace(r.URL.Query().Get("window_minutes")); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			windowMinutes = n
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	points, err := h.quota.Trends(ctx, tenantID, windowMinutes)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "window_minutes": windowMinutes, "points": points})
}
