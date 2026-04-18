package router

import (
	"os"
	"path/filepath"
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestBootstrapFromFile_NoOpWhenPathEmpty(t *testing.T) {
	r := New("default-prov", "default-model")
	if err := r.BootstrapFromFile(""); err != nil {
		t.Fatalf("expected no-op nil err, got %v", err)
	}
}

func TestBootstrapFromFile_NoOpWhenFileMissing(t *testing.T) {
	r := New("default-prov", "default-model")
	missing := filepath.Join(t.TempDir(), "not-exists.json")
	if err := r.BootstrapFromFile(missing); err != nil {
		t.Fatalf("expected no-op nil err for missing file, got %v", err)
	}
}

func TestBootstrapFromJSON_ApplyChannelsAbilitiesAndPolicy(t *testing.T) {
	r := New("default-prov", "default-model")

	raw := []byte(`{
		"channels": [
			{"id":"code-primary","provider":"mock-code","model":"deepseek-coder","task":"code","enabled":true,"priority":1,"weight":1}
		],
		"abilities": [
			{"id":"tenant-code","task":"code","channel_ids":["code-primary"],"enabled":true,"priority":1}
		],
		"policy": {"type":"direct","model":"claude-sonnet"}
	}`)

	if err := r.BootstrapFromJSON(raw); err != nil {
		t.Fatalf("unexpected bootstrap err: %v", err)
	}

	policyDecision := r.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}})
	if policyDecision.RouteMode != "policy" || policyDecision.Model != "claude-sonnet" {
		t.Fatalf("expected policy route to claude-sonnet, got mode=%s model=%s", policyDecision.RouteMode, policyDecision.Model)
	}

	r.SetGlobalPolicy(nil)
	abilityDecision := r.Decide(providers.ChatCompletionRequest{
		RouteAbilities: []string{"tenant-code"},
		TaskHint:       "code",
		Messages:       []providers.ChatMessage{{Role: "user", Content: "write code"}},
	})
	if abilityDecision.RouteMode != "ability" || abilityDecision.Channel != "code-primary" {
		t.Fatalf("expected ability route via code-primary, got mode=%s channel=%s", abilityDecision.RouteMode, abilityDecision.Channel)
	}
}

func TestBootstrapFromFile_ApplyFromExistingFile(t *testing.T) {
	r := New("default-prov", "default-model")
	path := filepath.Join(t.TempDir(), "router-bootstrap.json")
	content := []byte(`{"policy":{"type":"direct","model":"gpt-4o-mini"}}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}

	if err := r.BootstrapFromFile(path); err != nil {
		t.Fatalf("unexpected bootstrap file err: %v", err)
	}

	decision := r.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hi"}}})
	if decision.RouteMode != "policy" || decision.Model != "gpt-4o-mini" {
		t.Fatalf("expected policy route to gpt-4o-mini, got mode=%s model=%s", decision.RouteMode, decision.Model)
	}
}
