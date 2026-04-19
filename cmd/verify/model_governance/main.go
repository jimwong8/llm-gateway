package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/governance"
	"llm-gateway/gateway/internal/httpserver"
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

type rolloutServiceStub struct{}

func (s *rolloutServiceStub) Start(_ context.Context, input governance.StartRolloutInput) (governance.Rollout, governance.DistributionEvent, error) {
	return governance.Rollout{ID: "ro-1", PolicyVersionID: input.PolicyVersionID, Status: governance.RolloutStatusRunning}, governance.DistributionEvent{ID: "evt-1"}, nil
}
func (s *rolloutServiceStub) Promote(_ context.Context, input governance.PromoteRolloutInput) (governance.Rollout, error) {
	return governance.Rollout{ID: input.RolloutID, RolloutPercent: input.RolloutPercent, Status: governance.RolloutStatusPromoted}, nil
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

type runtimeResolverStub struct{}

func (s *runtimeResolverStub) Resolve(_ context.Context, input governance.ResolveInput) (governance.ResolveDecision, error) {
	return governance.ResolveDecision{RequestID: input.RequestID, ResolvedModel: "model-a", MatchedScopeType: "agent"}, nil
}

func main() {
	srv := httpserver.New(config.Config{AdminAPIKey: adminToken}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithModelGovernanceHandler(httpserver.NewModelGovernanceHandler().
			WithRecommendationService(&recommendationServiceStub{}).
			WithApprovalService(&approvalServiceStub{}).
			WithVersionService(&versionServiceStub{}).
			WithRolloutService(&rolloutServiceStub{}).
			WithEvaluationService(&evaluationServiceStub{}).
			WithDriftService(&driftServiceStub{})).
		WithModelRuntimeHandler(httpserver.NewModelRuntimeHandler().WithResolver(&runtimeResolverStub{}))

	handler := srv.Handler()

	assertStatus(handler, http.MethodPost, "/admin/governance/recommendations", `{"agent_id":"a1","task_type":"chat","environment":"prod"}`, http.StatusUnauthorized, false)
	assertStatus(handler, http.MethodPost, "/admin/governance/recommendations", `{"agent_id":"a1","task_type":"chat","environment":"prod"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/approvals", `{"recommendation_id":"rec-1","decision":"approved","approved_by":"ops","effective_scope":{"scope":"agent","environment":"prod"}}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/policy-versions", `{"approval_id":"ap-1","created_by":"ops"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/policy-versions/pv-1/approve", `{"approved_by":"ops"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/policy-versions/pv-1/activate", `{}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/rollouts", `{"policy_version_id":"pv-1","triggered_by":"ops"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/rollouts/ro-1/promote", `{"rollout_percent":50}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/evaluations", `{"dataset_id":"d1","agent_id":"a1","task_type":"chat","environment":"prod"}`, http.StatusCreated, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/evaluations/run-1/status", `{"status":"succeeded"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/drifts", `{"environment":"prod","agent_id":"a1"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/drifts/drift-1/acknowledge", `{"reason":"ok"}`, http.StatusOK, true)
	assertStatus(handler, http.MethodPost, "/admin/governance/drifts/drift-1/resolve", `{"reason":"fixed"}`, http.StatusOK, true)
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
