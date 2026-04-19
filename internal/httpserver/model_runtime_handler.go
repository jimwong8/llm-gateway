package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"llm-gateway/gateway/internal/governance"
)

type runtimeResolver interface {
	Resolve(ctx context.Context, input governance.ResolveInput) (governance.ResolveDecision, error)
	ObserverSnapshot() governance.RuntimeResolverObserverSnapshot
}

type ModelRuntimeHandler struct {
	resolver runtimeResolver
	queryer  governanceSQLQueryer
	timeNow  func() time.Time
}

func NewModelRuntimeHandler() *ModelRuntimeHandler {
	return &ModelRuntimeHandler{timeNow: time.Now}
}

func (h *ModelRuntimeHandler) WithResolver(resolver runtimeResolver) *ModelRuntimeHandler {
	h.resolver = resolver
	return h
}

func (h *ModelRuntimeHandler) WithQueryer(queryer governanceSQLQueryer) *ModelRuntimeHandler {
	h.queryer = queryer
	return h
}

func (h *ModelRuntimeHandler) WithTimeNow(now func() time.Time) *ModelRuntimeHandler {
	if now != nil {
		h.timeNow = now
	}
	return h
}

func (h *ModelRuntimeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/runtime/resolve", "/admin/governance/runtime/resolve":
		h.handleResolve(w, r)
	case "/admin/governance/runtime-decisions":
		h.handleRuntimeDecisions(w, r)
	case "/admin/governance/distribution-events":
		h.handleDistributionEvents(w, r)
	case "/admin/governance/runtime-observer":
		h.handleRuntimeObserver(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
	}
}

type resolveRuntimeRequest struct {
	RequestID           string `json:"request_id"`
	TenantID            string `json:"tenant_id"`
	Environment         string `json:"environment"`
	AgentID             string `json:"agent_id"`
	TaskType            string `json:"task_type"`
	SystemFallbackModel string `json:"system_fallback_model"`
	RolloutID           string `json:"rollout_id"`
}

func (h *ModelRuntimeHandler) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if h.resolver == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "runtime resolver unavailable"})
		return
	}
	var req resolveRuntimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	decision, err := h.resolver.Resolve(r.Context(), governance.ResolveInput{
		RequestID:           strings.TrimSpace(req.RequestID),
		TenantID:            strings.TrimSpace(req.TenantID),
		Environment:         strings.TrimSpace(req.Environment),
		AgentID:             strings.TrimSpace(req.AgentID),
		TaskType:            strings.TrimSpace(req.TaskType),
		SystemFallbackModel: strings.TrimSpace(req.SystemFallbackModel),
		RolloutID:           strings.TrimSpace(req.RolloutID),
	})
	if err != nil {
		writeJSON(w, runtimeResolveErrorStatus(err), map[string]any{"error": map[string]any{"message": err.Error(), "type": "runtime_resolve_error"}})
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (h *ModelRuntimeHandler) handleRuntimeDecisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.queryer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "governance query unavailable"})
		return
	}
	rows, err := h.queryer.QueryContext(r.Context(), `
SELECT request_id,
       policy_version_id,
       rollout_id,
       environment,
       tenant_id,
       agent_id,
       task_type,
       matched_scope_type,
       matched_scope,
       resolved_model,
       fallback_chain,
       policy_fallback_used,
       system_fallback_used,
       latency_ms,
       success,
       error_type,
       error_message,
       created_at
FROM runtime_decision_snapshots
ORDER BY created_at DESC
LIMIT $1
`, parseLimit(r, 20))
	if err != nil {
		internalError(w, err)
		return
	}
	defer rows.Close()
	data, err := scanRowsToMaps(rows)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (h *ModelRuntimeHandler) handleDistributionEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.queryer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "governance query unavailable"})
		return
	}
	rows, err := h.queryer.QueryContext(r.Context(), `
SELECT event_id,
       policy_version_id,
       rollout_id,
       environment,
       event_type,
       payload,
       created_at
FROM model_distribution_events
ORDER BY created_at DESC
LIMIT $1
`, parseLimit(r, 20))
	if err != nil {
		internalError(w, err)
		return
	}
	defer rows.Close()
	data, err := scanRowsToMaps(rows)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (h *ModelRuntimeHandler) handleRuntimeObserver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.resolver == nil || h.queryer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "runtime observer unavailable"})
		return
	}

	environment := strings.TrimSpace(r.URL.Query().Get("environment"))
	if environment == "" {
		environment = "prod"
	}
	limit := parseLimit(r, 20)

	activePolicyVersionID, activePolicyUpdatedAt, err := h.queryActivePolicy(r.Context(), environment)
	if err != nil {
		internalError(w, err)
		return
	}

	snapshot := h.resolver.ObserverSnapshot()
	cacheEntries := make([]governance.RuntimeResolverCacheEntry, 0, len(snapshot.CacheEntries))
	for _, entry := range snapshot.CacheEntries {
		if environment != "" && !strings.EqualFold(entry.Environment, environment) {
			continue
		}
		cacheEntries = append(cacheEntries, entry)
	}
	sort.Slice(cacheEntries, func(i, j int) bool {
		return cacheEntries[i].CachedAt.After(cacheEntries[j].CachedAt)
	})

	runtimeFacts, err := h.listRuntimeDecisionFacts(r.Context(), environment, limit)
	if err != nil {
		internalError(w, err)
		return
	}
	distributionFacts, err := h.listDistributionFacts(r.Context(), environment, limit)
	if err != nil {
		internalError(w, err)
		return
	}

	now := time.Now().UTC()
	if h.timeNow != nil {
		now = h.timeNow().UTC()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"environment": environment,
		"observed_at": now,
		"active_policy": map[string]any{
			"version_id": activePolicyVersionID,
			"updated_at": activePolicyUpdatedAt,
		},
		"cache": map[string]any{
			"entry_count":                  len(cacheEntries),
			"entries":                      cacheEntries,
			"invalidation_count":           snapshot.CacheInvalidationCount,
			"last_invalidated_at":          snapshot.LastInvalidatedAt,
			"last_invalidated_environment": snapshot.LastInvalidatedEnvironment,
		},
		"facts": map[string]any{
			"runtime_decisions":   runtimeFacts,
			"distribution_events": distributionFacts,
		},
	})
}

func (h *ModelRuntimeHandler) queryActivePolicy(ctx context.Context, environment string) (string, time.Time, error) {
	rows, err := h.queryer.QueryContext(ctx, `
SELECT policy_version_id,
       COALESCE(activated_at, approved_at, created_at) AS updated_at
FROM model_policy_versions
WHERE environment = $1 AND status = 'active'
ORDER BY COALESCE(activated_at, approved_at, created_at) DESC, id DESC
LIMIT 1
`, environment)
	if err != nil {
		return "", time.Time{}, err
	}
	defer rows.Close()
	data, err := scanRowsToMaps(rows)
	if err != nil {
		return "", time.Time{}, err
	}
	if len(data) == 0 {
		return "", time.Time{}, nil
	}
	versionID, _ := data[0]["policy_version_id"].(string)
	updatedAt, _ := data[0]["updated_at"].(time.Time)
	return strings.TrimSpace(versionID), updatedAt.UTC(), nil
}

func (h *ModelRuntimeHandler) listRuntimeDecisionFacts(ctx context.Context, environment string, limit int) ([]map[string]any, error) {
	rows, err := h.queryer.QueryContext(ctx, `
SELECT request_id,
       policy_version_id,
       rollout_id,
       resolved_model,
       matched_scope_type,
       success,
       created_at
FROM runtime_decision_snapshots
WHERE ($1 = '' OR environment = $1)
ORDER BY created_at DESC
LIMIT $2
`, environment, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRowsToMaps(rows)
}

func (h *ModelRuntimeHandler) listDistributionFacts(ctx context.Context, environment string, limit int) ([]map[string]any, error) {
	rows, err := h.queryer.QueryContext(ctx, `
SELECT event_id,
       policy_version_id,
       rollout_id,
       event_type,
       payload,
       created_at
FROM model_distribution_events
WHERE ($1 = '' OR environment = $1)
ORDER BY created_at DESC
LIMIT $2
`, environment, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRowsToMaps(rows)
}

func runtimeResolveErrorStatus(err error) int {
	switch {
	case errors.Is(err, governance.ErrActivePolicyNotFound), errors.Is(err, governance.ErrNoMatchingPolicyScope):
		return http.StatusNotFound
	case errors.Is(err, governance.ErrNoResolvedModel):
		return http.StatusBadRequest
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "required") || strings.Contains(message, "invalid") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

var _ governanceSQLQueryer = (*sql.DB)(nil)
