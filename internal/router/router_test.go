package router

import (
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestRouter_Decide_ManualOverride(t *testing.T) {
	r := New("default-prov", "default-model")

	req := providers.ChatCompletionRequest{
		RouteMode:      "manual",
		PreferredModel: "gpt-4o-mini",
	}

	decision := r.Decide(req)
	if decision.RouteMode != "manual" {
		t.Errorf("expected mode manual, got %s", decision.RouteMode)
	}
	if decision.Model != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %s", decision.Model)
	}
}

func TestRouter_Decide_GlobalPolicy(t *testing.T) {
	r := New("default-prov", "default-model")

	// Create a policy that routes everything to claude-sonnet
	policyRaw := []byte(`{"type": "direct", "model": "claude-sonnet"}`)
	p, err := ParsePolicyConfig(policyRaw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	r.SetGlobalPolicy(p)

	req := providers.ChatCompletionRequest{
		RouteMode: "auto",
	}

	decision := r.Decide(req)
	if decision.RouteMode != "policy" {
		t.Errorf("expected mode policy, got %s", decision.RouteMode)
	}
	if decision.Model != "claude-sonnet" {
		t.Errorf("expected model claude-sonnet, got %s", decision.Model)
	}
}

func TestClassifyTask(t *testing.T) {
	tests := []struct {
		req  providers.ChatCompletionRequest
		want string
	}{
		{
			req:  providers.ChatCompletionRequest{TaskHint: "code"},
			want: "code",
		},
		{
			req: providers.ChatCompletionRequest{
				Messages: []providers.ChatMessage{{Content: "help me write a python script"}},
			},
			want: "code",
		},
		{
			req: providers.ChatCompletionRequest{
				Messages: []providers.ChatMessage{{Content: "analyze this report"}},
			},
			want: "analysis",
		},
		{
			req: providers.ChatCompletionRequest{
				Messages: []providers.ChatMessage{{Content: "hello"}},
			},
			want: "general",
		},
	}

	for i, test := range tests {
		got := classifyTask(test.req)
		if got != test.want {
			t.Errorf("test %d: expected %s, got %s", i, test.want, got)
		}
	}
}

func TestRouter_Decide_ExplicitRouteChannel(t *testing.T) {
	r := New("default-prov", "default-model")
	r.SetChannels([]Channel{{ID: "premium-openai", Provider: "openai", Model: "gpt-4o-mini", Task: "general", Enabled: true, Priority: 1, Weight: 1}})

	decision := r.Decide(providers.ChatCompletionRequest{
		RouteMode:    "auto",
		RouteChannel: "premium-openai",
		Messages:     []providers.ChatMessage{{Role: "user", Content: "hello"}},
	})

	if decision.RouteMode != "channel" {
		t.Fatalf("expected channel mode, got %s", decision.RouteMode)
	}
	if decision.Channel != "premium-openai" {
		t.Fatalf("expected channel premium-openai, got %s", decision.Channel)
	}
	if decision.Model != "gpt-4o-mini" || decision.Provider != "openai" {
		t.Fatalf("unexpected target: %s/%s", decision.Provider, decision.Model)
	}
}

func TestRouter_Decide_RouteAbilities(t *testing.T) {
	r := New("default-prov", "default-model")
	r.SetChannels([]Channel{
		{ID: "code-primary", Provider: "mock-code", Model: "deepseek-coder", Task: "code", Enabled: true, Priority: 1, Weight: 1},
		{ID: "code-fallback", Provider: "mock-fail", Model: "fail-code", Task: "code", Enabled: true, Priority: 2, Weight: 1},
	})
	r.SetAbilities([]Ability{{ID: "tenant-code", Task: "code", ChannelIDs: []string{"code-primary", "code-fallback"}, Enabled: true, Priority: 1}})

	decision := r.Decide(providers.ChatCompletionRequest{
		RouteAbilities: []string{"tenant-code"},
		TaskHint:       "code",
		Messages:       []providers.ChatMessage{{Role: "user", Content: "write some code"}},
	})

	if decision.RouteMode != "ability" {
		t.Fatalf("expected ability mode, got %s", decision.RouteMode)
	}
	if decision.Ability != "tenant-code" {
		t.Fatalf("expected ability tenant-code, got %s", decision.Ability)
	}
	if decision.Channel != "code-primary" {
		t.Fatalf("expected primary channel code-primary, got %s", decision.Channel)
	}
	if decision.FallbackModel != "fail-code" {
		t.Fatalf("expected fallback model fail-code, got %s", decision.FallbackModel)
	}
}

func TestRouter_Decide_LegacyRouting_UnchangedWithoutControlPlaneConfig(t *testing.T) {
	r := New("default-prov", "default-model")

	decision := r.Decide(providers.ChatCompletionRequest{
		RouteMode: "auto",
		TaskHint:  "code",
		Messages:  []providers.ChatMessage{{Role: "user", Content: "write a golang function"}},
	})

	if decision.RouteMode != "auto" {
		t.Fatalf("expected auto mode, got %s", decision.RouteMode)
	}
	if decision.Channel != "" {
		t.Fatalf("expected no channel for legacy routing, got %s", decision.Channel)
	}
	if decision.Ability != "" {
		t.Fatalf("expected no ability for legacy routing, got %s", decision.Ability)
	}
	if decision.Model != "deepseek-coder" || decision.Provider != "mock-code" {
		t.Fatalf("expected legacy task-based routing to choose deepseek-coder/mock-code, got %s/%s", decision.Provider, decision.Model)
	}
}

func TestRouter_Decide_ExplicitChannel_UnknownFallsBackToLegacy(t *testing.T) {
	r := New("default-prov", "default-model")

	decision := r.Decide(providers.ChatCompletionRequest{
		RouteMode:    "auto",
		RouteChannel: "missing-channel",
		TaskHint:     "code",
		Messages:     []providers.ChatMessage{{Role: "user", Content: "write a golang function"}},
	})

	if decision.RouteMode != "auto" {
		t.Fatalf("expected auto mode when explicit channel is missing, got %s", decision.RouteMode)
	}
	if decision.Channel != "" {
		t.Fatalf("expected empty channel when explicit channel is missing, got %s", decision.Channel)
	}
	if decision.Model != "deepseek-coder" || decision.Provider != "mock-code" {
		t.Fatalf("expected fallback to legacy task-based routing target, got %s/%s", decision.Provider, decision.Model)
	}
}

func TestRouter_Decide_ExplicitAbility_UnknownFallsBackToLegacy(t *testing.T) {
	r := New("default-prov", "default-model")

	decision := r.Decide(providers.ChatCompletionRequest{
		RouteMode:      "auto",
		RouteAbilities: []string{"tenant-code"},
		TaskHint:       "code",
		Messages:       []providers.ChatMessage{{Role: "user", Content: "write a golang function"}},
	})

	if decision.RouteMode != "auto" {
		t.Fatalf("expected auto mode when explicit ability is unavailable, got %s", decision.RouteMode)
	}
	if decision.Ability != "" {
		t.Fatalf("expected empty ability when explicit ability is unavailable, got %s", decision.Ability)
	}
	if decision.Channel != "" {
		t.Fatalf("expected empty channel when explicit ability is unavailable, got %s", decision.Channel)
	}
	if decision.Model != "deepseek-coder" || decision.Provider != "mock-code" {
		t.Fatalf("expected fallback to legacy task-based routing target, got %s/%s", decision.Provider, decision.Model)
	}
}
