package controlplane

import "testing"

func TestCompensationRecordShape(t *testing.T) {
	r := CompensationRecord{
		Module:          "policy",
		TenantID:        "t1",
		Environment:     "prod",
		Version:         "v1",
		FailedStage:     FailedStagePromotionGate,
		ErrorSummary:    "gate failed",
		SuggestedAction: "retry manually",
	}
	if r.Module == "" || r.FailedStage == "" || r.SuggestedAction == "" {
		t.Fatalf("unexpected compensation record: %+v", r)
	}
}

func TestCompensationStageAndSuggestionShape(t *testing.T) {
	cases := []string{
		FailedStagePromotionGate,
		FailedStagePromotionValidation,
		FailedStageReleaseWrite,
		FailedStageReload,
		FailedStageConfigSync,
	}
	for _, stage := range cases {
		if SuggestedActionFor(stage) == "" {
			t.Fatalf("expected suggested action for stage %s", stage)
		}
	}
}
