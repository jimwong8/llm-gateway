package policy

import (
	"context"
	"testing"
)

func TestRoleBindingShape(t *testing.T) {
	b := TenantRoleBinding{TenantID: "t1", Subject: "alice", Role: "admin"}
	if b.Role != "admin" {
		t.Fatalf("unexpected binding: %+v", b)
	}
}

func TestProviderPolicyShape(t *testing.T) {
	p := TenantProviderPolicy{TenantID: "t1", Provider: "openai", Mode: "allow", Enabled: true}
	if p.Mode != "allow" {
		t.Fatalf("unexpected provider policy: %+v", p)
	}
}

func TestSensitiveRuleShape(t *testing.T) {
	r := SensitiveRule{TenantID: "t1", Pattern: "secret", Action: "block", Enabled: true}
	if r.Action != "block" {
		t.Fatalf("unexpected sensitive rule: %+v", r)
	}
}

func TestStoreMethodSignaturesCompile(t *testing.T) {
	var s *Store
	ctx := context.Background()
	_, _ = s, ctx
	_ = func() error { return nil }
}

func TestSetAllowedModelsOverlayAndReadBack(t *testing.T) {
	s := &Store{}
	s.SetAllowedModelsOverlay("tenant-a", []string{"gpt-4o-mini", "gpt-4o-mini", "claude-sonnet", "  "})

	models, err := s.AllowedModels(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("AllowedModels returned error: %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-4o-mini" || models[1] != "claude-sonnet" {
		t.Fatalf("unexpected overlay models: %+v", models)
	}
}

func TestSetProviderPoliciesOverlayAndReadBack(t *testing.T) {
	s := &Store{}
	s.SetProviderPoliciesOverlay("tenant-a", []TenantProviderPolicy{
		{Provider: "openai", Mode: "deny", Enabled: true},
		{Provider: "openai", Mode: "allow", Enabled: false},
		{Provider: "anthropic", Mode: "allow", Enabled: true},
		{Provider: "", Mode: "deny", Enabled: true},
		{Provider: "siliconflow", Mode: "invalid", Enabled: true},
	})

	policies, err := s.ProviderPolicies(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("ProviderPolicies returned error: %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("expected two normalized provider policies, got %+v", policies)
	}
	if policies[0].TenantID != "tenant-a" || policies[0].Provider != "openai" || policies[0].Mode != "allow" || policies[0].Enabled {
		t.Fatalf("unexpected first normalized provider policy: %+v", policies[0])
	}
	if policies[1].TenantID != "tenant-a" || policies[1].Provider != "anthropic" || policies[1].Mode != "allow" || !policies[1].Enabled {
		t.Fatalf("unexpected second normalized provider policy: %+v", policies[1])
	}
}

func TestProviderPoliciesOverlayDoesNotAffectAllowedModels(t *testing.T) {
	s := &Store{}
	s.SetAllowedModelsOverlay("tenant-a", []string{"gpt-4o-mini"})
	s.SetProviderPoliciesOverlay("tenant-a", []TenantProviderPolicy{{TenantID: "tenant-a", Provider: "openai", Mode: "deny", Enabled: true}})

	models, err := s.AllowedModels(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("AllowedModels returned error: %v", err)
	}
	if len(models) != 1 || models[0] != "gpt-4o-mini" {
		t.Fatalf("expected allowed_models unchanged by provider overlay, got %+v", models)
	}
}

func TestSetSensitiveRulesOverlayAndReadBack(t *testing.T) {
	s := &Store{}
	s.SetSensitiveRulesOverlay("tenant-a", []SensitiveRule{
		{Pattern: "secret", Action: "block", Enabled: true},
		{Pattern: "secret", Action: "block", Enabled: false},
		{Pattern: "pii", Action: "block", Enabled: true},
		{Pattern: " ", Action: "block", Enabled: true},
		{Pattern: "token", Action: "mask", Enabled: true},
	})

	rules, err := s.SensitiveRules(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("SensitiveRules returned error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected two normalized sensitive rules, got %+v", rules)
	}
	if rules[0].TenantID != "tenant-a" || rules[0].Pattern != "secret" || rules[0].Action != "block" || rules[0].Enabled {
		t.Fatalf("unexpected first normalized sensitive rule: %+v", rules[0])
	}
	if rules[1].TenantID != "tenant-a" || rules[1].Pattern != "pii" || rules[1].Action != "block" || !rules[1].Enabled {
		t.Fatalf("unexpected second normalized sensitive rule: %+v", rules[1])
	}
}

func TestSetRoleBindingsOverlayAndRoleForReadBack(t *testing.T) {
	s := &Store{}
	s.SetRoleBindingsOverlay("tenant-a", []TenantRoleBinding{
		{Subject: "alice", Role: "admin"},
		{Subject: "alice", Role: "operator"},
		{Subject: "bob", Role: "readonly"},
		{Subject: " ", Role: "admin"},
		{Subject: "charlie", Role: "invalid"},
	})

	role, err := s.RoleFor(context.Background(), "tenant-a", "alice")
	if err != nil {
		t.Fatalf("RoleFor returned error: %v", err)
	}
	if role != "operator" {
		t.Fatalf("expected normalized/last-write role for alice=operator, got %q", role)
	}

	role, err = s.RoleFor(context.Background(), "tenant-a", "bob")
	if err != nil {
		t.Fatalf("RoleFor returned error: %v", err)
	}
	if role != "readonly" {
		t.Fatalf("expected bob readonly role, got %q", role)
	}
}

func TestRoleBindingsOverlayDoesNotAffectAllowedModelsProviderPoliciesOrSensitiveRules(t *testing.T) {
	s := &Store{}
	s.SetAllowedModelsOverlay("tenant-a", []string{"gpt-4o-mini"})
	s.SetProviderPoliciesOverlay("tenant-a", []TenantProviderPolicy{{TenantID: "tenant-a", Provider: "openai", Mode: "deny", Enabled: true}})
	s.SetSensitiveRulesOverlay("tenant-a", []SensitiveRule{{Pattern: "secret", Action: "block", Enabled: true}})
	s.SetRoleBindingsOverlay("tenant-a", []TenantRoleBinding{{Subject: "alice", Role: "admin"}})

	models, err := s.AllowedModels(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("AllowedModels returned error: %v", err)
	}
	if len(models) != 1 || models[0] != "gpt-4o-mini" {
		t.Fatalf("expected allowed_models unchanged by role bindings overlay, got %+v", models)
	}

	policies, err := s.ProviderPolicies(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("ProviderPolicies returned error: %v", err)
	}
	if len(policies) != 1 || policies[0].Provider != "openai" || policies[0].Mode != "deny" {
		t.Fatalf("expected provider policies unchanged by role bindings overlay, got %+v", policies)
	}

	rules, err := s.SensitiveRules(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("SensitiveRules returned error: %v", err)
	}
	if len(rules) != 1 || rules[0].Pattern != "secret" || rules[0].Action != "block" {
		t.Fatalf("expected sensitive rules unchanged by role bindings overlay, got %+v", rules)
	}
}

