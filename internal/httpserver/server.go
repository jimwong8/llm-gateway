package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/admin"
	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/cache"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/health"
	"llm-gateway/gateway/internal/memory"
	"llm-gateway/gateway/internal/policy"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/quota"
	"llm-gateway/gateway/internal/router"
	"llm-gateway/gateway/internal/runtime"
	"llm-gateway/gateway/internal/semantic"
)

type runtimeCompensationReader interface {
	CompensationRecords() []controlplane.CompensationRecord
}

type controlplaneCompensationStore interface {
	List() []controlplane.CompensationRecord
}

type Server struct {
	cfg                           config.Config
	providers                     *providers.Registry
	cache                         cache.L1Cache
	router                        *router.Router
	audit                         *audit.Store
	semantic                      semantic.L2Cache
	memory                        *memory.Store
	billing                       *billing.Store
	quota                         *quota.Limiter
	admin                         *admin.Store
	policy                        *policy.Store
	runtimeCompensationReader     runtimeCompensationReader
	controlplaneCompensationStore controlplaneCompensationStore
	runtimeManager                *runtime.Manager
	runtimePublisher              *runtime.Publisher
	controlPlaneAdmin             *AdminHandler
}

func New(cfg config.Config, registry *providers.Registry, redisCache cache.L1Cache, rt *router.Router, auditStore *audit.Store, semanticCache semantic.L2Cache, memoryStore *memory.Store, billingStore *billing.Store, limiter *quota.Limiter, adminStore *admin.Store, policyStore *policy.Store) *Server {
	return &Server{cfg: cfg, providers: registry, cache: redisCache, router: rt, audit: auditStore, semantic: semanticCache, memory: memoryStore, billing: billingStore, quota: limiter, admin: adminStore, policy: policyStore}
}

func (s *Server) WithRuntimeCompensationReader(reader runtimeCompensationReader) *Server {
	s.runtimeCompensationReader = reader
	return s
}

func (s *Server) WithControlplaneCompensationStore(store controlplaneCompensationStore) *Server {
	s.controlplaneCompensationStore = store
	return s
}

func (s *Server) WithControlPlane(service *controlplane.Service, auditor *audit.Recorder, publisher *runtime.Publisher, manager *runtime.Manager) *Server {
	if service == nil {
		service = controlplane.NewService()
	}
	if auditor != nil {
		service.WithAuditRecorder(auditor)
	}
	if publisher != nil {
		service.WithReleasePublisher(publisher)
	}

	adminHandler := NewAdminHandler(service).WithAdminToken(s.cfg.AdminAPIKey)
	if auditor != nil {
		adminHandler.WithAuditReader(auditor)
	}
	if publisher != nil {
		adminHandler.WithRuntimeReader(publisher)
		adminHandler.WithRuntimeReplayPublisher(publisher)
		s.runtimePublisher = publisher
	}

	if manager != nil {
		s.runtimeManager = manager
		s.runtimeCompensationReader = manager
	}

	s.controlPlaneAdmin = adminHandler
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/v1/models", s.models)
	mux.HandleFunc("/v1/chat/completions", s.chatCompletions)
	mux.HandleFunc("/admin/health", s.requireAdmin(s.adminHealth))
	mux.HandleFunc("/admin/usage", s.requireAdmin(s.adminUsage))
	mux.HandleFunc("/admin/audit", s.requireAdmin(s.adminAudit))
	mux.HandleFunc("/admin/observability/summary", s.requireAdmin(s.adminObservabilitySummary))
	mux.HandleFunc("/admin/observability/cache", s.requireAdmin(s.adminObservabilityCache))
	mux.HandleFunc("/admin/observability/providers", s.requireAdmin(s.adminObservabilityProviders))
	mux.HandleFunc("/admin/observability/hotspots", s.requireAdmin(s.adminObservabilityHotspots))
	mux.HandleFunc("/admin/observability/quota", s.requireAdmin(s.adminObservabilityQuota))
	mux.HandleFunc("/admin/observability/quota/trends", s.requireAdmin(s.adminObservabilityQuotaTrends))
	mux.HandleFunc("/admin/policies/models", s.requireAdmin(s.adminPoliciesModels))
	mux.HandleFunc("/admin/assets", s.requireAdmin(s.adminAssets))
	mux.HandleFunc("/admin/assets/stats", s.requireAdmin(s.adminAssetStats))
	mux.HandleFunc("/admin/assets/reuse-audits", s.requireAdmin(s.adminAssetReuseAudits))
	mux.HandleFunc("/admin/assets/versions", s.requireAdmin(s.adminAssetVersions))
	mux.HandleFunc("/admin/assets/rollback", s.requireAdmin(s.adminAssetRollback))
	mux.HandleFunc("/admin/control-plane/compensations", s.requireAdmin(s.adminCompensations))
	s.mountControlPlaneAdminRoutes(mux)
	mux.HandleFunc("/admin/ui", s.adminUI)
	mux.HandleFunc("/admin/ui/", s.adminUI)
	mux.HandleFunc("/", s.notFound)
	return loggingMiddleware(mux)
}

func (s *Server) mountControlPlaneAdminRoutes(mux *http.ServeMux) {
	if s.controlPlaneAdmin == nil {
		return
	}
	mux.HandleFunc("/admin/inheritance-drafts", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/releases", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/releases/rollback", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/releases/replay", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/control-plane/compensations/replay", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/promotions", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/audit-events", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/runtime-events", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/config-versions", s.requireAdmin(s.controlPlaneRoute))
	mux.HandleFunc("/admin/config-versions/", s.requireAdmin(s.controlPlaneRoute))
}

func (s *Server) controlPlaneRoute(w http.ResponseWriter, r *http.Request) {
	if s.controlPlaneAdmin == nil {
		s.notFound(w, r)
		return
	}

	s.syncRuntimeManagerFromPublisher()
	proxyReq := r.Clone(r.Context())
	if strings.TrimSpace(proxyReq.Header.Get("Authorization")) == "" {
		if key := strings.TrimSpace(proxyReq.Header.Get("X-Admin-Key")); key != "" {
			proxyReq.Header.Set("Authorization", "Bearer "+key)
		}
	}
	if strings.HasPrefix(proxyReq.URL.Path, "/admin/config-versions/") {
		versionID := strings.TrimPrefix(proxyReq.URL.Path, "/admin/config-versions/")
		if versionID != "" {
			versionID = strings.SplitN(versionID, "/", 2)[0]
			proxyReq.SetPathValue("versionID", versionID)
		}
	}

	s.controlPlaneAdmin.ServeHTTP(w, proxyReq)
}

func (s *Server) syncRuntimeManagerFromPublisher() {
	if s.runtimeManager == nil || s.runtimePublisher == nil {
		return
	}
	for _, event := range s.runtimePublisher.Events() {
		module := strings.TrimSpace(event.Version.Module)
		if module == "" {
			module = "control-plane"
		}
		if current := s.runtimeManager.GetStatus(module); current.LastSeenEventVersion == event.Version.Version && current.LastSeenEventVersion != "" {
			continue
		}
		seenAt := event.Version.CreatedAt
		if seenAt.IsZero() {
			seenAt = time.Now().UTC()
		}
		s.runtimeManager.MarkEventSeen(module, event.Version.Version, seenAt)
		if current := s.runtimeManager.GetStatus(module); strings.TrimSpace(current.LastReloadStatus) == "" {
			s.runtimeManager.SetStatus(module, "ok", "")
		}
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.Header.Get("X-Admin-Key"))
		if token == "" {
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				token = strings.TrimSpace(auth[7:])
			}
		}
		if token == "" || token != s.cfg.AdminAPIKey {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "admin authentication required", "type": "authentication_error"}})
			return
		}
		if s.policy != nil {
			tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
			subject := currentSubject(r)
			if tenantID != "" && subject != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				role, err := s.policy.RoleFor(ctx, tenantID, subject)
				cancel()
				if err == nil && role != "" && !roleAllowsMethod(role, r.Method) {
					s.writeAuditAsync(audit.Event{RequestPayload: map[string]any{"tenant_id": tenantID, "policy": "admin_rbac_denied", "role": role, "subject": subject, "path": r.URL.Path}})
					writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "role not permitted for admin endpoint", "type": "authorization_error", "tenant_id": tenantID, "role": role}})
					return
				}
			}
		}
		next(w, r)
	}
}

func currentSubject(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-Subject"))
}

func roleAllowsMethod(role, method string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	switch role {
	case "admin":
		return true
	case "operator":
		return true
	case "readonly":
		return method == http.MethodGet
	default:
		return false
	}
}

func containsSensitive(req providers.ChatCompletionRequest, rules []policy.SensitiveRule) (string, bool) {
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		needle := strings.ToLower(strings.TrimSpace(rule.Pattern))
		if needle == "" {
			continue
		}
		for _, msg := range req.Messages {
			if strings.Contains(strings.ToLower(msg.Content), needle) {
				return needle, true
			}
		}
	}
	return "", false
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	cacheStatus := "disabled"
	if s.cache != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if false && ctx != nil {
			cacheStatus = "error"
		} else {
			cacheStatus = "ok"
		}
	}
	auditStatus := "disabled"
	if s.audit != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.audit.Ping(ctx); err != nil {
			auditStatus = "error"
		} else {
			auditStatus = "ok"
		}
	}
	semanticStatus := "disabled"
	if s.semantic != nil {
		semanticStatus = "ok"
	}
	memoryStatus := "disabled"
	if s.memory != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.memory.Ping(ctx); err != nil {
			memoryStatus = "error"
		} else {
			memoryStatus = "ok"
		}
	}
	billingStatus := "disabled"
	if s.billing != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.billing.Ping(ctx); err != nil {
			billingStatus = "error"
		} else {
			billingStatus = "ok"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": s.cfg.AppName, "env": s.cfg.AppEnv, "mock_mode": s.cfg.MockMode, "cache": cacheStatus, "audit": auditStatus, "semantic_cache": semanticStatus, "memory": memoryStatus, "billing": billingStatus, "time": time.Now().UTC().Format(time.RFC3339)})
}

func (s *Server) adminHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	providerStatuses := health.CheckProviders(s.cfg, s.providers)
	runtimeSummary := map[string]any{"enabled": s.runtimeManager != nil, "status_total": 0, "ok": 0, "error": 0, "skipped": 0, "latest_reload_at": ""}
	if s.runtimeManager != nil {
		statuses := s.runtimeManager.AllStatuses()
		runtimeSummary["status_total"] = len(statuses)
		latestReloadAt := time.Time{}
		okCount := 0
		errorCount := 0
		skippedCount := 0
		for _, status := range statuses {
			switch strings.ToLower(strings.TrimSpace(status.LastReloadStatus)) {
			case "ok":
				okCount++
			case "error":
				errorCount++
			case "skipped":
				skippedCount++
			}
			if status.LastReloadAt.After(latestReloadAt) {
				latestReloadAt = status.LastReloadAt
			}
		}
		runtimeSummary["ok"] = okCount
		runtimeSummary["error"] = errorCount
		runtimeSummary["skipped"] = skippedCount
		if !latestReloadAt.IsZero() {
			runtimeSummary["latest_reload_at"] = latestReloadAt.UTC().Format(time.RFC3339)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service":            s.cfg.AppName,
		"time":               time.Now().UTC().Format(time.RFC3339),
		"providers":          providerStatuses,
		"provider_summary":   health.SummarizeProviders(providerStatuses),
		"runtime_summary":    runtimeSummary,
		"compensation_stats": map[string]any{"total": s.compensationCount(), "runtime": s.runtimeCompensationCount(), "controlplane": s.controlplaneCompensationCount()},
		"admin_auth":         "enabled",
	})
}

func (s *Server) runtimeCompensationCount() int {
	if s.runtimeCompensationReader == nil {
		return 0
	}
	return len(s.runtimeCompensationReader.CompensationRecords())
}

func (s *Server) controlplaneCompensationCount() int {
	if s.controlplaneCompensationStore == nil {
		return 0
	}
	return len(s.controlplaneCompensationStore.List())
}

func (s *Server) compensationCount() int {
	return s.runtimeCompensationCount() + s.controlplaneCompensationCount()
}

func (s *Server) adminUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	limit := parseLimit(r, 20)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := s.admin.RecentUsage(ctx, limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
}

func (s *Server) adminAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	limit := parseLimit(r, 20)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := s.admin.RecentAudit(ctx, limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
}

func (s *Server) adminObservabilitySummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.billing == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "billing store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	row, err := s.billing.Summary(ctx, parseBillingFilter(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) adminObservabilityCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.billing == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "billing store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := s.billing.CacheBreakdown(ctx, parseBillingFilter(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
}

func (s *Server) adminObservabilityProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.billing == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "billing store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := s.billing.ProviderBreakdown(ctx, parseBillingFilter(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
}

func (s *Server) adminObservabilityHotspots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.billing == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "billing store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := s.billing.Hotspots(ctx, parseBillingFilter(r))
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) adminObservabilityQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.quota == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "quota limiter unavailable"})
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	row, err := s.quota.Summary(ctx, tenantID)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) adminObservabilityQuotaTrends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.quota == nil {
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
	points, err := s.quota.Trends(ctx, tenantID, windowMinutes)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "window_minutes": windowMinutes, "points": points})
}

func (s *Server) adminCompensations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	environment := strings.TrimSpace(r.URL.Query().Get("environment"))
	failedStage := strings.TrimSpace(r.URL.Query().Get("failed_stage"))
	module := strings.TrimSpace(r.URL.Query().Get("module"))
	limit := parseOptionalLimit(r)
	records := make([]controlplane.CompensationRecord, 0)
	if s.controlplaneCompensationStore != nil {
		records = append(records, s.controlplaneCompensationStore.List()...)
	}
	if s.runtimeCompensationReader != nil {
		records = append(records, s.runtimeCompensationReader.CompensationRecords()...)
	}

	filtered := make([]controlplane.CompensationRecord, 0, len(records))
	for _, record := range records {
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		if environment != "" && record.Environment != environment {
			continue
		}
		if failedStage != "" && record.FailedStage != failedStage {
			continue
		}
		if module != "" && record.Module != module {
			continue
		}
		filtered = append(filtered, record)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if !filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		}
		if filtered[i].Module != filtered[j].Module {
			return filtered[i].Module < filtered[j].Module
		}
		if filtered[i].TenantID != filtered[j].TenantID {
			return filtered[i].TenantID < filtered[j].TenantID
		}
		return filtered[i].Version < filtered[j].Version
	})

	filteredTotal := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	filterMeta := map[string]any{}
	if tenantID != "" {
		filterMeta["tenant_id"] = tenantID
	}
	if environment != "" {
		filterMeta["environment"] = environment
	}
	if failedStage != "" {
		filterMeta["failed_stage"] = failedStage
	}
	if module != "" {
		filterMeta["module"] = module
	}

	summary := map[string]any{
		"total":          len(records),
		"filtered_total": filteredTotal,
		"returned":       len(filtered),
		"filters":        filterMeta,
	}
	if limit > 0 {
		summary["limit"] = limit
	}

	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": filtered, "summary": summary})
}

func parseOptionalLimit(r *http.Request) int {
	q := strings.TrimSpace(r.URL.Query().Get("limit"))
	if q == "" {
		return 0
	}
	n, err := strconv.Atoi(q)
	if err != nil || n <= 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
}

func (s *Server) adminPoliciesModels(w http.ResponseWriter, r *http.Request) {
	if s.policy == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "policy store unavailable"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		models, err := s.policy.AllowedModels(ctx, tenantID)
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "models": models})
	case http.MethodPost:
		var body struct {
			TenantID string `json:"tenant_id"`
			Model    string `json:"model"`
			Enabled  bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		if strings.TrimSpace(body.TenantID) == "" || strings.TrimSpace(body.Model) == "" {
			badRequest(w, "tenant_id and model are required")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.policy.Upsert(ctx, body.TenantID, body.Model, body.Enabled); err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tenant_id": body.TenantID, "model": body.Model, "enabled": body.Enabled})
	default:
		methodNotAllowed(w, r)
	}
}

func (s *Server) adminAssets(w http.ResponseWriter, r *http.Request) {
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	parseBody := func(r *http.Request) (admin.AssetCreateInput, int64, string, error) {
		var body struct {
			ID              int64    `json:"id"`
			TenantID        string   `json:"tenant_id"`
			UserID          string   `json:"user_id"`
			SessionID       string   `json:"session_id"`
			SourceModel     string   `json:"source_model"`
			TaskType        string   `json:"task_type"`
			Title           string   `json:"title"`
			Summary         string   `json:"summary"`
			Tags            []string `json:"tags"`
			SourceRequestID string   `json:"source_request_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return admin.AssetCreateInput{}, 0, "", err
		}
		return admin.AssetCreateInput{TenantID: body.TenantID, UserID: body.UserID, SessionID: body.SessionID, SourceModel: body.SourceModel, TaskType: body.TaskType, Title: body.Title, Summary: body.Summary, Tags: body.Tags, SourceRequestID: body.SourceRequestID}, body.ID, body.TenantID, nil
	}
	switch r.Method {
	case http.MethodGet:
		limit := parseLimit(r, 20)
		offset := parseOffset(r)
		includeDeleted := parseBoolQuery(r, "include_deleted")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		rows, err := s.admin.ListAssets(ctx, admin.AssetFilter{TenantID: strings.TrimSpace(r.URL.Query().Get("tenant_id")), TaskType: strings.TrimSpace(r.URL.Query().Get("task_type")), SourceModel: strings.TrimSpace(r.URL.Query().Get("source_model")), Tag: strings.TrimSpace(r.URL.Query().Get("tag")), Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Limit: limit, Offset: offset, IncludeDeleted: includeDeleted})
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows, "limit": limit, "offset": offset, "include_deleted": includeDeleted})
	case http.MethodPost:
		in, _, _, err := parseBody(r)
		if err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		if strings.TrimSpace(in.TenantID) == "" || strings.TrimSpace(in.SourceModel) == "" || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Summary) == "" {
			badRequest(w, "tenant_id, source_model, title and summary are required")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		row, err := s.admin.CreateAsset(ctx, in)
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, row)
	case http.MethodPut:
		in, id, tenantID, err := parseBody(r)
		if err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		if id <= 0 || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(in.SourceModel) == "" || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Summary) == "" {
			badRequest(w, "id, tenant_id, source_model, title and summary are required")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		row, err := s.admin.UpdateAsset(ctx, id, tenantID, in)
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, row)
	case http.MethodDelete:
		id, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("id")), 10, 64)
		if err != nil || id <= 0 {
			badRequest(w, "id is required")
			return
		}
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.admin.DeleteAsset(ctx, tenantID, id); err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id, "tenant_id": tenantID})
	default:
		methodNotAllowed(w, r)
	}
}

func (s *Server) adminAssetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	limit := parseLimit(r, 20)
	includeDeleted := parseBoolQuery(r, "include_deleted")
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stats, err := s.admin.AssetStats(ctx, tenantID, includeDeleted, limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":       tenantID,
		"include_deleted": includeDeleted,
		"limit":           limit,
		"overview":        stats.Overview,
		"by_task":         stats.ByTask,
		"by_model":        stats.ByModel,
		"by_tag":          stats.ByTag,
	})
}

func (s *Server) adminAssetReuseAudits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	limit := parseLimit(r, 20)
	offset := parseOffset(r)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := s.admin.RecentAssetReuse(ctx, strings.TrimSpace(r.URL.Query().Get("tenant_id")), limit, offset)
	if err != nil {
		internalError(w, err)
		return
	}
	stats, err := s.admin.AssetStats(ctx, strings.TrimSpace(r.URL.Query().Get("tenant_id")), false, limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows, "limit": limit, "offset": offset, "stats": map[string]any{"overview": stats.Overview, "by_task": stats.ByTask}})
}

func (s *Server) adminAssetVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	assetID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("asset_id")), 10, 64)
	if err != nil || assetID <= 0 {
		badRequest(w, "asset_id is required")
		return
	}
	limit := parseLimit(r, 20)
	offset := parseOffset(r)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := s.admin.ListAssetVersions(ctx, strings.TrimSpace(r.URL.Query().Get("tenant_id")), assetID, limit, offset)
	if err != nil {
		internalError(w, err)
		return
	}
	stats, err := s.admin.AssetStats(ctx, strings.TrimSpace(r.URL.Query().Get("tenant_id")), true, limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows, "limit": limit, "offset": offset, "asset_id": assetID, "stats": map[string]any{"overview": stats.Overview, "by_model": stats.ByModel}})
}

func (s *Server) adminAssetRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if s.admin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin store unavailable"})
		return
	}
	var body struct {
		AssetID  int64  `json:"asset_id"`
		Version  int    `json:"version"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	if body.AssetID <= 0 || body.Version <= 0 || strings.TrimSpace(body.TenantID) == "" {
		badRequest(w, "asset_id, version and tenant_id are required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	row, err := s.admin.RollbackAsset(ctx, body.TenantID, body.AssetID, body.Version)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func parseBoolQuery(r *http.Request, key string) bool {
	value := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseOffset(r *http.Request) int {
	q := r.URL.Query().Get("offset")
	if q == "" {
		return 0
	}
	n, err := strconv.Atoi(q)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func parseLimit(r *http.Request, fallback int) int {
	q := r.URL.Query().Get("limit")
	if q == "" {
		return fallback
	}
	n, err := strconv.Atoi(q)
	if err != nil || n <= 0 {
		return fallback
	}
	if n > 100 {
		return 100
	}
	return n
}

func parseBillingFilter(r *http.Request) billing.QueryFilter {
	filter := billing.QueryFilter{
		TenantID: strings.TrimSpace(r.URL.Query().Get("tenant_id")),
		Provider: strings.TrimSpace(r.URL.Query().Get("provider")),
		Model:    strings.TrimSpace(r.URL.Query().Get("model")),
		Limit:    parseLimit(r, 10),
	}
	if from := strings.TrimSpace(r.URL.Query().Get("from")); from != "" {
		if ts, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = ts
		}
	}
	if to := strings.TrimSpace(r.URL.Query().Get("to")); to != "" {
		if ts, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = ts
		}
	}
	return filter
}

func (s *Server) models(w http.ResponseWriter, _ *http.Request) {
	data := make([]map[string]any, 0)
	for _, item := range s.router.Models() {
		data = append(data, map[string]any{"id": item.ID, "object": "model", "owned_by": item.Provider, "task": item.Task, "description": item.Description, "capability": item.Capability, "cost_score": item.CostScore, "latency_score": item.LatencyScore, "health_score": item.HealthScore})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	startedAt := time.Now()

	var req providers.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	if len(req.Messages) == 0 {
		badRequest(w, "messages is required")
		return
	}

	req = normalizeRequestIdentity(req, r)

	if s.policy != nil && req.TenantID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		allowedModels, err := s.policy.AllowedModels(ctx, req.TenantID)
		rules, rulesErr := s.policy.SensitiveRules(ctx, req.TenantID)
		providerPolicies, providerErr := s.policy.ProviderPolicies(ctx, req.TenantID)
		role, roleErr := s.policy.RoleFor(ctx, req.TenantID, currentSubject(r))
		cancel()
		if err != nil {
			log.Printf("policy lookup failed: %v", err)
		} else if len(allowedModels) > 0 {
			req.CandidateModels = intersectCandidateModels(req.CandidateModels, allowedModels)
			if req.PreferredModel != "" && !containsFold(allowedModels, req.PreferredModel) {
				s.writeAuditAsync(audit.Event{RequestID: requestID, RequestPayload: map[string]any{"tenant_id": req.TenantID, "policy": "preferred_model_denied", "model": req.PreferredModel}})
				writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "preferred model not allowed for tenant", "type": "policy_error", "tenant_id": req.TenantID, "model": req.PreferredModel}})
				return
			}
		}
		if roleErr == nil && role != "" && !roleAllowsMethod(role, r.Method) {
			s.writeAuditAsync(audit.Event{RequestID: requestID, RequestPayload: map[string]any{"tenant_id": req.TenantID, "policy": "rbac_denied", "role": role, "subject": currentSubject(r)}})
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "role not permitted", "type": "policy_error", "tenant_id": req.TenantID, "role": role}})
			return
		}
		if rulesErr == nil {
			if matched, ok := containsSensitive(req, rules); ok {
				s.writeAuditAsync(audit.Event{RequestID: requestID, RequestPayload: map[string]any{"tenant_id": req.TenantID, "policy": "sensitive_block", "pattern": matched, "path": r.URL.Path}})
				writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "sensitive content blocked", "type": "policy_error", "tenant_id": req.TenantID, "pattern": matched}})
				return
			}
		}
		if providerErr == nil && len(providerPolicies) > 0 {
			deniedProviders := map[string]bool{}
			for _, item := range providerPolicies {
				if item.Enabled && item.Mode == "deny" {
					deniedProviders[strings.ToLower(strings.TrimSpace(item.Provider))] = true
				}
			}
			if req.PreferredModel != "" {
				for denied := range deniedProviders {
					if strings.EqualFold(denied, req.PreferredModel) {
						s.writeAuditAsync(audit.Event{RequestID: requestID, RequestPayload: map[string]any{"tenant_id": req.TenantID, "policy": "provider_denied", "provider": denied}})
						writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "provider denied for tenant", "type": "policy_error", "tenant_id": req.TenantID, "provider": denied}})
						return
					}
				}
			}
			if len(req.CandidateModels) > 0 {
				filtered := make([]string, 0, len(req.CandidateModels))
				for _, candidate := range req.CandidateModels {
					blocked := false
					for denied := range deniedProviders {
						if strings.Contains(strings.ToLower(candidate), denied) {
							blocked = true
							break
						}
					}
					if !blocked {
						filtered = append(filtered, candidate)
					}
				}
				req.CandidateModels = filtered
				if len(req.CandidateModels) == 0 {
					s.writeAuditAsync(audit.Event{RequestID: requestID, RequestPayload: map[string]any{"tenant_id": req.TenantID, "policy": "provider_denied_all_candidates"}})
					writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "all candidate models denied by provider policy", "type": "policy_error", "tenant_id": req.TenantID}})
					return
				}
			}
		}
	}

	if s.quota != nil && req.TenantID != "" {
		allowed, used, err := s.quota.Allow(r.Context(), req.TenantID)
		if err != nil {
			log.Printf("quota check failed: %v", err)
		} else {
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", s.quota.RPM()))
			w.Header().Set("X-RateLimit-Used", fmt.Sprintf("%d", used))
			if !allowed {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": map[string]any{"message": "tenant rate limit exceeded", "type": "rate_limit_error", "tenant_id": req.TenantID}})
				return
			}
		}
	}

	memoryCount := 0
	if s.memory != nil && req.SessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		memories, err := s.memory.Recent(ctx, req.TenantID, req.UserID, req.SessionID, s.cfg.MemoryMaxItems)
		cancel()
		if err != nil {
			log.Printf("memory load failed: %v", err)
		} else if len(memories) > 0 {
			req = memory.InjectMemory(req, memories)
			memoryCount = len(memories)
			w.Header().Set("X-Memory-Injected", fmt.Sprintf("%d", memoryCount))
		}
	}

	decision := s.router.Decide(req)
	req.Model = decision.Model
	if s.admin != nil && req.TenantID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		enrichedReq, injectedAsset := s.injectAssetContext(ctx, req, decision.Task, decision.Model)
		cancel()
		if injectedAsset != nil {
			req = enrichedReq
			w.Header().Set("X-Asset-Injected", "1")
			w.Header().Set("X-Asset-Id", fmt.Sprintf("%d", injectedAsset.ID))
			w.Header().Set("X-Asset-Source", "l4_postgres")
			s.writeAssetReuseAuditAsync(requestID, req, decision.Model, decision.Task, injectedAsset.ID, "l4_postgres")
		}
	}
	w.Header().Set("X-Request-Id", requestID)
	w.Header().Set("X-Route-Mode", decision.RouteMode)
	w.Header().Set("X-Route-Task", decision.Task)
	w.Header().Set("X-Route-Model", decision.Model)
	w.Header().Set("X-Route-Provider", decision.Provider)
	if decision.Channel != "" {
		w.Header().Set("X-Route-Channel", decision.Channel)
	}
	if decision.Ability != "" {
		w.Header().Set("X-Route-Ability", decision.Ability)
	}
	w.Header().Set("X-Route-Reason", decision.Reason)
	if decision.FallbackModel != "" {
		w.Header().Set("X-Route-Fallback-Model", decision.FallbackModel)
	}
	routeScore := ""
	if len(decision.Scores) > 0 {
		routeScore = fmt.Sprintf("%.4f", decision.Scores[0].TotalScore)
		w.Header().Set("X-Route-Score", routeScore)
	}

	cacheStatus := "BYPASS"
	fallbackUsed := false
	var resp providers.ChatCompletionResponse

	if s.cache != nil {
		key, err := s.cache.BuildKey(req)
		if err != nil {
			internalError(w, err)
			return
		}
		cached, hit, err := s.cache.Get(r.Context(), key)
		if err == nil && hit {
			cacheStatus = "HIT"
			w.Header().Set("X-Cache", cacheStatus)
			s.writeAuditAsync(audit.Event{RequestID: requestID, RouteMode: decision.RouteMode, RouteTask: decision.Task, RouteModel: decision.Model, RouteProvider: decision.Provider, RouteReason: decision.Reason, RouteScore: routeScore, CacheStatus: cacheStatus, FallbackUsed: false, RequestPayload: requestToMap(req), ResponsePayload: responseToMap(*cached)})
			s.writeBillingAsync(buildUsageEvent(requestID, req, decision, decision.Provider, "HIT", "l1_exact", false, true, "", "", time.Since(startedAt), *cached))
			writeJSON(w, http.StatusOK, cached)
			return
		}
		if err != nil {
			log.Printf("redis get failed: %v", err)
		}
	}

	if s.semantic != nil {
		hit, err := s.semantic.Search(r.Context(), req)
		if err != nil {
			log.Printf("semantic search failed: %v", err)
		} else if hit != nil {
			cacheStatus = "SEMANTIC_HIT"
			w.Header().Set("X-Cache", cacheStatus)
			w.Header().Set("X-Semantic-Score", fmt.Sprintf("%.4f", hit.Score))
			s.writeAuditAsync(audit.Event{RequestID: requestID, RouteMode: decision.RouteMode, RouteTask: decision.Task, RouteModel: decision.Model, RouteProvider: decision.Provider, RouteReason: decision.Reason, RouteScore: routeScore, CacheStatus: cacheStatus, FallbackUsed: false, RequestPayload: requestToMap(req), ResponsePayload: responseToMap(hit.Response)})
			s.writeBillingAsync(buildUsageEvent(requestID, req, decision, decision.Provider, "SEMANTIC_HIT", "l2_semantic", false, true, "", "", time.Since(startedAt), hit.Response))
			writeJSON(w, http.StatusOK, hit.Response)
			return
		}
	}

	var err error
	resp, err = s.providers.ChatCompletion(r.Context(), decision.Provider, req)
	if err != nil && decision.FallbackModel != "" {
		fallbackReq := req
		fallbackReq.Model = decision.FallbackModel
		fallbackDecision := s.router.Decide(providers.ChatCompletionRequest{Model: decision.FallbackModel, Messages: req.Messages, TaskHint: decision.Task, RouteMode: "manual", PreferredModel: decision.FallbackModel})
		resp, err = s.providers.ChatCompletion(r.Context(), fallbackDecision.Provider, fallbackReq)
		if err == nil {
			fallbackUsed = true
			w.Header().Set("X-Route-Fallback-Used", "true")
			w.Header().Set("X-Route-Model", fallbackDecision.Model)
			w.Header().Set("X-Route-Provider", fallbackDecision.Provider)
			w.Header().Set("X-Route-Reason", "primary route failed, fallback model used")
			decision.Model = fallbackDecision.Model
			decision.Provider = fallbackDecision.Provider
			decision.Reason = "primary route failed, fallback model used"
		}
	}
	if err != nil {
		s.writeBillingAsync(buildUsageEvent(requestID, req, decision, decision.Provider, cacheStatus, "none", fallbackUsed, false, "provider_error", err.Error(), time.Since(startedAt), resp))
		internalError(w, err)
		return
	}

	if s.cache != nil {
		key, keyErr := s.cache.BuildKey(req)
		if keyErr == nil {
			if err := s.cache.Set(r.Context(), key, &resp); err != nil {
				log.Printf("redis set failed: %v", err)
			}
		}
	}
	if s.semantic != nil {
		go func(req providers.ChatCompletionRequest, resp providers.ChatCompletionResponse) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := s.semantic.Upsert(ctx, req, resp); err != nil {
				log.Printf("semantic upsert failed: %v", err)
			}
		}(req, resp)
	}
	if s.memory != nil && req.SessionID != "" {
		go func(req providers.ChatCompletionRequest, resp providers.ChatCompletionResponse) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			writeReq := req
			if len(resp.Choices) > 0 {
				writeReq.Messages = append(writeReq.Messages, providers.ChatMessage{Role: "assistant", Content: resp.Choices[0].Message.Content})
			}
			if err := s.memory.AppendFromRequest(ctx, writeReq); err != nil {
				log.Printf("memory append failed: %v", err)
			}
		}(req, resp)
	}
	if s.admin != nil && req.TenantID != "" {
		s.writeAssetAsync(requestID, req, decision.Task, resp)
	}

	cacheStatus = "MISS"
	w.Header().Set("X-Cache", cacheStatus)
	s.writeAuditAsync(audit.Event{RequestID: requestID, RouteMode: decision.RouteMode, RouteTask: decision.Task, RouteModel: decision.Model, RouteProvider: decision.Provider, RouteReason: decision.Reason, RouteScore: routeScore, CacheStatus: cacheStatus, FallbackUsed: fallbackUsed, RequestPayload: requestToMap(req), ResponsePayload: responseToMap(resp)})
	s.writeBillingAsync(buildUsageEvent(requestID, req, decision, decision.Provider, "MISS", "none", fallbackUsed, true, "", "", time.Since(startedAt), resp))
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) injectAssetContext(ctx context.Context, req providers.ChatCompletionRequest, taskType, sourceModel string) (providers.ChatCompletionRequest, *admin.AssetRow) {
	if s.admin == nil || strings.TrimSpace(req.TenantID) == "" {
		return req, nil
	}
	tags := deriveAssetTags(req, taskType, sourceModel)
	assets, err := s.admin.ListAssets(ctx, admin.AssetFilter{
		TenantID:    req.TenantID,
		TaskType:    strings.TrimSpace(taskType),
		SourceModel: strings.TrimSpace(sourceModel),
		Tag:         firstTag(tags),
		Keyword:     primaryKeyword(req),
		Limit:       1,
	})
	if err != nil {
		log.Printf("asset lookup failed: %v", err)
		return req, nil
	}
	if len(assets) == 0 {
		assets, err = s.admin.ListAssets(ctx, admin.AssetFilter{TenantID: req.TenantID, TaskType: strings.TrimSpace(taskType), SourceModel: strings.TrimSpace(sourceModel), Limit: 1})
		if err != nil {
			log.Printf("asset fallback lookup failed: %v", err)
			return req, nil
		}
	}
	if len(assets) == 0 {
		return req, nil
	}
	asset := assets[0]
	message := providers.ChatMessage{Role: "system", Content: buildAssetContext(asset)}
	req.Messages = append([]providers.ChatMessage{message}, req.Messages...)
	return req, &asset
}

func buildAssetContext(asset admin.AssetRow) string {
	parts := []string{"Reusable knowledge asset:", "Title: " + asset.Title, "Summary: " + asset.Summary}
	if len(asset.Tags) > 0 {
		parts = append(parts, "Tags: "+strings.Join(asset.Tags, ", "))
	}
	parts = append(parts, "Use this as background context when relevant, but prefer the current user request.")
	return strings.Join(parts, "\n")
}

func (s *Server) writeAssetReuseAuditAsync(requestID string, req providers.ChatCompletionRequest, routeModel, routeTask string, assetID int64, hitSource string) {
	if s.admin == nil || assetID <= 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.admin.RecordReuse(ctx, req.TenantID, assetID, requestID, routeModel, routeTask, hitSource); err != nil {
			log.Printf("asset reuse audit failed: %v", err)
		}
	}()
}

func (s *Server) writeAssetAsync(requestID string, req providers.ChatCompletionRequest, taskType string, resp providers.ChatCompletionResponse) {
	if s.admin == nil || strings.TrimSpace(req.TenantID) == "" || len(resp.Choices) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		summary := strings.TrimSpace(resp.Choices[0].Message.Content)
		if summary == "" {
			return
		}
		if len(summary) > 280 {
			summary = summary[:280]
		}
		title := buildAssetTitle(req)
		_, err := s.admin.CreateAsset(ctx, admin.AssetCreateInput{
			TenantID:        req.TenantID,
			UserID:          req.UserID,
			SessionID:       req.SessionID,
			SourceModel:     resp.Model,
			TaskType:        taskType,
			Title:           title,
			Summary:         summary,
			Tags:            deriveAssetTags(req, taskType, resp.Model),
			SourceRequestID: requestID,
		})
		if err != nil {
			log.Printf("asset write failed: %v", err)
		}
	}()
}

func buildAssetTitle(req providers.ChatCompletionRequest) string {
	content := primaryKeyword(req)
	if content == "" {
		return "conversation asset"
	}
	if len(content) > 80 {
		content = content[:80]
	}
	return content
}

func deriveAssetTags(req providers.ChatCompletionRequest, taskType, sourceModel string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	appendTag := func(value string) {
		value = sanitizeTag(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	appendTag(taskType)
	appendTag(sourceModel)
	for _, msg := range req.Messages {
		if msg.Role != "user" {
			continue
		}
		for _, token := range strings.Fields(strings.ToLower(msg.Content)) {
			appendTag(token)
			if len(out) >= 6 {
				return out
			}
		}
	}
	return out
}

func sanitizeTag(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	raw := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			raw = append(raw, r)
			continue
		}
		raw = append(raw, ' ')
	}
	value = strings.Join(strings.Fields(string(raw)), "-")
	if len(value) < 3 {
		return ""
	}
	if len(value) > 32 {
		value = value[:32]
	}
	return value
}

func firstTag(tags []string) string {
	for _, tag := range tags {
		if strings.TrimSpace(tag) != "" {
			return tag
		}
	}
	return ""
}

func primaryKeyword(req providers.ChatCompletionRequest) string {
	for _, msg := range req.Messages {
		if msg.Role != "user" {
			continue
		}
		text := strings.Join(strings.Fields(strings.TrimSpace(msg.Content)), " ")
		if text == "" {
			continue
		}
		if len(text) > 120 {
			text = text[:120]
		}
		return text
	}
	return ""
}

func assetMessageCount(req providers.ChatCompletionRequest) int {
	count := 0
	for _, msg := range req.Messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Reusable knowledge asset:") {
			count++
		}
	}
	return count
}

func normalizeRequestIdentity(req providers.ChatCompletionRequest, r *http.Request) providers.ChatCompletionRequest {
	req.TenantID = firstNonEmpty(strings.TrimSpace(req.TenantID), strings.TrimSpace(r.Header.Get("X-Tenant-Id")))
	req.UserID = firstNonEmpty(strings.TrimSpace(req.UserID), strings.TrimSpace(r.Header.Get("X-User-Id")))
	req.SessionID = firstNonEmpty(strings.TrimSpace(req.SessionID), strings.TrimSpace(r.Header.Get("X-Session-Id")))
	return req
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intersectCandidateModels(existing []string, allowed []string) []string {
	if len(allowed) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return allowed
	}
	out := make([]string, 0)
	for _, model := range existing {
		if containsFold(allowed, model) {
			out = append(out, model)
		}
	}
	return out
}

func containsFold(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}

func (s *Server) writeAuditAsync(event audit.Event) {
	if s.audit == nil || !s.cfg.AuditLogEnabled {
		return
	}
	go func() {
		child, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.audit.Insert(child, event); err != nil {
			log.Printf("audit insert failed: %v", err)
		}
	}()
}

func (s *Server) writeBillingAsync(event billing.UsageEvent) {
	if s.billing == nil || !s.cfg.BillingEnabled {
		return
	}
	go func() {
		child, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if event.EstimatedCost == 0 && event.TotalTokens > 0 {
			event.EstimatedCost = estimateCost(event.TotalTokens)
		}
		if err := s.billing.Insert(child, event); err != nil {
			log.Printf("billing insert failed: %v", err)
		}
	}()
}

func buildUsageEvent(requestID string, req providers.ChatCompletionRequest, decision router.Decision, provider string, cacheStatus string, cacheLayer string, fallbackUsed bool, success bool, errorType string, errorMessage string, latency time.Duration, resp providers.ChatCompletionResponse) billing.UsageEvent {
	routeProvider := decision.Provider
	if routeProvider == "" {
		routeProvider = provider
	}
	routeModel := decision.Model
	if routeModel == "" {
		routeModel = resp.Model
	}
	model := resp.Model
	if model == "" {
		model = req.Model
	}
	return billing.UsageEvent{
		TenantID:         req.TenantID,
		UserID:           req.UserID,
		RequestID:        requestID,
		Model:            model,
		Provider:         provider,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		EstimatedCost:    estimateCost(resp.Usage.TotalTokens),
		CacheStatus:      cacheStatus,
		CacheLayer:       cacheLayer,
		RouteMode:        decision.RouteMode,
		RouteProvider:    routeProvider,
		RouteModel:       routeModel,
		FallbackUsed:     fallbackUsed,
		LatencyMs:        int(latency / time.Millisecond),
		Success:          success,
		ErrorType:        errorType,
		ErrorMessage:     errorMessage,
	}
}

func estimateCost(totalTokens int) float64 { return float64(totalTokens) / 1_000_000 }

func requestToMap(req providers.ChatCompletionRequest) map[string]any {
	out := map[string]any{"model": req.Model, "route_mode": req.RouteMode, "route_channel": req.RouteChannel, "route_abilities": req.RouteAbilities, "route_policy_key": req.RoutePolicyKey, "preferred_model": req.PreferredModel, "candidate_models": req.CandidateModels, "task_hint": req.TaskHint, "session_id": req.SessionID, "user_id": req.UserID, "tenant_id": req.TenantID}
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}
	out["messages"] = msgs
	return out
}

func responseToMap(resp providers.ChatCompletionResponse) map[string]any {
	return map[string]any{"id": resp.ID, "object": resp.Object, "created": resp.Created, "model": resp.Model, "choices": resp.Choices, "usage": resp.Usage}
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) })
}
func badRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": message, "type": "invalid_request_error"}})
}
func internalError(w http.ResponseWriter, err error) {
	if err == nil {
		err = errors.New("unknown internal error")
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "resource not found", "type": "not_found_error"}})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error(), "type": "internal_server_error"}})
}
func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"message": "method not allowed", "type": "method_not_allowed", "method": r.Method}})
}
