package governance

import "testing"

func TestBuildPolicyDiff(t *testing.T) {
	base := RuntimePolicy{
		Version:      1,
		Environment:  "prod",
		DefaultModel: "gpt-4o-mini",
		Agents: map[string]AgentPolicy{
			"writer": {
				PrimaryModel:  "gpt-4o-mini",
				FallbackChain: []string{"gpt-4o"},
			},
		},
		Metadata: map[string]string{"owner": "ops"},
	}
	current := RuntimePolicy{
		Version:      2,
		Environment:  "prod",
		DefaultModel: "gpt-4.1-mini",
		Agents: map[string]AgentPolicy{
			"writer": {
				PrimaryModel:  "gpt-4.1-mini",
				FallbackChain: []string{"gpt-4o", "gpt-4.1"},
			},
			"reviewer": {
				PrimaryModel: "gpt-4.1",
			},
		},
		Metadata: map[string]string{"owner": "ml-platform"},
	}

	diff := buildPolicyDiff(current, &base)
	if len(diff) == 0 {
		t.Fatalf("expected non-empty diff")
	}

	foundModified := false
	foundAdded := false
	for _, item := range diff {
		if item.Path == "default_model" && item.ChangeType == "modified" {
			foundModified = true
		}
		if item.Path == "agents.reviewer.primary_model" && item.ChangeType == "added" {
			foundAdded = true
		}
	}
	if !foundModified {
		t.Fatalf("expected modified default_model in diff: %+v", diff)
	}
	if !foundAdded {
		t.Fatalf("expected added reviewer policy in diff: %+v", diff)
	}
}

func TestBuildPolicyDiffWithoutBase(t *testing.T) {
	current := RuntimePolicy{Version: 3, Environment: "staging", DefaultModel: "gpt-4.1"}
	diff := buildPolicyDiff(current, nil)
	if len(diff) == 0 {
		t.Fatalf("expected diff entries when base is nil")
	}
	for _, item := range diff {
		if item.ChangeType != "added" {
			t.Fatalf("expected only added entries when base=nil, got %+v", diff)
		}
	}
}
