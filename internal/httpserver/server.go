package httpserver

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	modelGovernanceAdmin          *ModelGovernanceHandler
	modelRuntime                  *ModelRuntimeHandler
	memoryAdmin                   *MemoryAdminHandler
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

func (s *Server) WithModelGovernanceHandler(handler *ModelGovernanceHandler) *Server {
	s.modelGovernanceAdmin = handler
	return s
}

func (s *Server) WithModelRuntimeHandler(handler *ModelRuntimeHandler) *Server {
	s.modelRuntime = handler
	return s
}

func (s *Server) WithMemoryAdminHandler(handler *MemoryAdminHandler) *Server {
	s.memoryAdmin = handler
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
	s.mountModelGovernanceRoutes(mux)
	s.mountModelRuntimeRoutes(mux)
	s.mountMemoryAdminRoutes(mux)
	mux.HandleFunc("/admin/ui", s.adminUI)
	mux.HandleFunc("/admin/ui/", s.adminUI)
	mux.HandleFunc("/", s.notFound)
	return panicRecoveryMiddleware(loggingMiddleware(mux))
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

func (s *Server) mountModelGovernanceRoutes(mux *http.ServeMux) {
	if s.modelGovernanceAdmin == nil {
		return
	}
	mux.HandleFunc("/admin/governance/recommendations", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/approvals", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/policy-versions", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/policy-versions/", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/rollouts", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/rollouts/", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/dashboard/rollouts", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/rollbacks", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/rollbacks/", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/evaluations", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/evaluations/", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/drifts", s.requireAdmin(s.modelGovernanceRoute))
	mux.HandleFunc("/admin/governance/drifts/", s.requireAdmin(s.modelGovernanceRoute))
}

func (s *Server) mountModelRuntimeRoutes(mux *http.ServeMux) {
	if s.modelRuntime == nil {
		return
	}
	mux.HandleFunc("/v1/runtime/resolve", s.modelRuntimeResolveRoute)
	mux.HandleFunc("/admin/governance/runtime/resolve", s.requireAdmin(s.modelRuntimeResolveRoute))
	mux.HandleFunc("/admin/governance/runtime-decisions", s.requireAdmin(s.modelRuntimeResolveRoute))
	mux.HandleFunc("/admin/governance/distribution-events", s.requireAdmin(s.modelRuntimeResolveRoute))
	mux.HandleFunc("/admin/governance/runtime-observer", s.requireAdmin(s.modelRuntimeResolveRoute))
}

func (s *Server) mountMemoryAdminRoutes(mux *http.ServeMux) {
	if s.memoryAdmin == nil {
		return
	}
	mux.HandleFunc("/admin/memory/candidate-facts", s.requireAdmin(s.memoryAdminRoute))
	mux.HandleFunc("/admin/memory/candidate-facts/", s.requireAdmin(s.memoryAdminRoute))
	mux.HandleFunc("/admin/memory/project-facts", s.requireAdmin(s.memoryAdminRoute))
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
	if versionPath, ok := strings.CutPrefix(proxyReq.URL.Path, "/admin/config-versions/"); ok {
		versionID := versionPath
		if versionID != "" {
			versionID = strings.SplitN(versionID, "/", 2)[0]
			proxyReq.SetPathValue("versionID", versionID)
		}
	}

	s.controlPlaneAdmin.ServeHTTP(w, proxyReq)
}

func (s *Server) modelGovernanceRoute(w http.ResponseWriter, r *http.Request) {
	if s.modelGovernanceAdmin == nil {
		s.notFound(w, r)
		return
	}
	s.modelGovernanceAdmin.ServeHTTP(w, r)
}

func (s *Server) modelRuntimeResolveRoute(w http.ResponseWriter, r *http.Request) {
	if s.modelRuntime == nil {
		s.notFound(w, r)
		return
	}
	s.modelRuntime.ServeHTTP(w, r)
}

func (s *Server) memoryAdminRoute(w http.ResponseWriter, r *http.Request) {
	if s.memoryAdmin == nil {
		s.notFound(w, r)
		return
	}
	s.memoryAdmin.ServeHTTP(w, r)
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
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.AdminAPIKey)) != 1 {
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
				if err == nil && role != "" && !roleAllowsAdminPath(role, r.URL.Path, r.Method) {
					s.writeAuditAsync(audit.Event{RequestPayload: map[string]any{"tenant_id": tenantID, "policy": "admin_rbac_denied", "role": role, "subject": subject, "path": r.URL.Path, "method": r.Method}})
					writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "role not permitted for admin endpoint", "type": "authorization_error", "tenant_id": tenantID, "role": role, "path": r.URL.Path, "method": r.Method}})
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

func governancePathMatches(path, base string) bool {
	path = strings.TrimSpace(path)
	base = strings.TrimSpace(base)
	if path == base {
		return true
	}
	return strings.HasPrefix(path, base+"/")
}

func roleAllowsGovernanceAction(role, path, method string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	path = strings.TrimSpace(path)
	switch role {
	case "admin":
		return true
	case "operator":
		if method == http.MethodGet {
			return true
		}
		if method != http.MethodPost {
			return false
		}
		return governancePathMatches(path, "/admin/governance/recommendations") ||
			governancePathMatches(path, "/admin/governance/evaluations") ||
			governancePathMatches(path, "/admin/governance/drifts")
	case "approver":
		if method == http.MethodGet {
			return true
		}
		if method != http.MethodPost {
			return false
		}
		return governancePathMatches(path, "/admin/governance/approvals") ||
			governancePathMatches(path, "/admin/governance/policy-versions") ||
			governancePathMatches(path, "/admin/governance/rollouts") ||
			governancePathMatches(path, "/admin/governance/rollbacks")
	case "viewer", "readonly":
		return method == http.MethodGet
	default:
		return false
	}
}

func roleAllowsAdminPath(role, path, method string) bool {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "/admin/governance/") {
		return roleAllowsGovernanceAction(role, path, method)
	}
	return roleAllowsMethod(role, method)
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

	req, sessionSource := normalizeRequestIdentity(req, r)
	w.Header().Set(sessionIDHeader, req.SessionID)
	slog.Info("session_id resolved", "source", sessionSource, "session_id", req.SessionID)

	if s.policy != nil && req.TenantID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		allowedModels, err := s.policy.AllowedModels(ctx, req.TenantID)
		rules, rulesErr := s.policy.SensitiveRules(ctx, req.TenantID)
		providerPolicies, providerErr := s.policy.ProviderPolicies(ctx, req.TenantID)
		role, roleErr := s.policy.RoleFor(ctx, req.TenantID, currentSubject(r))
		cancel()
		if err != nil {
			slog.Warn("policy lookup failed: %v", "err", err)
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
			slog.Warn("quota check failed: %v", "err", err)
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
		injectedMemoryMessages := make([]providers.ChatMessage, 0, 5)
		remainingDynamicBudget := 2800

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		// Phase L: 主链路明确依赖 GetProjectFacts 的默认语义（仅返回 active facts）。
		activeFacts, err := s.memory.GetProjectFacts(ctx, req.TenantID, req.UserID)
		cancel()
		if err != nil {
			slog.Warn("project facts load failed: %v", "err", err)
		} else if len(activeFacts) > 0 {
			activeFacts = rerankProjectFactsForRecall(activeFacts, time.Now().UTC())
			if factBlock := memory.FormatProjectFacts(activeFacts); factBlock != "" {
				injectedMemoryMessages = append(injectedMemoryMessages, providers.ChatMessage{Role: "system", Content: factBlock})
			}
		}

		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		prefs, err := s.memory.GetUserPreferences(ctx, req.TenantID, req.UserID)
		cancel()
		if err != nil {
			slog.Warn("user preferences load failed: %v", "err", err)
		} else if len(prefs) > 0 {
			prefs = rerankUserPreferencesForRecall(prefs, time.Now().UTC())
			if prefBlock := memory.FormatUserPreferences(prefs); prefBlock != "" {
				injectedMemoryMessages = append(injectedMemoryMessages, providers.ChatMessage{Role: "system", Content: prefBlock})
			}
		}

		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		summary, err := s.memory.GetSessionSummary(ctx, req.TenantID, req.UserID, req.SessionID)
		cancel()
		if err != nil {
			slog.Warn("session summary load failed: %v", "err", err)
		} else if sessionSummaryHasContent(summary) && remainingDynamicBudget > 0 {
			summaryBudget := minInt(1200, remainingDynamicBudget)
			summaryBlock := truncateRunes(memory.FormatSessionSummary(summary), summaryBudget)
			if strings.TrimSpace(summaryBlock) != "" {
				injectedMemoryMessages = append(injectedMemoryMessages, providers.ChatMessage{Role: "system", Content: summaryBlock})
				remainingDynamicBudget -= len([]rune(summaryBlock))
			}
		}

		recentLimit := clampRecentLimit(s.cfg.MemoryMaxItems, 4)
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		memories, err := s.memory.Recent(ctx, req.TenantID, req.UserID, req.SessionID, recentLimit)
		cancel()
		if err != nil {
			slog.Warn("memory load failed: %v", "err", err)
		} else if len(memories) > 0 && remainingDynamicBudget > 0 {
			recentBudget := minInt(1000, remainingDynamicBudget)
			recentBlock, injectedCount := formatRecentMemoryBlock(memories, 180, recentBudget)
			if recentBlock != "" {
				injectedMemoryMessages = append(injectedMemoryMessages, providers.ChatMessage{Role: "system", Content: recentBlock})
				memoryCount = injectedCount
				remainingDynamicBudget -= len([]rune(recentBlock))
				w.Header().Set("X-Memory-Injected", fmt.Sprintf("%d", memoryCount))
			}
		}

		if remainingDynamicBudget > 0 {
			recallBudget := minInt(600, remainingDynamicBudget)
			recallMessage, recallCount, recallRunes := s.buildLightweightHistoryRecallMessage(req, recallBudget)
			if recallMessage != nil {
				injectedMemoryMessages = append(injectedMemoryMessages, *recallMessage)
				remainingDynamicBudget -= recallRunes
				if recallCount > 0 {
					w.Header().Set("X-Memory-Recall-Injected", fmt.Sprintf("%d", recallCount))
				}
			}
		}

		req = injectAfterLeadingSystemMessages(req, injectedMemoryMessages)
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
			slog.Warn("redis get failed: %v", "err", err)
		}
	}

	if s.semantic != nil {
		hit, err := s.semantic.Search(r.Context(), req)
		if err != nil {
			slog.Warn("semantic search failed: %v", "err", err)
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
				slog.Warn("redis set failed: %v", "err", err)
			}
		}
	}
	if s.semantic != nil {
		go func(req providers.ChatCompletionRequest, resp providers.ChatCompletionResponse) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := s.semantic.Upsert(ctx, req, resp); err != nil {
				slog.Warn("semantic upsert failed: %v", "err", err)
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
				slog.Warn("memory append failed: %v", "err", err)
				return
			}
			if err := s.memory.RefreshSessionSummary(ctx, req.TenantID, req.UserID, req.SessionID); err != nil {
				slog.Warn("session summary refresh failed: %v", "err", err)
			}
			prefs := extractExplicitUserPreferences(writeReq)
			for _, pref := range prefs {
				if err := s.memory.UpsertUserPreference(ctx, pref); err != nil {
					slog.Warn("user preference upsert failed", "key", pref.Key, "err", err)
				}
			}
			facts := extractConfirmedProjectFacts(writeReq)
			for _, fact := range facts {
				if err := s.memory.UpsertProjectFact(ctx, fact); err != nil {
					slog.Warn("project fact upsert failed", "key", fact.Key, "err", err)
				}
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
		slog.Warn("asset lookup failed: %v", "err", err)
		return req, nil
	}
	if len(assets) == 0 {
		assets, err = s.admin.ListAssets(ctx, admin.AssetFilter{TenantID: req.TenantID, TaskType: strings.TrimSpace(taskType), SourceModel: strings.TrimSpace(sourceModel), Limit: 1})
		if err != nil {
			slog.Warn("asset fallback lookup failed: %v", "err", err)
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
			slog.Warn("asset reuse audit failed: %v", "err", err)
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
			slog.Warn("asset write failed: %v", "err", err)
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

func sessionSummaryHasContent(summary *memory.SessionSummary) bool {
	if summary == nil {
		return false
	}
	if strings.TrimSpace(summary.CurrentGoal) != "" {
		return true
	}
	if len(summary.CompletedItems) > 0 {
		return true
	}
	if len(summary.OpenItems) > 0 {
		return true
	}
	if len(summary.KeyDecisions) > 0 {
		return true
	}
	if len(summary.Blockers) > 0 {
		return true
	}
	return false
}

func (s *Server) buildLightweightHistoryRecallMessage(req providers.ChatCompletionRequest, runeBudget int) (*providers.ChatMessage, int, int) {
	if runeBudget <= 0 {
		return nil, 0, 0
	}
	query := latestUserMessage(req)
	if !shouldTriggerLightweightHistoryRecall(query) {
		return nil, 0, 0
	}

	const (
		searchLimit           = 6
		maxInjectedFragments  = 3
		messagesPerFragment   = 3
		messageContentRuneMax = 180
		snippetContentRuneMax = 120
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	results, err := s.memory.SearchMessages(ctx, req.TenantID, query, searchLimit, 0)
	if err != nil {
		slog.Warn("memory recall search failed: %v", "err", err)
		return nil, 0, 0
	}
	if len(results) == 0 {
		return nil, 0, 0
	}

	fragments := make([]string, 0, maxInjectedFragments)
	seenAnchors := make(map[string]struct{}, maxInjectedFragments)
	totalRunes := 0

	for _, result := range results {
		if len(fragments) >= maxInjectedFragments || totalRunes >= runeBudget {
			break
		}

		anchorKey := result.SessionID + ":" + strconv.FormatInt(result.Seq, 10)
		if _, seen := seenAnchors[anchorKey]; seen {
			continue
		}
		seenAnchors[anchorKey] = struct{}{}

		around, aroundErr := s.memory.GetMessagesAroundAnchor(ctx, result.SessionID, result.Seq, messagesPerFragment)
		if aroundErr != nil {
			slog.Warn("memory recall around-anchor failed", "session_id", result.SessionID, "seq", result.Seq, "err", aroundErr)
			continue
		}
		if len(around) == 0 {
			continue
		}

		lines := make([]string, 0, len(around)+1)
		snippet := compactText(result.Snippet, snippetContentRuneMax)
		if snippet != "" {
			lines = append(lines, "命中摘要: "+snippet)
		}
		for _, msg := range around {
			content := compactText(msg.Content, messageContentRuneMax)
			if content == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s[%d]: %s", normalizeRecallRole(msg.Role), msg.Seq, content))
		}
		if len(lines) == 0 {
			continue
		}

		fragment := fmt.Sprintf("片段 %d (session=%s, anchor_seq=%d)\n%s", len(fragments)+1, compactSessionID(result.SessionID), result.Seq, strings.Join(lines, "\n"))
		fragmentRunes := len([]rune(fragment))
		if totalRunes+fragmentRunes > runeBudget && len(fragments) > 0 {
			break
		}
		if fragmentRunes > runeBudget {
			continue
		}
		fragments = append(fragments, fragment)
		totalRunes += fragmentRunes
	}

	if len(fragments) == 0 {
		return nil, 0, 0
	}

	content := "[Lightweight History Recall · 低优先级原始历史]\n以下为与当前问题相关的历史片段（原始历史，优先级低于 active project facts / session summary / user preferences），仅在相关时参考，不要覆盖当前用户指令。\n\n" + strings.Join(fragments, "\n\n")
	content = truncateRunes(content, runeBudget)
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, 0, 0
	}

	recallMessage := &providers.ChatMessage{Role: "system", Content: content}
	return recallMessage, len(fragments), len([]rune(content))
}

func injectAfterLeadingSystemMessages(req providers.ChatCompletionRequest, injected []providers.ChatMessage) providers.ChatCompletionRequest {
	if len(injected) == 0 {
		return req
	}
	if len(req.Messages) == 0 {
		req.Messages = append(req.Messages, injected...)
		return req
	}

	idx := 0
	for idx < len(req.Messages) && strings.EqualFold(req.Messages[idx].Role, "system") {
		idx++
	}
	out := make([]providers.ChatMessage, 0, len(req.Messages)+len(injected))
	out = append(out, req.Messages[:idx]...)
	out = append(out, injected...)
	out = append(out, req.Messages[idx:]...)
	req.Messages = out
	return req
}

func formatRecentMemoryBlock(items []memory.Item, maxItemRunes int, runeBudget int) (string, int) {
	if len(items) == 0 || runeBudget <= 0 {
		return "", 0
	}
	lines := make([]string, 0, len(items))
	used := 0
	injected := 0
	for i := len(items) - 1; i >= 0; i-- {
		content := compactText(items[i].Content, maxItemRunes)
		if content == "" {
			continue
		}
		line := fmt.Sprintf("- %s: %s", items[i].Role, content)
		lineRunes := len([]rune(line))
		if used+lineRunes > runeBudget {
			break
		}
		lines = append(lines, line)
		used += lineRunes
		injected++
	}
	if len(lines) == 0 {
		return "", 0
	}
	message := "Session memory:\n" + strings.Join(lines, "\n")
	message = truncateRunes(message, runeBudget)
	message = strings.TrimSpace(message)
	if message == "" {
		return "", 0
	}
	return message, injected
}

func clampRecentLimit(raw int, fallback int) int {
	if raw <= 0 {
		return fallback
	}
	if raw > 8 {
		return 8
	}
	return raw
}

func truncateRunes(s string, limit int) string {
	s = strings.TrimSpace(s)
	if s == "" || limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func latestUserMessage(req providers.ChatCompletionRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if !strings.EqualFold(req.Messages[i].Role, "user") {
			continue
		}
		content := strings.TrimSpace(req.Messages[i].Content)
		if content != "" {
			return content
		}
	}
	return ""
}

func shouldTriggerLightweightHistoryRecall(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	length := len([]rune(query))
	if length >= 120 {
		return true
	}
	if length < 12 {
		return false
	}
	queryLower := strings.ToLower(query)
	signals := []string{"继续", "刚才", "之前", "回到", "那个", "上次", "历史", "前面", "continue", "previous", "earlier", "back to", "last time", "history"}
	for _, signal := range signals {
		if strings.Contains(queryLower, strings.ToLower(signal)) {
			return true
		}
	}
	return false
}

func normalizeRecallRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "user", "assistant", "system":
		return role
	default:
		return "message"
	}
}

func compactText(text string, maxRunes int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}
	if maxRunes > 0 {
		runes := []rune(text)
		if len(runes) > maxRunes {
			text = string(runes[:maxRunes]) + "…"
		}
	}
	return text
}

func compactSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if len(sessionID) <= 18 {
		return sessionID
	}
	return sessionID[:9] + "..." + sessionID[len(sessionID)-6:]
}

func rerankProjectFactsForRecall(facts []memory.ProjectFact, now time.Time) []memory.ProjectFact {
	if len(facts) <= 1 {
		return facts
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out := make([]memory.ProjectFact, 0, len(facts))
	for _, fact := range facts {
		if strings.TrimSpace(fact.Key) == "" || strings.TrimSpace(fact.Value) == "" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(fact.Status))
		if status != "" && status != "active" {
			continue
		}
		out = append(out, fact)
	}
	sort.SliceStable(out, func(i, j int) bool {
		si := projectFactRecallScore(out[i], now)
		sj := projectFactRecallScore(out[j], now)
		if si != sj {
			return si > sj
		}
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		if !out[i].LastVerifiedAt.Equal(out[j].LastVerifiedAt) {
			return out[i].LastVerifiedAt.After(out[j].LastVerifiedAt)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func projectFactRecallScore(fact memory.ProjectFact, now time.Time) int {
	score := 0
	status := strings.ToLower(strings.TrimSpace(fact.Status))
	if status == "" || status == "active" {
		score += 400
	} else {
		score -= 300
	}
	if strings.TrimSpace(fact.SourceText) != "" && isConfirmedProjectFactSignal(fact.SourceText) {
		score += 220
	}
	if strings.TrimSpace(fact.SourceText) != "" && !isTentativeProjectFactSignal(fact.SourceText) {
		score += 120
	}

	if !fact.LastVerifiedAt.IsZero() {
		age := now.Sub(fact.LastVerifiedAt)
		switch {
		case age <= 30*24*time.Hour:
			score += 220
		case age <= 90*24*time.Hour:
			score += 120
		case age <= 180*24*time.Hour:
			score += 40
		default:
			score -= 180
		}
	}
	if !fact.UpdatedAt.IsZero() {
		age := now.Sub(fact.UpdatedAt)
		switch {
		case age <= 7*24*time.Hour:
			score += 120
		case age <= 30*24*time.Hour:
			score += 70
		case age <= 90*24*time.Hour:
			score += 20
		default:
			score -= 40
		}
	}
	return score
}

func rerankUserPreferencesForRecall(prefs []memory.UserPreference, now time.Time) []memory.UserPreference {
	if len(prefs) <= 1 {
		return prefs
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out := make([]memory.UserPreference, 0, len(prefs))
	for _, pref := range prefs {
		if strings.TrimSpace(pref.Key) == "" || strings.TrimSpace(pref.Value) == "" {
			continue
		}
		out = append(out, pref)
	}
	sort.SliceStable(out, func(i, j int) bool {
		si := userPreferenceRecallScore(out[i], now)
		sj := userPreferenceRecallScore(out[j], now)
		if si != sj {
			return si > sj
		}
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func userPreferenceRecallScore(pref memory.UserPreference, now time.Time) int {
	score := 0
	if strings.TrimSpace(pref.SourceText) != "" {
		score += 180
	}
	if !pref.UpdatedAt.IsZero() {
		age := now.Sub(pref.UpdatedAt)
		switch {
		case age <= 7*24*time.Hour:
			score += 140
		case age <= 30*24*time.Hour:
			score += 80
		case age <= 180*24*time.Hour:
			score += 20
		default:
			score -= 30
		}
	}
	return score
}

func extractExplicitUserPreferences(req providers.ChatCompletionRequest) []memory.UserPreference {
	if strings.TrimSpace(req.UserID) == "" {
		return nil
	}

	byKey := map[string]memory.UserPreference{}
	for _, msg := range req.Messages {
		if !strings.EqualFold(msg.Role, "user") {
			continue
		}
		sourceText := normalizePreferenceSignalText(msg.Content)
		if sourceText == "" {
			continue
		}

		if value, ok := extractExplicitLanguagePreference(sourceText); ok {
			byKey["language"] = memory.UserPreference{TenantID: req.TenantID, UserID: req.UserID, Key: "language", Value: value, SourceText: sourceText}
		}
		if value, ok := extractExplicitVerbosityPreference(sourceText); ok {
			byKey["verbosity"] = memory.UserPreference{TenantID: req.TenantID, UserID: req.UserID, Key: "verbosity", Value: value, SourceText: sourceText}
		}
		if value, ok := extractExplicitConfirmationPreference(sourceText); ok {
			byKey["confirmation"] = memory.UserPreference{TenantID: req.TenantID, UserID: req.UserID, Key: "confirmation", Value: value, SourceText: sourceText}
		}
	}

	orderedKeys := []string{"language", "verbosity", "confirmation"}
	out := make([]memory.UserPreference, 0, len(byKey))
	for _, key := range orderedKeys {
		pref, ok := byKey[key]
		if ok {
			out = append(out, pref)
		}
	}
	return out
}

func extractConfirmedProjectFacts(req providers.ChatCompletionRequest) []memory.ProjectFact {
	if strings.TrimSpace(req.UserID) == "" {
		return nil
	}

	byKey := map[string]memory.ProjectFact{}
	for _, msg := range req.Messages {
		if !strings.EqualFold(msg.Role, "user") && !strings.EqualFold(msg.Role, "assistant") {
			continue
		}
		sourceText := normalizePreferenceSignalText(msg.Content)
		if sourceText == "" {
			continue
		}
		key, value, ok := extractProjectFactKV(sourceText)
		if !ok {
			continue
		}
		if !isConfirmedProjectFactSignal(sourceText) {
			continue
		}
		if isTentativeProjectFactSignal(sourceText) {
			continue
		}
		byKey[key] = memory.ProjectFact{
			TenantID:   req.TenantID,
			UserID:     req.UserID,
			Key:        key,
			Value:      value,
			SourceText: sourceText,
		}
	}

	orderedKeys := []string{"pg_truth", "redis_role", "oracle_review_mode"}
	out := make([]memory.ProjectFact, 0, len(byKey))
	for _, key := range orderedKeys {
		fact, ok := byKey[key]
		if ok {
			out = append(out, fact)
		}
	}
	return out
}

func extractProjectFactKV(content string) (string, string, bool) {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return "", "", false
	}

	pgTruthSignals := []string{"pg is truth", "postgres is truth", "postgresql is truth", "pg作为truth", "pg 是 truth", "以 pg 为准", "postgres 为准", "postgres 是事实来源"}
	if hasAnySignal(lower, pgTruthSignals) {
		return "pg_truth", "PG is Truth", true
	}
	redisHotSignals := []string{"redis 只做热层", "redis只做热层", "redis only for hot layer", "redis only as hot cache", "redis 仅做缓存热层", "redis 仅作热层"}
	if hasAnySignal(lower, redisHotSignals) {
		return "redis_role", "Redis 只做热层", true
	}
	oracleParallelSignals := []string{"oracle 审查默认拆小并行", "oracle审查默认拆小并行", "oracle review defaults to small parallel tasks", "oracle review default split into parallel"}
	if hasAnySignal(lower, oracleParallelSignals) {
		return "oracle_review_mode", "Oracle 审查默认拆小并行", true
	}
	return "", "", false
}

func isConfirmedProjectFactSignal(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return false
	}
	confirmedSignals := []string{"已确认", "已定", "最终决定", "已经落地", "确定采用", "结论：", "confirm", "confirmed", "decided", "we use", "is truth", "只做", "默认"}
	return hasAnySignal(lower, confirmedSignals)
}

func isTentativeProjectFactSignal(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return false
	}
	tentativeSignals := []string{"考虑", "候选", "可能", "试试", "暂定", "先这样", "maybe", "might", "proposal", "proposed", "option", "候选方案", "讨论"}
	return hasAnySignal(lower, tentativeSignals)
}

func normalizePreferenceSignalText(content string) string {
	content = strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if content == "" {
		return ""
	}
	runes := []rune(content)
	if len(runes) > 240 {
		content = string(runes[:240])
	}
	return content
}

func extractExplicitLanguagePreference(content string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(content))
	explicitSignals := []string{
		"以后都用中文", "以后用中文", "今后都用中文", "后续都用中文", "之后都用中文",
		"请一直用中文", "请始终用中文", "默认用中文", "都用中文回答", "一直用中文回答",
		"from now on", "always use chinese", "respond in chinese",
	}
	if hasAnySignal(lower, explicitSignals) {
		return "zh-CN", true
	}
	return "", false
}

func extractExplicitVerbosityPreference(content string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(content))
	negativeSignals := []string{"不要简短", "别太简短", "不要太简洁", "详细一点", "说详细", "展开讲"}
	if hasAnySignal(lower, negativeSignals) {
		return "", false
	}
	explicitSignals := []string{
		"回答简洁", "回答简短", "请简洁", "请简短", "简洁一点", "简短一点", "尽量简洁", "尽量简短",
		"be concise", "keep it brief",
	}
	if hasAnySignal(lower, explicitSignals) {
		return "low", true
	}
	return "", false
}

func extractExplicitConfirmationPreference(content string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(content))
	explicitSignals := []string{
		"不要频繁确认", "不用频繁确认", "不要每次都确认", "不用每次确认", "减少确认", "少确认",
		"不要总是确认", "不要反复确认", "don't ask for confirmation too often", "minimal confirmation",
	}
	if hasAnySignal(lower, explicitSignals) {
		return "minimal", true
	}
	return "", false
}

func hasAnySignal(content string, signals []string) bool {
	for _, signal := range signals {
		signal = strings.TrimSpace(strings.ToLower(signal))
		if signal == "" {
			continue
		}
		if strings.Contains(content, signal) {
			return true
		}
	}
	return false
}

func normalizeRequestIdentity(req providers.ChatCompletionRequest, r *http.Request) (providers.ChatCompletionRequest, sessionIDSource) {
	req.TenantID = firstNonEmpty(strings.TrimSpace(req.TenantID), strings.TrimSpace(r.Header.Get("X-Tenant-Id")))
	req.UserID = firstNonEmpty(strings.TrimSpace(req.UserID), strings.TrimSpace(r.Header.Get("X-User-Id")))
	var source sessionIDSource
	req.SessionID, source = resolveOrCreateSessionID(req.SessionID, r)
	return req, source
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
			slog.Warn("audit insert failed: %v", "err", err)
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
			slog.Warn("billing insert failed: %v", "err", err)
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

func panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Warn("panic recovered", "path", r.URL.Path, "method", r.Method, "err", rec)
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "internal server error", "type": "internal_server_error"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
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
