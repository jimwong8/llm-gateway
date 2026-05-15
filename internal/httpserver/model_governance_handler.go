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

type recommendationGenerator interface {
	Generate(ctx context.Context, input governance.GenerateRecommendationInput) (governance.Recommendation, error)
}

type approvalDecider interface {
	Decide(ctx context.Context, input governance.ApprovalInput) (governance.Approval, error)
}

type versionLifecycle interface {
	CreateFromApproval(ctx context.Context, approvalID, createdBy string) (governance.PolicyVersion, error)
	Approve(ctx context.Context, versionID, approvedBy string) (governance.PolicyVersion, error)
	Activate(ctx context.Context, versionID string) (governance.PolicyVersion, error)
	GetDiff(ctx context.Context, versionID string) (governance.PolicyVersionDiff, error)
}

type rolloutCoordinator interface {
	Start(ctx context.Context, input governance.StartRolloutInput) (governance.Rollout, governance.DistributionEvent, error)
	Promote(ctx context.Context, input governance.PromoteRolloutInput) (governance.Rollout, error)
}

type rollbackExecutor interface {
	Execute(ctx context.Context, input governance.ExecuteRollbackInput) (governance.ExecuteRollbackResult, error)
}

type rollbackRecordStore interface {
	Create(ctx context.Context, input governance.ExecuteRollbackInput, result governance.ExecuteRollbackResult) (governance.RollbackRecord, error)
	List(ctx context.Context, limit int) ([]governance.RollbackRecord, error)
	Get(ctx context.Context, rollbackID string) (governance.RollbackRecord, error)
}

type evaluationRunner interface {
	StartRun(ctx context.Context, input governance.StartEvaluationRunInput) (governance.EvaluationRun, error)
	UpdateRunStatus(ctx context.Context, runID string, status governance.EvaluationRunStatus) (governance.EvaluationRun, error)
}

type driftManager interface {
	DetectModelMismatch(ctx context.Context, input governance.DetectDriftInput) (governance.PolicyDrift, bool, error)
	Acknowledge(ctx context.Context, driftID, reason string) (governance.PolicyDrift, error)
	Resolve(ctx context.Context, driftID, reason string) (governance.PolicyDrift, error)
}

type governanceSQLQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type rolloutDashboardReader interface {
	ListRows(ctx context.Context, query governance.RolloutDashboardQuery) ([]governance.RolloutDashboardRow, error)
}

type ModelGovernanceHandler struct {
	recommendations  recommendationGenerator
	approvals        approvalDecider
	versions         versionLifecycle
	rollouts         rolloutCoordinator
	rolloutDashboard rolloutDashboardReader
	rollback         rollbackExecutor
	rollbackRecords  rollbackRecordStore
	evaluations      evaluationRunner
	drifts           driftManager
	queryer          governanceSQLQueryer
}

func NewModelGovernanceHandler() *ModelGovernanceHandler {
	return &ModelGovernanceHandler{}
}

func (h *ModelGovernanceHandler) WithRecommendationService(service recommendationGenerator) *ModelGovernanceHandler {
	h.recommendations = service
	return h
}

func (h *ModelGovernanceHandler) WithApprovalService(service approvalDecider) *ModelGovernanceHandler {
	h.approvals = service
	return h
}

func (h *ModelGovernanceHandler) WithVersionService(service versionLifecycle) *ModelGovernanceHandler {
	h.versions = service
	return h
}

func (h *ModelGovernanceHandler) WithRolloutService(service rolloutCoordinator) *ModelGovernanceHandler {
	h.rollouts = service
	return h
}

func (h *ModelGovernanceHandler) WithRolloutDashboardService(service rolloutDashboardReader) *ModelGovernanceHandler {
	h.rolloutDashboard = service
	return h
}

func (h *ModelGovernanceHandler) WithRollbackService(service rollbackExecutor) *ModelGovernanceHandler {
	h.rollback = service
	return h
}

func (h *ModelGovernanceHandler) WithRollbackRecordStore(store rollbackRecordStore) *ModelGovernanceHandler {
	h.rollbackRecords = store
	return h
}

func (h *ModelGovernanceHandler) WithEvaluationService(service evaluationRunner) *ModelGovernanceHandler {
	h.evaluations = service
	return h
}

func (h *ModelGovernanceHandler) WithDriftService(service driftManager) *ModelGovernanceHandler {
	h.drifts = service
	return h
}

func (h *ModelGovernanceHandler) WithQueryer(queryer governanceSQLQueryer) *ModelGovernanceHandler {
	h.queryer = queryer
	return h
}

func (h *ModelGovernanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case path == "/admin/governance/recommendations":
		h.handleRecommendations(w, r)
	case path == "/admin/governance/approvals":
		h.handleApprovals(w, r)
	case path == "/admin/governance/policy-versions":
		h.handlePolicyVersions(w, r)
	case strings.HasPrefix(path, "/admin/governance/policy-versions/"):
		h.handlePolicyVersionActions(w, r)
	case path == "/admin/governance/rollouts":
		h.handleRollouts(w, r)
	case strings.HasPrefix(path, "/admin/governance/rollouts/"):
		h.handleRolloutActions(w, r)
	case path == "/admin/governance/dashboard/rollouts":
		h.handleDashboardRollouts(w, r)
	case path == "/admin/governance/rollbacks":
		h.handleRollbacks(w, r)
	case strings.HasPrefix(path, "/admin/governance/rollbacks/"):
		h.handleRollbackByID(w, r)
	case path == "/admin/governance/evaluations":
		h.handleEvaluations(w, r)
	case strings.HasPrefix(path, "/admin/governance/evaluations/"):
		h.handleEvaluationActions(w, r)
	case path == "/admin/governance/drifts":
		h.handleDrifts(w, r)
	case strings.HasPrefix(path, "/admin/governance/drifts/"):
		h.handleDriftActions(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": path}})
	}
}

type generateRecommendationRequest struct {
	TenantID    string `json:"tenant_id"`
	AgentID     string `json:"agent_id"`
	TaskType    string `json:"task_type"`
	Environment string `json:"environment"`
	RequestedBy string `json:"requested_by"`
	Summary     string `json:"summary"`
}

func (h *ModelGovernanceHandler) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.queryer == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "governance query unavailable"})
			return
		}
		rows, err := h.queryer.QueryContext(r.Context(), `
SELECT recommendation_id AS id,
       agent_id,
       task_type,
       environment,
       recommended_model,
       status,
       created_at,
       updated_at
FROM model_recommendations
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
	case http.MethodPost:
		if h.recommendations == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "recommendation service unavailable"})
			return
		}
		var req generateRecommendationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		rec, err := h.recommendations.Generate(r.Context(), governance.GenerateRecommendationInput{
			TenantID:    strings.TrimSpace(req.TenantID),
			AgentID:     strings.TrimSpace(req.AgentID),
			TaskType:    strings.TrimSpace(req.TaskType),
			Environment: strings.TrimSpace(req.Environment),
			RequestedBy: strings.TrimSpace(req.RequestedBy),
			Summary:     strings.TrimSpace(req.Summary),
		})
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, rec)
	default:
		methodNotAllowed(w, r)
	}
}

type decideApprovalRequest struct {
	RecommendationID string                      `json:"recommendation_id"`
	Decision         governance.ApprovalDecision `json:"decision"`
	FinalModel       string                      `json:"final_model"`
	ApprovalReason   string                      `json:"approval_reason"`
	ApprovedBy       string                      `json:"approved_by"`
	EffectiveScope   governance.EffectiveScope   `json:"effective_scope"`
}

func (h *ModelGovernanceHandler) handleApprovals(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.queryer == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "governance query unavailable"})
			return
		}
		rows, err := h.queryer.QueryContext(r.Context(), `
SELECT approval_id AS id,
       recommendation_id,
       decision,
       final_model,
       approval_reason,
       approved_by,
       effective_scope,
       created_at
FROM model_approvals
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
	case http.MethodPost:
		if h.approvals == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "approval service unavailable"})
			return
		}
		var req decideApprovalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		approval, err := h.approvals.Decide(r.Context(), governance.ApprovalInput{
			RecommendationID: strings.TrimSpace(req.RecommendationID),
			Decision:         req.Decision,
			FinalModel:       strings.TrimSpace(req.FinalModel),
			ApprovalReason:   strings.TrimSpace(req.ApprovalReason),
			ApprovedBy:       strings.TrimSpace(req.ApprovedBy),
			EffectiveScope: governance.EffectiveScope{
				Scope:       strings.TrimSpace(req.EffectiveScope.Scope),
				ProjectID:   strings.TrimSpace(req.EffectiveScope.ProjectID),
				Environment: strings.TrimSpace(req.EffectiveScope.Environment),
			},
		})
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, approval)
	default:
		methodNotAllowed(w, r)
	}
}

type createPolicyVersionRequest struct {
	ApprovalID string `json:"approval_id"`
	CreatedBy  string `json:"created_by"`
}

type mutatePolicyVersionRequest struct {
	ApprovedBy string `json:"approved_by"`
}

func (h *ModelGovernanceHandler) handlePolicyVersions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.queryer == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "governance query unavailable"})
			return
		}
		rows, err := h.queryer.QueryContext(r.Context(), `
SELECT policy_version_id AS id,
       environment,
       status,
       source_approval_id,
       created_by,
       approved_by,
       approved_at,
       activated_at,
       created_at
FROM model_policy_versions
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
	case http.MethodPost:
		if h.versions == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "version service unavailable"})
			return
		}
		var req createPolicyVersionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		version, err := h.versions.CreateFromApproval(r.Context(), strings.TrimSpace(req.ApprovalID), strings.TrimSpace(req.CreatedBy))
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, version)
	default:
		methodNotAllowed(w, r)
	}
}

func (h *ModelGovernanceHandler) handlePolicyVersionActions(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/governance/policy-versions/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}
	if h.versions == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "version service unavailable"})
		return
	}
	versionID := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])

	if r.Method == http.MethodGet {
		switch action {
		case "diff":
			diff, err := h.versions.GetDiff(r.Context(), versionID)
			if err != nil {
				writeGovernanceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, diff)
			return
		default:
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
			return
		}
	}

	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	switch action {
	case "approve":
		var req mutatePolicyVersionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		version, err := h.versions.Approve(r.Context(), versionID, strings.TrimSpace(req.ApprovedBy))
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, version)
	case "activate":
		version, err := h.versions.Activate(r.Context(), versionID)
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, version)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
	}
}

func (h *ModelGovernanceHandler) handleDashboardRollouts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.rolloutDashboard == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollout dashboard service unavailable"})
		return
	}
	rows, err := h.rolloutDashboard.ListRows(r.Context(), governance.RolloutDashboardQuery{Limit: parseLimit(r, 20)})
	if err != nil {
		writeGovernanceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
}

func (h *ModelGovernanceHandler) handleRollouts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.queryer == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "governance query unavailable"})
			return
		}
		rows, err := h.queryer.QueryContext(r.Context(), `
SELECT rollout_id AS id,
       policy_version_id,
       environment,
       rollout_mode,
       rollout_percent,
       status,
       trigger_reason,
       triggered_by,
       created_at,
       updated_at
FROM model_rollouts
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
	case http.MethodPost:
		if h.rollouts == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollout service unavailable"})
			return
		}
		var input governance.StartRolloutInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		rollout, event, err := h.rollouts.Start(r.Context(), input)
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"rollout": rollout, "distribution_event": event})
	default:
		methodNotAllowed(w, r)
	}
}

func (h *ModelGovernanceHandler) handleRolloutActions(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/governance/rollouts/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	rolloutID := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	switch action {
	case "promote":
		if h.rollouts == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollout service unavailable"})
			return
		}
		var body struct {
			RolloutPercent int    `json:"rollout_percent"`
			GuardSummary   string `json:"guard_summary"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		rollout, err := h.rollouts.Promote(r.Context(), governance.PromoteRolloutInput{
			RolloutID:      rolloutID,
			RolloutPercent: body.RolloutPercent,
			GuardSummary:   strings.TrimSpace(body.GuardSummary),
		})
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rollout)
	case "rollback":
		h.handleRolloutRollback(w, r, rolloutID)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
	}
}

func (h *ModelGovernanceHandler) handleRolloutRollback(w http.ResponseWriter, r *http.Request, rolloutID string) {
	if h.rollback == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollback service unavailable"})
		return
	}
	if h.rollbackRecords == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollback record store unavailable"})
		return
	}
	var body struct {
		Actor  string `json:"actor"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	input := governance.ExecuteRollbackInput{
		RolloutID: strings.TrimSpace(rolloutID),
		Actor:     strings.TrimSpace(body.Actor),
		Reason:    strings.TrimSpace(body.Reason),
	}
	result, err := h.rollback.Execute(r.Context(), input)
	if err != nil {
		writeGovernanceError(w, err)
		return
	}
	record, err := h.rollbackRecords.Create(r.Context(), input, result)
	if err != nil {
		writeGovernanceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"rollback": record, "result": result})
}

func (h *ModelGovernanceHandler) handleRollbacks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.rollbackRecords == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollback record store unavailable"})
			return
		}
		items, err := h.rollbackRecords.List(r.Context(), parseLimit(r, 20))
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": items})
	case http.MethodPost:
		if h.rollback == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollback service unavailable"})
			return
		}
		if h.rollbackRecords == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollback record store unavailable"})
			return
		}
		var input governance.ExecuteRollbackInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		input.RolloutID = strings.TrimSpace(input.RolloutID)
		input.Actor = strings.TrimSpace(input.Actor)
		input.Reason = strings.TrimSpace(input.Reason)
		result, err := h.rollback.Execute(r.Context(), input)
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		record, err := h.rollbackRecords.Create(r.Context(), input, result)
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"rollback": record, "result": result})
	default:
		methodNotAllowed(w, r)
	}
}

func (h *ModelGovernanceHandler) handleRollbackByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if h.rollbackRecords == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "rollback record store unavailable"})
		return
	}
	rollbackID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/admin/governance/rollbacks/"))
	if rollbackID == "" || strings.Contains(rollbackID, "/") {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}
	record, err := h.rollbackRecords.Get(r.Context(), rollbackID)
	if err != nil {
		writeGovernanceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *ModelGovernanceHandler) handleEvaluations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.queryer == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "governance query unavailable"})
			return
		}
		rows, err := h.queryer.QueryContext(r.Context(), `
SELECT run_id AS id,
       dataset_id,
       formula_id,
       agent_id,
       task_type,
       environment,
       status,
       started_at,
       completed_at,
       created_at
FROM evaluation_runs
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
	case http.MethodPost:
		if h.evaluations == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "evaluation service unavailable"})
			return
		}
		var input governance.StartEvaluationRunInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		run, err := h.evaluations.StartRun(r.Context(), input)
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, run)
	default:
		methodNotAllowed(w, r)
	}
}

func (h *ModelGovernanceHandler) handleEvaluationActions(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/governance/evaluations/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if h.evaluations == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "evaluation service unavailable"})
		return
	}
	if strings.TrimSpace(parts[1]) != "status" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}
	var body struct {
		Status governance.EvaluationRunStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	run, err := h.evaluations.UpdateRunStatus(r.Context(), strings.TrimSpace(parts[0]), body.Status)
	if err != nil {
		writeGovernanceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *ModelGovernanceHandler) handleDrifts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.queryer == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "governance query unavailable"})
			return
		}
		rows, err := h.queryer.QueryContext(r.Context(), `
SELECT drift_id AS id,
       environment,
       agent_id,
       active_model,
       recommended_model,
       status,
       details,
       detected_at,
       updated_at
FROM policy_drifts
ORDER BY detected_at DESC
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
	case http.MethodPost:
		if h.drifts == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "drift service unavailable"})
			return
		}
		var body struct {
			TenantID    string `json:"tenant_id"`
			Environment string `json:"environment"`
			AgentID     string `json:"agent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		drift, detected, err := h.drifts.DetectModelMismatch(r.Context(), governance.DetectDriftInput{
			TenantID:    strings.TrimSpace(body.TenantID),
			Environment: strings.TrimSpace(body.Environment),
			AgentID:     strings.TrimSpace(body.AgentID),
		})
		if err != nil {
			writeGovernanceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"detected": detected, "drift": drift})
	default:
		methodNotAllowed(w, r)
	}
}

func (h *ModelGovernanceHandler) handleDriftActions(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/governance/drifts/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if h.drifts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "drift service unavailable"})
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	driftID := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	var (
		drift governance.PolicyDrift
		err   error
	)
	switch action {
	case "acknowledge":
		drift, err = h.drifts.Acknowledge(r.Context(), driftID, strings.TrimSpace(body.Reason))
	case "resolve":
		drift, err = h.drifts.Resolve(r.Context(), driftID, strings.TrimSpace(body.Reason))
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}
	if err != nil {
		writeGovernanceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, drift)
}

func writeGovernanceError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	writeJSON(w, governanceStatusFromError(err), map[string]any{"error": map[string]any{"message": err.Error(), "type": "governance_error"}})
}

func governanceStatusFromError(err error) int {
	switch {
	case errors.Is(err, governance.ErrApprovalRecommendationNotFound),
		errors.Is(err, governance.ErrPolicyVersionNotFound),
		errors.Is(err, governance.ErrEvaluationDatasetNotFound),
		errors.Is(err, governance.ErrScoringFormulaNotFound),
		errors.Is(err, governance.ErrEvaluationRunNotFound),
		errors.Is(err, governance.ErrPolicyDriftNotFound),
		errors.Is(err, governance.ErrActivePolicyNotFound),
		errors.Is(err, governance.ErrRolloutNotFound),
		errors.Is(err, governance.ErrRollbackNotFound):
		return http.StatusNotFound
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(msg, "required") || strings.Contains(msg, "invalid") || strings.Contains(msg, "must") || strings.Contains(msg, "mismatch") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func scanRowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, 16)
	for rows.Next() {
		raw := make([]any, len(columns))
		holders := make([]any, len(columns))
		for i := range raw {
			holders[i] = &raw[i]
		}
		if err := rows.Scan(holders...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(columns))
		for idx, name := range columns {
			item[name] = normalizeScannedValue(raw[idx])
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeScannedValue(in any) any {
	switch v := in.(type) {
	case nil:
		return nil
	case []byte:
		text := strings.TrimSpace(string(v))
		if text == "" {
			return ""
		}
		if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
			var decoded any
			if err := json.Unmarshal([]byte(text), &decoded); err == nil {
				return decoded
			}
		}
		return text
	case string:
		return strings.TrimSpace(v)
	default:
		return v
	}
}
