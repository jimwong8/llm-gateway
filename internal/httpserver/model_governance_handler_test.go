package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/governance"
)

type recommendationServiceStub struct {
	lastInput governance.GenerateRecommendationInput
	resp      governance.Recommendation
	err       error
}

func (s *recommendationServiceStub) Generate(_ context.Context, input governance.GenerateRecommendationInput) (governance.Recommendation, error) {
	s.lastInput = input
	if s.err != nil {
		return governance.Recommendation{}, s.err
	}
	return s.resp, nil
}

type approvalServiceStub struct {
	lastInput governance.ApprovalInput
	resp      governance.Approval
	err       error
}

func (s *approvalServiceStub) Decide(_ context.Context, input governance.ApprovalInput) (governance.Approval, error) {
	s.lastInput = input
	if s.err != nil {
		return governance.Approval{}, s.err
	}
	return s.resp, nil
}

type versionServiceStub struct {
	createResp   governance.PolicyVersion
	approveResp  governance.PolicyVersion
	activateResp governance.PolicyVersion
	diffResp     governance.PolicyVersionDiff

	createdApprovalID  string
	approvedVersionID  string
	activatedVersionID string
	diffVersionID      string
}

func (s *versionServiceStub) CreateFromApproval(_ context.Context, approvalID, _ string) (governance.PolicyVersion, error) {
	s.createdApprovalID = approvalID
	return s.createResp, nil
}

func (s *versionServiceStub) Approve(_ context.Context, versionID, _ string) (governance.PolicyVersion, error) {
	s.approvedVersionID = versionID
	return s.approveResp, nil
}

func (s *versionServiceStub) Activate(_ context.Context, versionID string) (governance.PolicyVersion, error) {
	s.activatedVersionID = versionID
	return s.activateResp, nil
}

func (s *versionServiceStub) GetDiff(_ context.Context, versionID string) (governance.PolicyVersionDiff, error) {
	s.diffVersionID = versionID
	return s.diffResp, nil
}

type rolloutServiceStub struct {
	startInput   governance.StartRolloutInput
	promoteInput governance.PromoteRolloutInput
}

func (s *rolloutServiceStub) Start(_ context.Context, input governance.StartRolloutInput) (governance.Rollout, governance.DistributionEvent, error) {
	s.startInput = input
	return governance.Rollout{ID: "ro-1", PolicyVersionID: input.PolicyVersionID}, governance.DistributionEvent{ID: "ev-1"}, nil
}

func (s *rolloutServiceStub) Promote(_ context.Context, input governance.PromoteRolloutInput) (governance.Rollout, error) {
	s.promoteInput = input
	return governance.Rollout{ID: input.RolloutID, RolloutPercent: input.RolloutPercent}, nil
}

type rollbackServiceStub struct {
	inputs []governance.ExecuteRollbackInput
}

func (s *rollbackServiceStub) Execute(_ context.Context, input governance.ExecuteRollbackInput) (governance.ExecuteRollbackResult, error) {
	s.inputs = append(s.inputs, input)
	rolloutID := input.RolloutID
	if rolloutID == "" {
		rolloutID = "ro-1"
	}
	return governance.ExecuteRollbackResult{
		Rollout:                 governance.Rollout{ID: rolloutID, TargetEnvironment: "prod", Status: governance.RolloutStatusRolledBack},
		RestoredPolicyVersionID: "pv-prev",
		RevertedPolicyVersionID: "pv-cur",
		DistributionEvent:       governance.DistributionEvent{ID: "ev-rb", EventType: governance.DistributionEventRollback},
	}, nil
}

type rollbackRecordStoreStub struct {
	createInputs []governance.ExecuteRollbackInput
	listCalled   bool
	getID        string
}

func (s *rollbackRecordStoreStub) Create(_ context.Context, input governance.ExecuteRollbackInput, _ governance.ExecuteRollbackResult) (governance.RollbackRecord, error) {
	s.createInputs = append(s.createInputs, input)
	id := "rb-1"
	if len(s.createInputs) > 1 {
		id = "rb-2"
	}
	rolloutID := input.RolloutID
	if rolloutID == "" {
		rolloutID = "ro-1"
	}
	return governance.RollbackRecord{ID: id, RolloutID: rolloutID, Actor: input.Actor, Environment: "prod", RestoredPolicyVersionID: "pv-prev", RevertedPolicyVersionID: "pv-cur"}, nil
}

func (s *rollbackRecordStoreStub) List(_ context.Context, _ int) ([]governance.RollbackRecord, error) {
	s.listCalled = true
	return []governance.RollbackRecord{{ID: "rb-1", RolloutID: "ro-1", Actor: "ops", Environment: "prod", RestoredPolicyVersionID: "pv-prev", RevertedPolicyVersionID: "pv-cur"}}, nil
}

func (s *rollbackRecordStoreStub) Get(_ context.Context, rollbackID string) (governance.RollbackRecord, error) {
	s.getID = rollbackID
	return governance.RollbackRecord{ID: rollbackID, RolloutID: "ro-1", Actor: "ops", Environment: "prod", RestoredPolicyVersionID: "pv-prev", RevertedPolicyVersionID: "pv-cur"}, nil
}

type rolloutDashboardServiceStub struct {
	rows []governance.RolloutDashboardRow
	err  error
}

func (s *rolloutDashboardServiceStub) ListRows(_ context.Context, _ governance.RolloutDashboardQuery) ([]governance.RolloutDashboardRow, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.rows, nil
}

type evaluationServiceStub struct {
	startInput  governance.StartEvaluationRunInput
	statusRunID string
	statusValue governance.EvaluationRunStatus
}

func (s *evaluationServiceStub) StartRun(_ context.Context, input governance.StartEvaluationRunInput) (governance.EvaluationRun, error) {
	s.startInput = input
	return governance.EvaluationRun{ID: "run-1", Status: governance.EvaluationRunStatusRunning}, nil
}

func (s *evaluationServiceStub) UpdateRunStatus(_ context.Context, runID string, status governance.EvaluationRunStatus) (governance.EvaluationRun, error) {
	s.statusRunID = runID
	s.statusValue = status
	return governance.EvaluationRun{ID: runID, Status: status}, nil
}

type driftServiceStub struct {
	detectInput governance.DetectDriftInput
	ackID       string
	resolveID   string
}

func (s *driftServiceStub) DetectModelMismatch(_ context.Context, input governance.DetectDriftInput) (governance.PolicyDrift, bool, error) {
	s.detectInput = input
	return governance.PolicyDrift{ID: "drift-1", Status: governance.PolicyDriftStatusDetected}, true, nil
}

func (s *driftServiceStub) Acknowledge(_ context.Context, driftID, _ string) (governance.PolicyDrift, error) {
	s.ackID = driftID
	return governance.PolicyDrift{ID: driftID, Status: governance.PolicyDriftStatusAccepted}, nil
}

func (s *driftServiceStub) Resolve(_ context.Context, driftID, _ string) (governance.PolicyDrift, error) {
	s.resolveID = driftID
	return governance.PolicyDrift{ID: driftID, Status: governance.PolicyDriftStatusResolved}, nil
}

type runtimeResolverStub struct {
	input    governance.ResolveInput
	resp     governance.ResolveDecision
	err      error
	snapshot governance.RuntimeResolverObserverSnapshot
}

func (s *runtimeResolverStub) Resolve(_ context.Context, input governance.ResolveInput) (governance.ResolveDecision, error) {
	s.input = input
	if s.err != nil {
		return governance.ResolveDecision{}, s.err
	}
	return s.resp, nil
}

func (s *runtimeResolverStub) ObserverSnapshot() governance.RuntimeResolverObserverSnapshot {
	return s.snapshot
}

func TestModelGovernanceHandlerCoreEndpoints(t *testing.T) {
	recSvc := &recommendationServiceStub{resp: governance.Recommendation{ID: "rec-1"}}
	approvalSvc := &approvalServiceStub{resp: governance.Approval{ID: "ap-1"}}
	versionSvc := &versionServiceStub{
		createResp:   governance.PolicyVersion{ID: "pv-1"},
		approveResp:  governance.PolicyVersion{ID: "pv-1", Status: governance.PolicyVersionApproved},
		activateResp: governance.PolicyVersion{ID: "pv-1", Status: governance.PolicyVersionActive},
		diffResp: governance.PolicyVersionDiff{
			CurrentVersion: governance.PolicyVersion{ID: "pv-1", Version: 2, Status: governance.PolicyVersionDraft},
			BaseVersion:    &governance.PolicyVersion{ID: "pv-0", Version: 1, Status: governance.PolicyVersionSuperseded},
			BaseType:       "previous",
			Changes:        []governance.PolicyDiffEntry{{Path: "default_model", ChangeType: "modified", From: "gpt-4o-mini", To: "gpt-4.1-mini"}},
		},
	}
	rolloutSvc := &rolloutServiceStub{}
	rolloutDashboardSvc := &rolloutDashboardServiceStub{rows: []governance.RolloutDashboardRow{{
		RolloutID:       "ro-1",
		PolicyVersionID: "pv-1",
		Environment:     "prod",
		Percent:         50,
		Status:          "running",
		ErrorRate:       0.02,
		P95Latency:      120,
		FallbackRate:    0.10,
		SampleCount:     48,
	}}}
	rollbackSvc := &rollbackServiceStub{}
	rollbackRecords := &rollbackRecordStoreStub{}
	evalSvc := &evaluationServiceStub{}
	driftSvc := &driftServiceStub{}

	h := NewModelGovernanceHandler().
		WithRecommendationService(recSvc).
		WithApprovalService(approvalSvc).
		WithVersionService(versionSvc).
		WithRolloutService(rolloutSvc).
		WithRolloutDashboardService(rolloutDashboardSvc).
		WithRollbackService(rollbackSvc).
		WithRollbackRecordStore(rollbackRecords).
		WithEvaluationService(evalSvc).
		WithDriftService(driftSvc)

	cases := []struct {
		method string
		path   string
		body   string
		code   int
	}{
		{http.MethodPost, "/admin/governance/recommendations", `{"tenant_id":"t1","agent_id":"a1","task_type":"chat","environment":"prod"}`, http.StatusCreated},
		{http.MethodPost, "/admin/governance/approvals", `{"recommendation_id":"rec-1","decision":"approved","approved_by":"ops","effective_scope":{"scope":"agent","environment":"prod"}}`, http.StatusCreated},
		{http.MethodPost, "/admin/governance/policy-versions", `{"approval_id":"ap-1","created_by":"ops"}`, http.StatusCreated},
		{http.MethodPost, "/admin/governance/policy-versions/pv-1/approve", `{"approved_by":"ops"}`, http.StatusOK},
		{http.MethodPost, "/admin/governance/policy-versions/pv-1/activate", `{}`, http.StatusOK},
		{http.MethodGet, "/admin/governance/policy-versions/pv-1/diff", ``, http.StatusOK},
		{http.MethodPost, "/admin/governance/rollouts", `{"policy_version_id":"pv-1","triggered_by":"ops"}`, http.StatusCreated},
		{http.MethodPost, "/admin/governance/rollouts/ro-1/promote", `{"rollout_percent":50}`, http.StatusOK},
		{http.MethodGet, "/admin/governance/dashboard/rollouts", ``, http.StatusOK},
		{http.MethodPost, "/admin/governance/rollbacks", `{"rollout_id":"ro-1","actor":"ops-bot","reason":"manual"}`, http.StatusCreated},
		{http.MethodGet, "/admin/governance/rollbacks", ``, http.StatusOK},
		{http.MethodGet, "/admin/governance/rollbacks/rb-1", ``, http.StatusOK},
		{http.MethodPost, "/admin/governance/rollouts/ro-2/rollback", `{"actor":"ops-bot","reason":"guard"}`, http.StatusCreated},
		{http.MethodPost, "/admin/governance/evaluations", `{"dataset_id":"d1","agent_id":"a1","task_type":"chat","environment":"prod"}`, http.StatusCreated},
		{http.MethodPost, "/admin/governance/evaluations/run-1/status", `{"status":"succeeded"}`, http.StatusOK},
		{http.MethodPost, "/admin/governance/drifts", `{"tenant_id":"t1","environment":"prod","agent_id":"a1"}`, http.StatusOK},
		{http.MethodPost, "/admin/governance/drifts/drift-1/acknowledge", `{"reason":"ok"}`, http.StatusOK},
		{http.MethodPost, "/admin/governance/drifts/drift-1/resolve", `{"reason":"fixed"}`, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.code {
				t.Fatalf("expected %d, got %d body=%s", tc.code, rr.Code, rr.Body.String())
			}
			if tc.path == "/admin/governance/dashboard/rollouts" {
				var payload struct {
					Data []governance.RolloutDashboardRow `json:"data"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
					t.Fatalf("decode dashboard payload failed: %v", err)
				}
				if len(payload.Data) != 1 || payload.Data[0].RolloutID != "ro-1" {
					t.Fatalf("unexpected dashboard rows: %+v", payload.Data)
				}
			}
		})
	}

	if recSvc.lastInput.AgentID != "a1" {
		t.Fatalf("expected recommendation input captured")
	}
	if approvalSvc.lastInput.RecommendationID != "rec-1" {
		t.Fatalf("expected approval input captured")
	}
	if versionSvc.createdApprovalID != "ap-1" || versionSvc.approvedVersionID != "pv-1" || versionSvc.activatedVersionID != "pv-1" || versionSvc.diffVersionID != "pv-1" {
		t.Fatalf("expected version actions captured")
	}
	if rolloutSvc.startInput.PolicyVersionID != "pv-1" || rolloutSvc.promoteInput.RolloutID != "ro-1" {
		t.Fatalf("expected rollout actions captured")
	}
	if len(rollbackSvc.inputs) != 2 {
		t.Fatalf("expected 2 rollback executions, got %d", len(rollbackSvc.inputs))
	}
	if rollbackSvc.inputs[0].RolloutID != "ro-1" || rollbackSvc.inputs[1].RolloutID != "ro-2" {
		t.Fatalf("unexpected rollback rollout ids: %+v", rollbackSvc.inputs)
	}
	if !rollbackRecords.listCalled || rollbackRecords.getID != "rb-1" {
		t.Fatalf("expected rollback list/get captured")
	}
	if len(rollbackRecords.createInputs) != 2 {
		t.Fatalf("expected rollback record create captured twice, got %d", len(rollbackRecords.createInputs))
	}
	if evalSvc.startInput.DatasetID != "d1" || evalSvc.statusRunID != "run-1" {
		t.Fatalf("expected evaluation actions captured")
	}
	if driftSvc.detectInput.AgentID != "a1" || driftSvc.ackID != "drift-1" || driftSvc.resolveID != "drift-1" {
		t.Fatalf("expected drift actions captured")
	}
}

func TestModelRuntimeHandlerResolveAndAdminListEndpoints(t *testing.T) {
	resolver := &runtimeResolverStub{resp: governance.ResolveDecision{RequestID: "req-1", ResolvedModel: "model-a"}}
	h := NewModelRuntimeHandler().WithResolver(resolver)

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/runtime/resolve", bytes.NewBufferString(`{"request_id":"req-1","tenant_id":"t1","environment":"prod","agent_id":"a1"}`))
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveResp := httptest.NewRecorder()
	h.ServeHTTP(resolveResp, resolveReq)
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resolveResp.Code, resolveResp.Body.String())
	}
	if resolver.input.RequestID != "req-1" || resolver.input.AgentID != "a1" {
		t.Fatalf("expected resolver input captured: %+v", resolver.input)
	}

	runtimeListReq := httptest.NewRequest(http.MethodGet, "/admin/governance/runtime-decisions", nil)
	runtimeListResp := httptest.NewRecorder()
	h.ServeHTTP(runtimeListResp, runtimeListReq)
	if runtimeListResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when queryer missing, got %d", runtimeListResp.Code)
	}

	distListReq := httptest.NewRequest(http.MethodGet, "/admin/governance/distribution-events", nil)
	distListResp := httptest.NewRecorder()
	h.ServeHTTP(distListResp, distListReq)
	if distListResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when queryer missing, got %d", distListResp.Code)
	}

	observerReq := httptest.NewRequest(http.MethodGet, "/admin/governance/runtime-observer", nil)
	observerResp := httptest.NewRecorder()
	h.ServeHTTP(observerResp, observerReq)
	if observerResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when queryer missing, got %d", observerResp.Code)
	}
}

func TestServerMountsGovernanceAndRuntimeRoutes(t *testing.T) {
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithModelGovernanceHandler(NewModelGovernanceHandler().
			WithRecommendationService(&recommendationServiceStub{resp: governance.Recommendation{ID: "rec-1"}}).
			WithVersionService(&versionServiceStub{diffResp: governance.PolicyVersionDiff{BaseType: "none", CurrentVersion: governance.PolicyVersion{ID: "pv-1"}}}).
			WithRolloutDashboardService(&rolloutDashboardServiceStub{rows: []governance.RolloutDashboardRow{{RolloutID: "ro-1"}}})).
		WithModelRuntimeHandler(NewModelRuntimeHandler().WithResolver(&runtimeResolverStub{resp: governance.ResolveDecision{RequestID: "req-1", ResolvedModel: "model-a"}}))

	unauth := httptest.NewRecorder()
	unauthReq := httptest.NewRequest(http.MethodPost, "/admin/governance/recommendations", bytes.NewBufferString(`{"agent_id":"a1","task_type":"chat","environment":"prod"}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(unauth, unauthReq)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauth.Code)
	}

	auth := httptest.NewRecorder()
	authReq := httptest.NewRequest(http.MethodPost, "/admin/governance/recommendations", bytes.NewBufferString(`{"agent_id":"a1","task_type":"chat","environment":"prod"}`))
	authReq.Header.Set("Content-Type", "application/json")
	authReq.Header.Set("X-Admin-Key", "k")
	s.Handler().ServeHTTP(auth, authReq)
	if auth.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", auth.Code, auth.Body.String())
	}

	resolve := httptest.NewRecorder()
	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/runtime/resolve", bytes.NewBufferString(`{"request_id":"req-1","environment":"prod","agent_id":"a1"}`))
	resolveReq.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(resolve, resolveReq)
	if resolve.Code != http.StatusOK {
		t.Fatalf("expected 200 for /v1/runtime/resolve, got %d body=%s", resolve.Code, resolve.Body.String())
	}

	observerUnauthorized := httptest.NewRecorder()
	observerUnauthorizedReq := httptest.NewRequest(http.MethodGet, "/admin/governance/runtime-observer", nil)
	s.Handler().ServeHTTP(observerUnauthorized, observerUnauthorizedReq)
	if observerUnauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized observer route, got %d", observerUnauthorized.Code)
	}

	observerAuthorized := httptest.NewRecorder()
	observerAuthorizedReq := httptest.NewRequest(http.MethodGet, "/admin/governance/runtime-observer", nil)
	observerAuthorizedReq.Header.Set("X-Admin-Key", "k")
	s.Handler().ServeHTTP(observerAuthorized, observerAuthorizedReq)
	if observerAuthorized.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for authorized observer route without queryer, got %d", observerAuthorized.Code)
	}

	dashboard := httptest.NewRecorder()
	dashboardReq := httptest.NewRequest(http.MethodGet, "/admin/governance/dashboard/rollouts", nil)
	dashboardReq.Header.Set("X-Admin-Key", "k")
	s.Handler().ServeHTTP(dashboard, dashboardReq)
	if dashboard.Code != http.StatusOK {
		t.Fatalf("expected 200 for /admin/governance/dashboard/rollouts, got %d body=%s", dashboard.Code, dashboard.Body.String())
	}

	diffResp := httptest.NewRecorder()
	diffReq := httptest.NewRequest(http.MethodGet, "/admin/governance/policy-versions/pv-1/diff", nil)
	diffReq.Header.Set("X-Admin-Key", "k")
	s.Handler().ServeHTTP(diffResp, diffReq)
	if diffResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for /admin/governance/policy-versions/:id/diff, got %d body=%s", diffResp.Code, diffResp.Body.String())
	}
}
