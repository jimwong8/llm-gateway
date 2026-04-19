package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"llm-gateway/gateway/internal/governance"
)

type runtimeResolver interface {
	Resolve(ctx context.Context, input governance.ResolveInput) (governance.ResolveDecision, error)
}

type ModelRuntimeHandler struct {
	resolver runtimeResolver
	queryer  governanceSQLQueryer
}

func NewModelRuntimeHandler() *ModelRuntimeHandler {
	return &ModelRuntimeHandler{}
}

func (h *ModelRuntimeHandler) WithResolver(resolver runtimeResolver) *ModelRuntimeHandler {
	h.resolver = resolver
	return h
}

func (h *ModelRuntimeHandler) WithQueryer(queryer governanceSQLQueryer) *ModelRuntimeHandler {
	h.queryer = queryer
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
