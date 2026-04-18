package router

import (
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestParsePolicyConfig_Direct(t *testing.T) {
	raw := []byte(`{"type": "direct", "model": "gpt-4"}`)
	p, err := ParsePolicyConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dp, ok := p.(*DirectPolicy)
	if !ok {
		t.Fatalf("expected DirectPolicy, got %T", p)
	}
	if dp.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", dp.Model)
	}
}

func TestParsePolicyConfig_LoadBalance(t *testing.T) {
	raw := []byte(`{
		"type": "load_balance",
		"weights": {
			"gpt-4": 0.7,
			"claude-3": 0.3
		}
	}`)
	p, err := ParsePolicyConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lbp, ok := p.(*LoadBalancePolicy)
	if !ok {
		t.Fatalf("expected LoadBalancePolicy, got %T", p)
	}
	if len(lbp.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(lbp.Targets))
	}
}

func TestParsePolicyConfig_Fallback(t *testing.T) {
	raw := []byte(`{
		"type": "fallback",
		"targets": [
			{"type": "direct", "model": "gpt-4"},
			{"type": "direct", "model": "gpt-3.5-turbo"}
		]
	}`)
	p, err := ParsePolicyConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fp, ok := p.(*FallbackPolicy)
	if !ok {
		t.Fatalf("expected FallbackPolicy, got %T", p)
	}
	if len(fp.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(fp.Targets))
	}
}

func TestPolicyExecute(t *testing.T) {
	r := New("default-prov", "default-model")

	// Test Direct
	dp := &DirectPolicy{Model: "gpt-4o-mini"}
	score, err := dp.Execute(providers.ChatCompletionRequest{}, r)
	if err != nil {
		t.Fatalf("direct policy failed: %v", err)
	}
	if score.Model != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini, got %s", score.Model)
	}

	// Test LoadBalance
	lbp := &LoadBalancePolicy{
		Targets: []lbTarget{
			{key: "gpt-4o-mini", weight: 1.0},
		},
	}
	score, err = lbp.Execute(providers.ChatCompletionRequest{}, r)
	if err != nil {
		t.Fatalf("lb policy failed: %v", err)
	}
	if score.Model != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini from lb, got %s", score.Model)
	}

	// Test Fallback (first fails, second succeeds)
	var fallbackTargets []Policy
	
	// Create a policy that will fail
	failConfig := []byte(`{"type": "load_balance", "weights": {}}`)
	failPolicy, _ := ParsePolicyConfig(failConfig)
	
	fallbackTargets = append(fallbackTargets, failPolicy) // This one will fail execution because targets is nil inside
	fallbackTargets = append(fallbackTargets, dp)         // This one succeeds

	fp := &FallbackPolicy{Targets: fallbackTargets}
	score, err = fp.Execute(providers.ChatCompletionRequest{}, r)
	if err != nil {
		t.Fatalf("fallback policy failed: %v", err)
	}
	if score.Model != "gpt-4o-mini" {
		t.Errorf("expected fallback to reach gpt-4o-mini, got %s", score.Model)
	}
}
