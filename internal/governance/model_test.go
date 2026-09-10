package governance_test

import (
	"encoding/json"
	"testing"
	"time"

	"llm-gateway/gateway/internal/governance"
)

func TestPolicyVersionJSONRoundTrip(t *testing.T) {
	version := governance.PolicyVersion{
		ID:          "pv-12",
		TenantID:    "t1",
		Environment: "prod",
		Version:     12,
		Status:      governance.PolicyVersionApproved,
		Policy: governance.RuntimePolicy{
			Version:      12,
			Environment:  "prod",
			DefaultModel: "model-default",
			Agents: map[string]governance.AgentPolicy{
				"security-reviewer": {
					PrimaryModel:  "model-x",
					FallbackChain: []string{"model-y"},
				},
			},
			Metadata: map[string]string{"source": "evaluation"},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
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
	if decoded.Status != governance.PolicyVersionApproved {
		t.Fatalf("unexpected status: %s", decoded.Status)
	}
	if decoded.Policy.Metadata["source"] != "evaluation" {
		t.Fatalf("unexpected metadata: %+v", decoded.Policy.Metadata)
	}
}

func TestEnumValidityAndTerminalBehavior(t *testing.T) {
	if !governance.RecommendationStatusReady.Valid() {
		t.Fatalf("expected recommendation status to be valid")
	}
	if governance.RecommendationStatus("bad").Valid() {
		t.Fatalf("unexpected valid recommendation status")
	}

	if !governance.ApprovalStatusOverridden.Valid() {
		t.Fatalf("expected approval status overridden to be valid")
	}
	if governance.ApprovalStatus("xxx").Valid() {
		t.Fatalf("unexpected valid approval status")
	}

	if !governance.PolicyVersionActive.Valid() {
		t.Fatalf("expected active policy status to be valid")
	}
	if governance.PolicyVersionStatus("nope").Valid() {
		t.Fatalf("unexpected valid policy version status")
	}

	if !governance.RolloutStatusFinalized.IsTerminal() {
		t.Fatalf("expected finalized rollout to be terminal")
	}
	if governance.RolloutStatusRunning.IsTerminal() {
		t.Fatalf("expected running rollout to be non-terminal")
	}

	if !governance.EvaluationRunStatusFailed.IsTerminal() {
		t.Fatalf("expected failed run to be terminal")
	}
	if governance.EvaluationRunStatusRunning.IsTerminal() {
		t.Fatalf("expected running run to be non-terminal")
	}

	if !governance.PolicyDriftStatusDetected.Valid() {
		t.Fatalf("expected drift status detected to be valid")
	}
	if governance.PolicyDriftStatus("other").Valid() {
		t.Fatalf("unexpected valid drift status")
	}
}
