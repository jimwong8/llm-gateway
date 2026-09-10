package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/governance"
	"llm-gateway/gateway/internal/httpserver"
	"llm-gateway/gateway/internal/memory"
)

const adminToken = "admin-secret"

type recommendationServiceStub struct{}

func (s *recommendationServiceStub) Generate(_ context.Context, input governance.GenerateRecommendationInput) (governance.Recommendation, error) {
	return governance.Recommendation{ID: "rec-1", AgentID: input.AgentID, Environment: input.Environment, TaskType: input.TaskType, RecommendedModel: "model-a", Status: governance.RecommendationStatusDraft}, nil
}

type approvalServiceStub struct{}

func (s *approvalServiceStub) Decide(_ context.Context, input governance.ApprovalInput) (governance.Approval, error) {
	return governance.Approval{ID: "ap-1", RecommendationID: input.RecommendationID, FinalModel: input.FinalModel}, nil
}

type versionServiceStub struct{}

func (s *versionServiceStub) CreateFromApproval(_ context.Context, approvalID, _ string) (governance.PolicyVersion, error) {
	return governance.PolicyVersion{ID: "pv-1", Status: governance.PolicyVersionDraft}, nil
}
func (s *versionServiceStub) Approve(_ context.Context, versionID, _ string) (governance.PolicyVersion, error) {
	return governance.PolicyVersion{ID: versionID, Status: governance.PolicyVersionApproved}, nil
}
func (s *versionServiceStub) Activate(_ context.Context, versionID string) (governance.PolicyVersion, error) {
	return governance.PolicyVersion{ID: versionID, Status: governance.PolicyVersionActive}, nil
}
func (s *versionServiceStub) GetDiff(_ context.Context, versionID string) (governance.PolicyVersionDiff, error) {
	return governance.PolicyVersionDiff{
		CurrentVersion: governance.PolicyVersion{ID: versionID, Version: 2, Status: governance.PolicyVersionDraft},
		BaseVersion:    &governance.PolicyVersion{ID: "pv-0", Version: 1, Status: governance.PolicyVersionSuperseded},
		BaseType:       "previous",
		Changes:        []governance.PolicyDiffEntry{{Path: "default_model", ChangeType: "modified", From: "gpt-4o-mini", To: "gpt-4.1-mini"}},
	}, nil
}

type rolloutServiceStub struct{}

func (s *rolloutServiceStub) Start(_ context.Context, input governance.StartRolloutInput) (governance.Rollout, governance.DistributionEvent, error) {
	return governance.Rollout{ID: "ro-1", PolicyVersionID: input.PolicyVersionID, Status: governance.RolloutStatusRunning}, governance.DistributionEvent{ID: "evt-1"}, nil
}
func (s *rolloutServiceStub) Promote(_ context.Context, input governance.PromoteRolloutInput) (governance.Rollout, error) {
	return governance.Rollout{ID: input.RolloutID, RolloutPercent: input.RolloutPercent, Status: governance.RolloutStatusPromoted}, nil
}

type rolloutDashboardServiceStub struct{}

func (s *rolloutDashboardServiceStub) ListRows(_ context.Context, _ governance.RolloutDashboardQuery) ([]governance.RolloutDashboardRow, error) {
	return []governance.RolloutDashboardRow{{RolloutID: "ro-1", PolicyVersionID: "pv-1", Environment: "prod", Percent: 50, Status: "running", ErrorRate: 0.01, P95Latency: 99, FallbackRate: 0.02, SampleCount: 10}}, nil
}

type rollbackServiceStub struct{}

func (s *rollbackServiceStub) Execute(_ context.Context, input governance.ExecuteRollbackInput) (governance.ExecuteRollbackResult, error) {
	rolloutID := input.RolloutID
	if rolloutID == "" {
		rolloutID = "ro-1"
	}
	return governance.ExecuteRollbackResult{
		Rollout:                 governance.Rollout{ID: rolloutID, TargetEnvironment: "prod", Status: governance.RolloutStatusRolledBack},
		RestoredPolicyVersionID: "pv-prev",
		RevertedPolicyVersionID: "pv-cur",
		DistributionEvent:       governance.DistributionEvent{ID: "evt-rb", EventType: governance.DistributionEventRollback},
	}, nil
}

type rollbackRecordStoreStub struct{}

func (s *rollbackRecordStoreStub) Create(_ context.Context, input governance.ExecuteRollbackInput, _ governance.ExecuteRollbackResult) (governance.RollbackRecord, error) {
	id := "rb-1"
	if input.RolloutID == "ro-2" {
		id = "rb-2"
	}
	return governance.RollbackRecord{ID: id, RolloutID: input.RolloutID, Actor: input.Actor, Environment: "prod", RestoredPolicyVersionID: "pv-prev", RevertedPolicyVersionID: "pv-cur"}, nil
}

func (s *rollbackRecordStoreStub) List(_ context.Context, _ int) ([]governance.RollbackRecord, error) {
	return []governance.RollbackRecord{{ID: "rb-1", RolloutID: "ro-1", Actor: "ops-bot", Environment: "prod", RestoredPolicyVersionID: "pv-prev", RevertedPolicyVersionID: "pv-cur"}}, nil
}

func (s *rollbackRecordStoreStub) Get(_ context.Context, rollbackID string) (governance.RollbackRecord, error) {
	return governance.RollbackRecord{ID: rollbackID, RolloutID: "ro-1", Actor: "ops-bot", Environment: "prod", RestoredPolicyVersionID: "pv-prev", RevertedPolicyVersionID: "pv-cur"}, nil
}

type evaluationServiceStub struct{}

func (s *evaluationServiceStub) StartRun(_ context.Context, input governance.StartEvaluationRunInput) (governance.EvaluationRun, error) {
	return governance.EvaluationRun{ID: "run-1", Status: governance.EvaluationRunStatusRunning}, nil
}
func (s *evaluationServiceStub) UpdateRunStatus(_ context.Context, runID string, status governance.EvaluationRunStatus) (governance.EvaluationRun, error) {
	return governance.EvaluationRun{ID: runID, Status: status}, nil
}

type driftServiceStub struct{}

func (s *driftServiceStub) DetectModelMismatch(_ context.Context, input governance.DetectDriftInput) (governance.PolicyDrift, bool, error) {
	return governance.PolicyDrift{ID: "drift-1", Status: governance.PolicyDriftStatusDetected}, true, nil
}
func (s *driftServiceStub) Acknowledge(_ context.Context, driftID, reason string) (governance.PolicyDrift, error) {
	return governance.PolicyDrift{ID: driftID, Status: governance.PolicyDriftStatusAccepted}, nil
}
func (s *driftServiceStub) Resolve(_ context.Context, driftID, reason string) (governance.PolicyDrift, error) {
	return governance.PolicyDrift{ID: driftID, Status: governance.PolicyDriftStatusResolved}, nil
}

type governanceSQLSource interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type runtimeResolverStub struct{}

type governanceQueryerStub struct{}

type memoryAdminStoreStub struct{}

func governanceVerifyDSN() string {
	if dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	if dsn := strings.TrimSpace(os.Getenv("GOVERNANCE_TEST_POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	return ""
}

func maybeBuildGovernanceRuntimeQueryer() (governanceSQLSource, bool, error) {
	dsn := governanceVerifyDSN()
	if dsn == "" {
		return &governanceQueryerStub{}, false, nil
	}
	store, err := governance.NewStore(dsn)
	if err != nil {
		return nil, false, err
	}
	seedGovernanceRuntimeObserverData(store)
	return store.DB(), true, nil
}

func seedGovernanceRuntimeObserverData(store *governance.Store) {
	if store == nil || store.DB() == nil {
		return
	}
	db := store.DB()
	ctx := context.Background()
	environment := "prod"
	policyVersionID := "pv-verify-runtime"
	activatedAt := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

	policyRaw, err := json.Marshal(governance.RuntimePolicy{
		Version:      1,
		Environment:  environment,
		DefaultModel: "model-a",
	})
	if err == nil {
		_, _ = db.ExecContext(ctx, `
INSERT INTO model_policy_versions (
	policy_version_id,
	environment,
	status,
	policy_json,
	created_by,
	approved_by,
	created_at,
	approved_at,
	activated_at
) VALUES ($1, $2, 'active', $3::jsonb, 'verify', 'verify', $4, $4, $4)
ON CONFLICT (policy_version_id) DO NOTHING
`, policyVersionID, environment, string(policyRaw), activatedAt)
	}

	matchedScopeRaw := `{"environment":"prod"}`
	fallbackRaw := `[]`
	_, _ = db.ExecContext(ctx, `
INSERT INTO runtime_decision_snapshots (
	request_id,
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
	success,
	created_at
) VALUES ($1, $2, 'ro-verify', $3, 'tenant-verify', 'agent-verify', 'chat', 'environment', $4::jsonb, $5, $6::jsonb, false, false, true, $7)
ON CONFLICT (request_id) DO NOTHING
`, "req-verify-runtime", policyVersionID, environment, matchedScopeRaw, "model-a", fallbackRaw, activatedAt.Add(30*time.Second))

	payloadRaw := `{"rollout_percent":100,"triggered_by":"verify"}`
	_, _ = db.ExecContext(ctx, `
INSERT INTO model_distribution_events (
	event_id,
	policy_version_id,
	rollout_id,
	environment,
	event_type,
	payload,
	created_at
) VALUES ($1, $2, 'ro-verify', $3, 'policy_distribution.activated', $4::jsonb, $5)
ON CONFLICT (event_id) DO NOTHING
`, "event-verify-runtime", policyVersionID, environment, payloadRaw, activatedAt.Add(time.Minute))
}

func (s *memoryAdminStoreStub) ListCandidateFacts(_ context.Context, tenantID, userID, status string) ([]memory.CandidateFact, error) {
	return []memory.CandidateFact{{ID: 1, TenantID: tenantID, UserID: userID, Key: "repo", Value: "mono", Status: firstNonEmpty(status, "pending")}}, nil
}

func (s *memoryAdminStoreStub) ListProjectFacts(_ context.Context, tenantID, userID, status string) ([]memory.ProjectFact, error) {
	return []memory.ProjectFact{{ID: 2, TenantID: tenantID, UserID: userID, Key: "stack", Value: "go", Status: firstNonEmpty(status, "active")}}, nil
}

func (s *memoryAdminStoreStub) ConfirmCandidateFact(_ context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error) {
	return &memory.CandidateFact{ID: 1, TenantID: tenantID, UserID: userID, Key: factKey, Value: "mono", Status: "confirmed"}, nil
}

func (s *memoryAdminStoreStub) RejectCandidateFact(_ context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error) {
	return &memory.CandidateFact{ID: 1, TenantID: tenantID, UserID: userID, Key: factKey, Value: "mono", Status: "rejected"}, nil
}

func (s *memoryAdminStoreStub) PromoteCandidateFact(_ context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error) {
	return &memory.CandidateFact{ID: 1, TenantID: tenantID, UserID: userID, Key: factKey, Value: "mono", Status: "promoted"}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *governanceQueryerStub) QueryContext(_ context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, errors.New("query unavailable in verify stub")
}

func (s *runtimeResolverStub) Resolve(_ context.Context, input governance.ResolveInput) (governance.ResolveDecision, error) {
	return governance.ResolveDecision{RequestID: input.RequestID, ResolvedModel: "model-a", MatchedScopeType: "agent"}, nil
}

func (s *runtimeResolverStub) ObserverSnapshot() governance.RuntimeResolverObserverSnapshot {
	return governance.RuntimeResolverObserverSnapshot{}
}

func main() {
	queryer, realRuntimeQueries, err := maybeBuildGovernanceRuntimeQueryer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build governance runtime queryer failed: %v\n", err)
		os.Exit(1)
	}

	srv := httpserver.New(config.Config{AdminAPIKey: adminToken}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithModelGovernanceHandler(httpserver.NewModelGovernanceHandler().
			WithRecommendationService(&recommendationServiceStub{}).
			WithApprovalService(&approvalServiceStub{}).
			WithVersionService(&versionServiceStub{}).
			WithRolloutService(&rolloutServiceStub{}).
			WithRolloutDashboardService(&rolloutDashboardServiceStub{}).
			WithRollbackService(&rollbackServiceStub{}).
			WithRollbackRecordStore(&rollbackRecordStoreStub{}).
			WithEvaluationService(&evaluationServiceStub{}).
			WithDriftService(&driftServiceStub{})).
		WithModelRuntimeHandler(httpserver.NewModelRuntimeHandler().WithResolver(&runtimeResolverStub{}).WithQueryer(queryer)).
		WithMemoryAdminHandler(httpserver.NewMemoryAdminHandler(&memoryAdminStoreStub{}))

	handler := srv.Handler()

	assertStatus(handler, http.MethodPost, "/admin/governance/recommendations", `{"agent_id":"a1","task_type":"chat","environment":"prod"}`, http.StatusUnauthorized, false)
	assertStatus(handler, http.MethodGet, "/admin/memory/candidate-facts?tenant_id=t1&user_id=u1", ``, http.StatusUnauthorized, false)
	assertStatus(handler, http.MethodPost, "/admin/governance/recommendations", `{"agent_id":"a1","task_type":"chat","environment":"prod"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/approvals", `{"recommendation_id":"rec-1","decision":"approved","approved_by":"ops","effective_scope":{"scope":"agent","environment":"prod"}}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/policy-versions", `{"approval_id":"ap-1","created_by":"ops"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/policy-versions/pv-1/approve", `{"approved_by":"ops"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/policy-versions/pv-1/activate", `{}`, http.StatusOK, true)
	assertStatus(handler, http.MethodGet, "/admin/governance/policy-versions/pv-1/diff", ``, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/rollouts", `{"policy_version_id":"pv-1","triggered_by":"ops"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/rollouts/ro-1/promote", `{"rollout_percent":50}`, http.StatusOK, true)
	assertStatus(handler, http.MethodGet, "/admin/governance/dashboard/rollouts", ``, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/rollbacks", `{"rollout_id":"ro-1","actor":"ops-bot","reason":"manual"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodGet, "/admin/governance/rollbacks", ``, http.StatusOK, true)
	assertStatus(handler, http.MethodGet, "/admin/governance/rollbacks/rb-1", ``, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/rollouts/ro-2/rollback", `{"actor":"ops-bot","reason":"guard"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/evaluations", `{"dataset_id":"d1","agent_id":"a1","task_type":"chat","environment":"prod"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/evaluations/run-1/status", `{"status":"succeeded"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/drifts", `{"environment":"prod","agent_id":"a1"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/drifts/drift-1/acknowledge", `{"reason":"ok"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/drifts/drift-1/resolve", `{"reason":"fixed"}`, http.StatusOK, true)
	if realRuntimeQueries {
		assertStatus(handler, http.MethodGet, "/admin/governance/runtime-decisions?limit=5", ``, http.StatusOK, true)
		assertStatus(handler, http.MethodGet, "/admin/governance/distribution-events?limit=5", ``, http.StatusOK, true)
		assertStatus(handler, http.MethodGet, "/admin/governance/runtime-observer?environment=prod&limit=5", ``, http.StatusOK, true)
	} else {
		assertStatus(handler, http.MethodGet, "/admin/governance/runtime-decisions", ``, http.StatusInternalServerError, true)
		assertStatus(handler, http.MethodGet, "/admin/governance/distribution-events", ``, http.StatusInternalServerError, true)
		assertStatus(handler, http.MethodGet, "/admin/governance/runtime-observer", ``, http.StatusInternalServerError, true)
	}
	assertStatus(handler, http.MethodGet, "/admin/memory/candidate-facts?tenant_id=t1&user_id=u1&status=pending", ``, http.StatusOK, true)
	assertStatus(handler, http.MethodGet, "/admin/memory/project-facts?tenant_id=t1&user_id=u1&status=active", ``, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/memory/candidate-facts/repo/confirm", `{"tenant_id":"t1","user_id":"u1"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/memory/candidate-facts/repo/reject", `{"tenant_id":"t1","user_id":"u1"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/memory/candidate-facts/repo/promote", `{"tenant_id":"t1","user_id":"u1"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/memory/candidate-facts/actions/confirm", `{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"repo"},{"tenant_id":"t1","user_id":"u1","fact_key":"stack"}]}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/memory/candidate-facts/actions/reject", `{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"repo"},{"tenant_id":"t1","user_id":"u1","fact_key":"stack"}]}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/memory/candidate-facts/actions/promote", `{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"repo"},{"tenant_id":"t1","user_id":"u1","fact_key":"stack"}]}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/v1/runtime/resolve", `{"request_id":"req-1","environment":"prod","agent_id":"a1"}`, http.StatusOK, false)

	fmt.Println("verify result: PASS model_governance admin/runtime routes")
}

func assertStatus(handler http.Handler, method, path, body string, expected int, withAdmin bool) {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if withAdmin {
		req.Header.Set("X-Admin-Key", adminToken)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != expected {
		fmt.Fprintf(os.Stderr, "verify %s %s expected %d got %d body=%s\n", method, path, expected, rr.Code, rr.Body.String())
		os.Exit(1)
	}
}
