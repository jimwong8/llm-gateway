# Model Governance Platform Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a production-usable model governance platform inside the current gateway repository, covering recommendation, human approval/override, policy versioning, rollout/rollback, runtime resolution, decision snapshots, audit, evaluation registry, and drift detection.

**Architecture:** Reuse the existing control-plane/runtime split already present in this repository. Introduce model-governance as a first-class control-plane domain that publishes immutable policy versions, while runtime resolution consumes published versions, emits immutable decision snapshots, and feeds metrics/rollback decisions back into governance workflows.

**Tech Stack:** Go, PostgreSQL, existing internal/controlplane + internal/runtime patterns, net/http admin endpoints, existing verify command pattern under `cmd/verify`, repository-local SQL migrations.

---

## File Structure

### Existing files to extend
- Modify: `cmd/server_main.go` — wire governance stores/services/handlers into the existing server startup.
- Modify: `internal/config/config.go` — add governance feature flags, defaults, rollout guard thresholds, and cache TTLs.
- Modify: `internal/httpserver/server.go` — mount governance admin endpoints and runtime resolve endpoint.
- Modify: `internal/controlplane/service.go` — optionally host governance version activation/replay hooks if existing release flow can be reused.
- Modify: `internal/runtime/bus.go` — add governance event types if needed for policy distribution and cache invalidation.
- Modify: `internal/runtime/reload.go` — extend runtime status tracking to include governance policy distribution health.
- Modify: `README.md` — document governance verify commands.

### New Go packages
- Create: `internal/governance/model.go` — domain structs for recommendations, approvals, versions, rollouts, snapshots, drifts.
- Create: `internal/governance/postgres.go` — shared governance DB bootstrapping/migrations helper if needed.
- Create: `internal/governance/recommendation_service.go` — recommendation generation orchestration.
- Create: `internal/governance/approval_service.go` — approval / override / reject logic.
- Create: `internal/governance/version_service.go` — policy version creation, approval, activation.
- Create: `internal/governance/rollout_service.go` — rollout start/promote/finalize state machine.
- Create: `internal/governance/rollback_service.go` — rollback execution.
- Create: `internal/governance/runtime_resolver.go` — runtime model resolution against active policy.
- Create: `internal/governance/scope_priority.go` — deterministic scope precedence resolver.
- Create: `internal/governance/distribution_service.go` — policy distribution event publication + delivery state.
- Create: `internal/governance/metrics_service.go` — aggregate rollout metrics windows from decision snapshots.
- Create: `internal/governance/drift_service.go` — drift detection between active policy and latest recommendation.
- Create: `internal/governance/audit_service.go` — governance audit event recording wrapper.
- Create: `internal/governance/evaluation_service.go` — evaluation run lifecycle and scoring formula handling.

### New repositories (can be in same package if codebase style prefers fewer files)
- Create: `internal/governance/recommendation_repo.go`
- Create: `internal/governance/approval_repo.go`
- Create: `internal/governance/version_repo.go`
- Create: `internal/governance/rollout_repo.go`
- Create: `internal/governance/snapshot_repo.go`
- Create: `internal/governance/evaluation_repo.go`
- Create: `internal/governance/drift_repo.go`

### New HTTP handlers
- Create: `internal/httpserver/model_governance_handler.go` — recommendation, approval, policy version, rollout, drift, evaluation APIs.
- Create: `internal/httpserver/model_runtime_handler.go` — `/runtime/model-policy/resolve` and runtime trace lookup APIs.
- Create: `internal/httpserver/model_governance_handler_test.go`
- Create: `internal/httpserver/model_runtime_handler_test.go`

### SQL migrations
- Create: `internal/db/migrations/004_model_governance_init.sql`
- Create: `internal/db/migrations/005_model_governance_constraints.sql`
- Create: `internal/db/migrations/006_model_governance_runtime.sql`
- Create: `internal/db/migrations/007_model_governance_evaluations.sql`
- Create: `internal/db/migrations/008_model_governance_distribution_and_drift.sql`

### Verify / smoke commands
- Create: `cmd/verify/model_governance/main.go` — end-to-end governance happy path.
- Create: `cmd/verify/model_governance_runtime/main.go` — runtime resolve + snapshot + metrics path.
- Create: `cmd/verify/model_governance_rollback/main.go` — rollout guard + rollback path.

### Documentation
- Create: `docs/superpowers/plans/2026-04-19-model-governance-platform.md` (this file)
- Modify: `RUNBOOK.md` — add governance operational runbook after implementation.

---

## Chunk 1: Database and domain foundations

### Task 1: Add governance schema migrations

**Files:**
- Create: `internal/db/migrations/004_model_governance_init.sql`
- Create: `internal/db/migrations/005_model_governance_constraints.sql`
- Create: `internal/db/migrations/006_model_governance_runtime.sql`
- Create: `internal/db/migrations/007_model_governance_evaluations.sql`
- Create: `internal/db/migrations/008_model_governance_distribution_and_drift.sql`
- Test: `internal/governance/postgres_test.go`

- [ ] **Step 1: Write the failing migration bootstrapping test**

```go
func TestGovernanceStoreBootstrapsSchema(t *testing.T) {
    dsn := testPostgresDSN(t)
    store, err := governance.NewStore(dsn)
    if err != nil {
        t.Fatalf("NewStore() error = %v", err)
    }
    tables := []string{
        "model_recommendations",
        "model_approvals",
        "model_policy_versions",
        "model_rollouts",
        "runtime_decision_snapshots",
        "evaluation_runs",
        "policy_drifts",
    }
    for _, table := range tables {
        if !store.TableExists(t.Context(), table) {
            t.Fatalf("expected table %s to exist", table)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestGovernanceStoreBootstrapsSchema -v`
Expected: FAIL with missing package / missing schema logic.

- [ ] **Step 3: Write minimal migration files and bootstrapping code**

Implement SQL for:
- recommendations
- approvals
- policy versions
- rollouts
- audit logs
- runtime decision snapshots
- evaluation datasets/runs/results/formulas
- distribution events
- drifts

Implement `NewStore(dsn)` to run migrations idempotently.

- [ ] **Step 4: Run the targeted test**

Run: `go test ./internal/governance -run TestGovernanceStoreBootstrapsSchema -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations internal/governance/postgres.go internal/governance/postgres_test.go
git commit -m "feat: add model governance schema"
```

### Task 2: Define governance domain models and enums

**Files:**
- Create: `internal/governance/model.go`
- Test: `internal/governance/model_test.go`

- [ ] **Step 1: Write the failing domain serialization test**

```go
func TestPolicyVersionJSONRoundTrip(t *testing.T) {
    version := governance.PolicyVersion{
        Status: governance.PolicyVersionApproved,
        Policy: governance.RuntimePolicy{
            Version: 12,
            Environment: "prod",
            Agents: map[string]governance.AgentPolicy{
                "security-reviewer": {
                    PrimaryModel: "model-x",
                    FallbackChain: []string{"model-y"},
                },
            },
        },
    }
    raw, err := json.Marshal(version)
    if err != nil {
        t.Fatalf("marshal error = %v", err)
    }
    var decoded governance.PolicyVersion
    if err := json.Unmarshal(raw, &decoded); err != nil {
        t.Fatalf("unmarshal error = %v", err)
    }
    if decoded.Policy.Agents["security-reviewer"].PrimaryModel != "model-x" {
        t.Fatalf("unexpected primary model: %+v", decoded)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestPolicyVersionJSONRoundTrip -v`
Expected: FAIL with missing types.

- [ ] **Step 3: Implement domain models**

Include:
- Recommendation / CandidateModel / ScoreBreakdown
- Approval / EffectiveScope
- PolicyVersion / RuntimePolicy / AgentPolicy
- Rollout / RolloutMetricsSnapshot
- EvaluationDataset / EvaluationRun / EvaluationResult
- RuntimeDecisionSnapshot
- PolicyDrift
- String enums and validation helpers

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestPolicyVersionJSONRoundTrip -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/model.go internal/governance/model_test.go
git commit -m "feat: add model governance domain types"
```

---

## Chunk 2: Evaluation registry and recommendation engine

### Task 3: Implement evaluation registry persistence

**Files:**
- Create: `internal/governance/evaluation_repo.go`
- Create: `internal/governance/evaluation_service.go`
- Test: `internal/governance/evaluation_service_test.go`

- [ ] **Step 1: Write the failing evaluation lifecycle test**

```go
func TestEvaluationRunLifecycle(t *testing.T) {
    svc := newEvaluationServiceForTest(t)
    datasetID := svc.CreateDataset(t.Context(), governance.EvaluationDatasetInput{
        Name: "security-audit-core",
        Version: "v1",
        TaskType: "security_audit",
    })
    formulaID := svc.CreateFormula(t.Context(), governance.ScoringFormulaInput{
        Version: "v1",
        FormulaJSON: []byte(`{"quality":0.4,"cost":0.2}`),
    })
    run, err := svc.StartRun(t.Context(), governance.StartEvaluationRunInput{
        DatasetID: datasetID,
        AgentID: "security-reviewer",
        TaskType: "security_audit",
        Environment: "staging",
        FormulaVersionID: formulaID,
    })
    if err != nil { t.Fatalf("StartRun() error = %v", err) }
    if run.Status != governance.EvaluationRunStatusRunning { t.Fatalf("unexpected status: %s", run.Status) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestEvaluationRunLifecycle -v`
Expected: FAIL with missing repo/service methods.

- [ ] **Step 3: Implement dataset/formula/run repositories and service methods**

Keep this slice limited to CRUD/lifecycle only. Do not add recommendation logic yet.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestEvaluationRunLifecycle -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/evaluation_repo.go internal/governance/evaluation_service.go internal/governance/evaluation_service_test.go
git commit -m "feat: add evaluation registry lifecycle"
```

### Task 4: Implement recommendation generation using evaluation results

**Files:**
- Create: `internal/governance/recommendation_repo.go`
- Create: `internal/governance/recommendation_service.go`
- Test: `internal/governance/recommendation_service_test.go`

- [ ] **Step 1: Write the failing recommendation test**

```go
func TestGenerateRecommendationPicksBestScoringModel(t *testing.T) {
    svc := newRecommendationServiceForTest(t)
    seedEvaluationResults(t, svc.Store(), []governance.EvaluationResultSeed{
        {Model: "model-a", FinalScore: 0.88},
        {Model: "model-b", FinalScore: 0.93},
    })
    rec, err := svc.Generate(t.Context(), governance.GenerateRecommendationInput{
        AgentID: "code-reviewer",
        TaskType: "code_review",
        Environment: "prod",
    })
    if err != nil { t.Fatalf("Generate() error = %v", err) }
    if rec.RecommendedModel != "model-b" { t.Fatalf("expected model-b, got %s", rec.RecommendedModel) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestGenerateRecommendationPicksBestScoringModel -v`
Expected: FAIL.

- [ ] **Step 3: Implement minimal recommendation pipeline**

Rules:
- Use latest successful evaluation run per agent/task/environment.
- Sort by final score descending.
- Store candidate list and score breakdown.
- Mark `approval_required=true` by default.
- Do not auto-activate.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestGenerateRecommendationPicksBestScoringModel -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/recommendation_repo.go internal/governance/recommendation_service.go internal/governance/recommendation_service_test.go
git commit -m "feat: add model recommendation service"
```

---

## Chunk 3: Approval and policy versioning

### Task 5: Implement approval workflow with override/reject

**Files:**
- Create: `internal/governance/approval_repo.go`
- Create: `internal/governance/approval_service.go`
- Test: `internal/governance/approval_service_test.go`

- [ ] **Step 1: Write the failing approval test**

```go
func TestApproveRecommendationWithOverride(t *testing.T) {
    svc := newApprovalServiceForTest(t)
    rec := seedRecommendation(t, svc.Store(), "security-reviewer", "model-a", "model-b")
    approval, err := svc.Decide(t.Context(), governance.ApprovalInput{
        RecommendationID: rec.ID,
        Decision: governance.ApprovalDecisionOverridden,
        FinalModel: "model-c",
        ApprovalReason: "cost too high",
        ApprovedBy: "alice",
        EffectiveScope: governance.EffectiveScope{Environment: "prod", ScopeType: "agent", AgentID: ptr("security-reviewer")},
    })
    if err != nil { t.Fatalf("Decide() error = %v", err) }
    if approval.FinalModel != "model-c" { t.Fatalf("unexpected final model: %s", approval.FinalModel) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestApproveRecommendationWithOverride -v`
Expected: FAIL.

- [ ] **Step 3: Implement approval rules**

Rules:
- Only pending recommendation may be approved/rejected.
- Override requires final model.
- Reject requires reason.
- Recommendation status transitions to approved/rejected.
- Emit governance audit event.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestApproveRecommendationWithOverride -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/approval_repo.go internal/governance/approval_service.go internal/governance/approval_service_test.go
git commit -m "feat: add approval workflow"
```

### Task 6: Implement policy version creation and approval

**Files:**
- Create: `internal/governance/version_repo.go`
- Create: `internal/governance/version_service.go`
- Test: `internal/governance/version_service_test.go`

- [ ] **Step 1: Write the failing version-from-approval test**

```go
func TestCreatePolicyVersionFromApproval(t *testing.T) {
    svc := newVersionServiceForTest(t)
    approval := seedApproval(t, svc.Store(), governance.SeedApprovalInput{
        AgentID: "code-reviewer",
        FinalModel: "model-b",
        Scope: governance.EffectiveScope{Environment: "prod", ScopeType: "agent", AgentID: ptr("code-reviewer")},
    })
    version, err := svc.CreateFromApproval(t.Context(), approval.ID, "alice")
    if err != nil { t.Fatalf("CreateFromApproval() error = %v", err) }
    if version.Status != governance.PolicyVersionStatusDraft { t.Fatalf("unexpected status: %s", version.Status) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestCreatePolicyVersionFromApproval -v`
Expected: FAIL.

- [ ] **Step 3: Implement version creation/approval/activation preconditions**

Rules:
- Create draft from approved approval.
- Store immutable policy JSON snapshot.
- Approve version before activation.
- Keep one active version per environment at a time.
- Do not perform rollout here.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestCreatePolicyVersionFromApproval -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/version_repo.go internal/governance/version_service.go internal/governance/version_service_test.go
git commit -m "feat: add policy versioning"
```

---

## Chunk 4: Runtime resolution and immutable decision snapshots

### Task 7: Implement deterministic scope priority resolver

**Files:**
- Create: `internal/governance/scope_priority.go`
- Test: `internal/governance/scope_priority_test.go`

- [ ] **Step 1: Write the failing precedence test**

```go
func TestResolveScopePriority(t *testing.T) {
    matches := []governance.ScopeMatch{
        {ScopeType: governance.ScopeGlobal},
        {ScopeType: governance.ScopeEnvironment},
        {ScopeType: governance.ScopeTenantAgent},
    }
    chosen := governance.ResolveHighestPriorityScope(matches)
    if chosen.ScopeType != governance.ScopeTenantAgent {
        t.Fatalf("expected tenant+agent, got %s", chosen.ScopeType)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestResolveScopePriority -v`
Expected: FAIL.

- [ ] **Step 3: Implement fixed precedence**

Precedence order:
1. emergency override
2. tenant+agent
3. tenant
4. agent
5. task_type
6. environment
7. global

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestResolveScopePriority -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/scope_priority.go internal/governance/scope_priority_test.go
git commit -m "feat: add governance scope priority resolver"
```

### Task 8: Implement runtime resolver + decision snapshot recording

**Files:**
- Create: `internal/governance/runtime_resolver.go`
- Create: `internal/governance/snapshot_repo.go`
- Test: `internal/governance/runtime_resolver_test.go`

- [ ] **Step 1: Write the failing runtime resolution test**

```go
func TestResolveRuntimePolicyAndWriteSnapshot(t *testing.T) {
    svc := newRuntimeResolverForTest(t)
    seedActivePolicy(t, svc.Store(), governance.RuntimePolicySeed{
        Environment: "prod",
        AgentID: "security-reviewer",
        PrimaryModel: "model-x",
        FallbackChain: []string{"model-y"},
    })
    decision, err := svc.Resolve(t.Context(), governance.ResolveInput{
        RequestID: mustUUID("11111111-1111-1111-1111-111111111111"),
        Environment: "prod",
        AgentID: "security-reviewer",
        TenantID: "tenant-a",
    })
    if err != nil { t.Fatalf("Resolve() error = %v", err) }
    if decision.ResolvedModel != "model-x" { t.Fatalf("unexpected model: %s", decision.ResolvedModel) }
    if !snapshotExists(t, svc.Store(), decision.RequestID) { t.Fatalf("expected snapshot persisted") }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestResolveRuntimePolicyAndWriteSnapshot -v`
Expected: FAIL.

- [ ] **Step 3: Implement runtime resolver and snapshot persistence**

Requirements:
- Load active policy for environment.
- Filter matching scopes.
- Use scope priority resolver.
- Return resolved model + fallback chain + matched scope.
- Persist immutable runtime_decision_snapshots row.
- Distinguish policy fallback vs system fallback fields.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestResolveRuntimePolicyAndWriteSnapshot -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/runtime_resolver.go internal/governance/snapshot_repo.go internal/governance/runtime_resolver_test.go
git commit -m "feat: add runtime resolution and decision snapshots"
```

---

## Chunk 5: Rollout, metrics, rollback, distribution

### Task 9: Implement rollout controller and policy distribution events

**Files:**
- Create: `internal/governance/rollout_repo.go`
- Create: `internal/governance/rollout_service.go`
- Create: `internal/governance/distribution_service.go`
- Test: `internal/governance/rollout_service_test.go`

- [ ] **Step 1: Write the failing rollout start test**

```go
func TestStartRolloutPublishesDistributionEvent(t *testing.T) {
    svc := newRolloutServiceForTest(t)
    version := seedApprovedPolicyVersion(t, svc.Store(), "prod")
    rollout, err := svc.Start(t.Context(), governance.StartRolloutInput{
        PolicyVersionID: version.ID,
        Environment: "prod",
        RolloutMode: governance.RolloutModeShadow,
        Percent: 5,
        TriggeredBy: "alice",
    })
    if err != nil { t.Fatalf("Start() error = %v", err) }
    if rollout.Status != governance.RolloutStatusRunning { t.Fatalf("unexpected rollout status: %s", rollout.Status) }
    if !distributionEventExists(t, svc.Store(), version.ID) { t.Fatalf("expected distribution event") }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestStartRolloutPublishesDistributionEvent -v`
Expected: FAIL.

- [ ] **Step 3: Implement rollout start/promote/finalize and event publication**

Rules:
- Only approved policy version can start rollout.
- Shadow/canary/full are explicit states.
- Distribution event created on activation and rollback.
- Promotion changes percent, not policy contents.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestStartRolloutPublishesDistributionEvent -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/rollout_repo.go internal/governance/rollout_service.go internal/governance/distribution_service.go internal/governance/rollout_service_test.go
git commit -m "feat: add rollout control and distribution events"
```

### Task 10: Implement metrics aggregation + rollout guard + rollback executor

**Files:**
- Create: `internal/governance/metrics_service.go`
- Create: `internal/governance/rollback_service.go`
- Test: `internal/governance/rollback_service_test.go`

- [ ] **Step 1: Write the failing rollback trigger test**

```go
func TestRollbackOnHighErrorRate(t *testing.T) {
    svc := newRollbackFlowForTest(t)
    rollout := seedRunningRollout(t, svc.Store(), "prod", 25)
    seedDecisionSnapshots(t, svc.Store(), rollout.ID, governance.RolloutMetricsSeed{
        ErrorRate: 0.10,
        P95LatencyMS: 900,
        FallbackRate: 0.01,
        SampleCount: 500,
    })
    verdict, err := svc.Guard().Evaluate(t.Context(), rollout.ID)
    if err != nil { t.Fatalf("Evaluate() error = %v", err) }
    if verdict.Action != governance.RolloutActionRollbackRequired {
        t.Fatalf("expected rollback required, got %s", verdict.Action)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestRollbackOnHighErrorRate -v`
Expected: FAIL.

- [ ] **Step 3: Implement metrics aggregation and rollback executor**

Requirements:
- Aggregate by rollout_id and policy_version_id.
- Output error_rate, p95, fallback_rate, sample_count.
- Guard returns keep / pause / rollback_suggested / rollback_required.
- Rollback executor flips active version back, writes audit event, publishes distribution event.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestRollbackOnHighErrorRate -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/metrics_service.go internal/governance/rollback_service.go internal/governance/rollback_service_test.go
git commit -m "feat: add rollout guard and rollback executor"
```

---

## Chunk 6: Drift detection and audit wrappers

### Task 11: Implement drift detector

**Files:**
- Create: `internal/governance/drift_repo.go`
- Create: `internal/governance/drift_service.go`
- Test: `internal/governance/drift_service_test.go`

- [ ] **Step 1: Write the failing drift test**

```go
func TestDetectModelMismatchDrift(t *testing.T) {
    svc := newDriftServiceForTest(t)
    seedActivePolicyModel(t, svc.Store(), "prod", "code-reviewer", "model-a")
    seedLatestRecommendation(t, svc.Store(), "prod", "code-reviewer", "model-b")
    drift, err := svc.Detect(t.Context(), governance.DriftDetectInput{Environment: "prod", AgentID: "code-reviewer"})
    if err != nil { t.Fatalf("Detect() error = %v", err) }
    if drift.DriftType != governance.DriftTypeModelMismatch { t.Fatalf("unexpected drift type: %s", drift.DriftType) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestDetectModelMismatchDrift -v`
Expected: FAIL.

- [ ] **Step 3: Implement drift detection**

Start simple:
- Compare active model vs latest recommendation.
- Record open drift if mismatch.
- Support acknowledge/resolve transitions.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestDetectModelMismatchDrift -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/drift_repo.go internal/governance/drift_service.go internal/governance/drift_service_test.go
git commit -m "feat: add policy drift detection"
```

### Task 12: Add governance audit service wrappers

**Files:**
- Create: `internal/governance/audit_service.go`
- Test: `internal/governance/audit_service_test.go`

- [ ] **Step 1: Write the failing audit wrapper test**

```go
func TestAuditServiceWritesGovernanceEvent(t *testing.T) {
    recorder := audit.NewRecorder()
    svc := governance.NewAuditService(recorder)
    err := svc.RecordRecommendationGenerated(t.Context(), governance.AuditRecommendationGeneratedInput{
        RecommendationID: mustUUID("11111111-1111-1111-1111-111111111111"),
        AgentID: "code-reviewer",
        RecommendedModel: "model-b",
        ActorID: "system",
    })
    if err != nil { t.Fatalf("RecordRecommendationGenerated() error = %v", err) }
    if len(recorder.Events()) != 1 { t.Fatalf("expected 1 event, got %d", len(recorder.Events())) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/governance -run TestAuditServiceWritesGovernanceEvent -v`
Expected: FAIL.

- [ ] **Step 3: Implement thin wrapper methods for key events**

Include:
- recommendation_generated
- approval_decided
- policy_version_created
- rollout_started
- rollout_promoted
- rollback_executed
- drift_detected

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/governance -run TestAuditServiceWritesGovernanceEvent -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/governance/audit_service.go internal/governance/audit_service_test.go
git commit -m "feat: add governance audit wrappers"
```

---

## Chunk 7: HTTP APIs and server wiring

### Task 13: Add model governance admin handler APIs

**Files:**
- Create: `internal/httpserver/model_governance_handler.go`
- Create: `internal/httpserver/model_governance_handler_test.go`
- Modify: `internal/httpserver/server.go`

- [ ] **Step 1: Write the failing HTTP handler test**

```go
func TestGenerateRecommendationEndpoint(t *testing.T) {
    srv := newGovernanceHTTPServerForTest(t)
    body := strings.NewReader(`{"agent_id":"code-reviewer","task_type":"code_review","environment":"staging"}`)
    req := httptest.NewRequest(http.MethodPost, "/admin/model-governance/recommendations/generate", body)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Admin-Key", "admin-dev-key")
    rr := httptest.NewRecorder()

    srv.Handler().ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestGenerateRecommendationEndpoint -v`
Expected: FAIL with route missing.

- [ ] **Step 3: Implement governance endpoints**

Minimum endpoints:
- POST `/admin/model-governance/recommendations/generate`
- GET `/admin/model-governance/recommendations`
- GET `/admin/model-governance/recommendations/{id}`
- POST `/admin/model-governance/approvals`
- POST `/admin/model-governance/policy-versions/from-approval/{approvalID}`
- GET `/admin/model-governance/policy-versions`
- GET `/admin/model-governance/policy-versions/{id}`
- POST `/admin/model-governance/policy-versions/{id}/approve`
- POST `/admin/model-governance/policy-versions/{id}/activate`
- POST `/admin/model-governance/rollouts`
- GET `/admin/model-governance/rollouts/{id}`
- POST `/admin/model-governance/rollouts/{id}/promote`
- POST `/admin/model-governance/rollouts/{id}/rollback`
- GET `/admin/model-governance/evaluations/runs`
- GET `/admin/model-governance/drifts`

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/httpserver -run TestGenerateRecommendationEndpoint -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/model_governance_handler.go internal/httpserver/model_governance_handler_test.go internal/httpserver/server.go
git commit -m "feat: add model governance admin APIs"
```

### Task 14: Add runtime resolve and trace endpoints

**Files:**
- Create: `internal/httpserver/model_runtime_handler.go`
- Create: `internal/httpserver/model_runtime_handler_test.go`
- Modify: `internal/httpserver/server.go`

- [ ] **Step 1: Write the failing runtime resolve handler test**

```go
func TestRuntimeResolveEndpoint(t *testing.T) {
    srv := newGovernanceHTTPServerForTest(t)
    req := httptest.NewRequest(http.MethodGet, "/runtime/model-policy/resolve?environment=prod&agent_id=security-reviewer", nil)
    rr := httptest.NewRecorder()
    srv.Handler().ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr.Code)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestRuntimeResolveEndpoint -v`
Expected: FAIL with route missing.

- [ ] **Step 3: Implement runtime endpoints**

Endpoints:
- GET `/runtime/model-policy/resolve`
- GET `/admin/model-governance/runtime-decisions/{requestID}`
- GET `/admin/model-governance/distribution-events`

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/httpserver -run TestRuntimeResolveEndpoint -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/model_runtime_handler.go internal/httpserver/model_runtime_handler_test.go internal/httpserver/server.go
git commit -m "feat: add runtime resolve and trace APIs"
```

### Task 15: Wire governance services into server startup

**Files:**
- Modify: `cmd/server_main.go`
- Modify: `internal/config/config.go`
- Test: `cmd/server_main_test.go`

- [ ] **Step 1: Write the failing startup wiring test**

```go
func TestServerMainWiresGovernanceServices(t *testing.T) {
    cfg := config.Load()
    app := buildServerForTest(cfg)
    if app.GovernanceHandler() == nil {
        t.Fatal("expected governance handler wired")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd -run TestServerMainWiresGovernanceServices -v`
Expected: FAIL.

- [ ] **Step 3: Add config flags and wire store/services/handlers**

Config additions:
- `MODEL_GOVERNANCE_ENABLED`
- `MODEL_GOVERNANCE_CACHE_TTL_SECONDS`
- `MODEL_GOVERNANCE_ROLLOUT_MAX_ERROR_RATE`
- `MODEL_GOVERNANCE_ROLLOUT_MAX_P95_MS`
- `MODEL_GOVERNANCE_ROLLOUT_MAX_FALLBACK_RATE`
- `MODEL_GOVERNANCE_MIN_SAMPLE_COUNT`

- [ ] **Step 4: Run targeted tests**

Run: `go test ./cmd -run TestServerMainWiresGovernanceServices -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/server_main.go internal/config/config.go cmd/server_main_test.go
git commit -m "feat: wire model governance services"
```

---

## Chunk 8: Verify commands and repository-wide validation

### Task 16: Add verify commands for governance happy path

**Files:**
- Create: `cmd/verify/model_governance/main.go`
- Create: `cmd/verify/model_governance_runtime/main.go`
- Create: `cmd/verify/model_governance_rollback/main.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing verify smoke test**

```go
func TestModelGovernanceVerifyCommandRuns(t *testing.T) {
    cmd := exec.Command("go", "run", "./cmd/verify/model_governance")
    out, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("verify command failed: %v\n%s", err, string(out))
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/verify/... -run TestModelGovernanceVerifyCommandRuns -v`
Expected: FAIL.

- [ ] **Step 3: Implement verify commands**

Commands must cover:
- recommendation -> approval -> version -> rollout
- runtime resolve writes snapshot
- metrics trigger rollback

- [ ] **Step 4: Run targeted tests**

Run: `go test ./cmd/verify/... -run TestModelGovernanceVerifyCommandRuns -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/verify/model_governance cmd/verify/model_governance_runtime cmd/verify/model_governance_rollback README.md
git commit -m "feat: add model governance verify commands"
```

### Task 17: Run full validation and fix integration issues

**Files:**
- Modify: any failing file from previous tasks

- [ ] **Step 1: Run focused governance tests**

Run: `go test ./internal/governance ./internal/httpserver ./cmd/... -v`
Expected: PASS.

- [ ] **Step 2: Run repository smoke**

Run: `go run ./cmd/verify/model_governance && go run ./cmd/verify/model_governance_runtime && go run ./cmd/verify/model_governance_rollback`
Expected: PASS.

- [ ] **Step 3: Run full repository tests**

Run: `go test ./...`
Expected: PASS, or only documented pre-existing failures unrelated to governance.

- [ ] **Step 4: Update README and runbook notes if needed**

Document new commands and admin endpoints.

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "feat: finalize model governance platform"
```

---

## Notes for implementation workers

- Prefer reusing existing `internal/controlplane`, `internal/runtime`, and `internal/httpserver` patterns over inventing new frameworks.
- Keep governance policy versioning separate from existing router bootstrap file mechanics; treat them as adjacent systems until a later unification step is explicitly needed.
- Use the existing admin auth pattern (`X-Admin-Key` / bearer token) for governance admin endpoints.
- Keep runtime resolve read-only with respect to configuration; it may write decision snapshots, but must never mutate policy state.
- Do not implement UI in this plan. First land backend, verify commands, and API contracts.
- If a verify command needs fixture data, create it inside the command rather than relying on undocumented external state.

---

Plan complete and saved to `docs/superpowers/plans/2026-04-19-model-governance-platform.md`. Ready to execute?
