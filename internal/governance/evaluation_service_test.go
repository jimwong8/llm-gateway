package governance_test

import (
	"context"
	"testing"

	"llm-gateway/gateway/internal/governance"
)

func newEvaluationServiceForTest(t *testing.T) *governance.EvaluationService {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewEvaluationService(store)
}

func TestEvaluationRunLifecycle(t *testing.T) {
	svc := newEvaluationServiceForTest(t)
	ctx := context.Background()

	datasetID, err := svc.CreateDataset(ctx, governance.EvaluationDatasetInput{
		Name:     "security-audit-core",
		Version:  "v1",
		TaskType: "security_audit",
	})
	if err != nil {
		t.Fatalf("CreateDataset() error = %v", err)
	}
	if datasetID == "" {
		t.Fatalf("CreateDataset() returned empty id")
	}

	formulaID, err := svc.CreateFormula(ctx, governance.ScoringFormulaInput{
		Version:     "v1",
		FormulaJSON: []byte(`{"quality":0.4,"cost":0.2}`),
	})
	if err != nil {
		t.Fatalf("CreateFormula() error = %v", err)
	}
	if formulaID == "" {
		t.Fatalf("CreateFormula() returned empty id")
	}

	run, err := svc.StartRun(ctx, governance.StartEvaluationRunInput{
		DatasetID:        datasetID,
		AgentID:          "security-reviewer",
		TaskType:         "security_audit",
		Environment:      "staging",
		FormulaVersionID: formulaID,
	})
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	if run.Status != governance.EvaluationRunStatusRunning {
		t.Fatalf("unexpected run status: %s", run.Status)
	}
	if run.DatasetID != datasetID {
		t.Fatalf("unexpected dataset id: %s", run.DatasetID)
	}
	if run.FormulaVersionID != formulaID {
		t.Fatalf("unexpected formula id: %s", run.FormulaVersionID)
	}
	if run.TaskType != "security_audit" {
		t.Fatalf("unexpected task type: %s", run.TaskType)
	}

	finished, err := svc.UpdateRunStatus(ctx, run.ID, governance.EvaluationRunStatusSucceeded)
	if err != nil {
		t.Fatalf("UpdateRunStatus() error = %v", err)
	}
	if finished.Status != governance.EvaluationRunStatusSucceeded {
		t.Fatalf("unexpected finished status: %s", finished.Status)
	}
	if finished.FinishedAt.IsZero() {
		t.Fatalf("expected finished_at to be set for terminal status")
	}
}
