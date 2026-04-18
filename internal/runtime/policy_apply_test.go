package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"llm-gateway/gateway/internal/controlplane"
	policy "llm-gateway/gateway/internal/policy"
)

type policyOverlayCaptureStore struct {
	overlays          map[string][]string
	roleOverlays      map[string][]policy.TenantRoleBinding
	providerOverlays  map[string][]policy.TenantProviderPolicy
	sensitiveOverlays map[string][]policy.SensitiveRule
}

func (s *policyOverlayCaptureStore) SetAllowedModelsOverlay(tenantID string, models []string) {
	if s.overlays == nil {
		s.overlays = map[string][]string{}
	}
	copied := make([]string, len(models))
	copy(copied, models)
	s.overlays[tenantID] = copied
}

func (s *policyOverlayCaptureStore) SetRoleBindingsOverlay(tenantID string, bindings []policy.TenantRoleBinding) {
	if s.roleOverlays == nil {
		s.roleOverlays = map[string][]policy.TenantRoleBinding{}
	}
	copied := make([]policy.TenantRoleBinding, len(bindings))
	copy(copied, bindings)
	s.roleOverlays[tenantID] = copied
}

func (s *policyOverlayCaptureStore) SetProviderPoliciesOverlay(tenantID string, policies []policy.TenantProviderPolicy) {
	if s.providerOverlays == nil {
		s.providerOverlays = map[string][]policy.TenantProviderPolicy{}
	}
	copied := make([]policy.TenantProviderPolicy, len(policies))
	copy(copied, policies)
	s.providerOverlays[tenantID] = copied
}

func (s *policyOverlayCaptureStore) SetSensitiveRulesOverlay(tenantID string, rules []policy.SensitiveRule) {
	if s.sensitiveOverlays == nil {
		s.sensitiveOverlays = map[string][]policy.SensitiveRule{}
	}
	copied := make([]policy.SensitiveRule, len(rules))
	copy(copied, rules)
	s.sensitiveOverlays[tenantID] = copied
}

type stubPolicyReleasedVersionResolver struct {
	version controlplane.ConfigVersion
	err     error
	calls   int
}

func (s *stubPolicyReleasedVersionResolver) GetVersion(_ context.Context, module, tenantID, environment, scope, projectID, versionID string) (controlplane.ConfigVersion, error) {
	s.calls++
	if s.err != nil {
		return controlplane.ConfigVersion{}, s.err
	}
	return s.version, nil
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_UsesResolverPayloadFirst(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_p1",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models": map[string]any{"allowed_models": []any{"stale-model"}},
			},
		},
	}})

	resolver := &stubPolicyReleasedVersionResolver{version: controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     "cfg_p1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"models/allowed_models": `["gpt-4o-mini","claude-sonnet"]`,
		},
	}}

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_p1"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
	got := store.overlays["tenant-a"]
	if len(got) != 2 || got[0] != "gpt-4o-mini" || got[1] != "claude-sonnet" {
		t.Fatalf("expected resolver models overlay applied, got %+v", got)
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_FallbacksToPublisherWhenResolverUnavailable(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_p2",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models": map[string]any{"allowed_models": []any{"gpt-4o-mini"}},
			},
		},
	}})
	resolver := &stubPolicyReleasedVersionResolver{err: errors.New("resolver unavailable")}

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_p2"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
	got := store.overlays["tenant-a"]
	if len(got) != 1 || got[0] != "gpt-4o-mini" {
		t.Fatalf("expected publisher fallback models overlay applied, got %+v", got)
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_UsesResolverPayloadFirstForSensitiveRules(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive_resolver",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"sensitive": map[string]any{"rules": []any{map[string]any{"pattern": "stale-secret", "action": "block", "enabled": true}}},
			},
		},
	}})
	resolver := &stubPolicyReleasedVersionResolver{version: controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     "cfg_sensitive_resolver",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"sensitive/rules": `[{"pattern":"resolver-secret","action":"block","enabled":true}]`,
		},
	}}

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive_resolver"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
	got := store.sensitiveOverlays["tenant-a"]
	if len(got) != 1 || got[0].Pattern != "resolver-secret" || got[0].Action != "block" || !got[0].Enabled {
		t.Fatalf("expected resolver sensitive rules overlay applied, got %+v", got)
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_FallbacksToPublisherForSensitiveRulesWhenResolverUnavailable(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive_fallback",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"sensitive": map[string]any{"rules": []any{map[string]any{"pattern": "fallback-secret", "action": "block", "enabled": true}}},
			},
		},
	}})
	resolver := &stubPolicyReleasedVersionResolver{err: errors.New("resolver unavailable")}

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive_fallback"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
	got := store.sensitiveOverlays["tenant-a"]
	if len(got) != 1 || got[0].Pattern != "fallback-secret" || got[0].Action != "block" || !got[0].Enabled {
		t.Fatalf("expected publisher fallback sensitive rules overlay applied, got %+v", got)
	}
}


func TestBuildPolicyPayloadDrivenApplyWithResolver_NoPolicyPayloadIsNoOp(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:       "tenant-a",
		PayloadRef:     "released://policy/tenant-a/prod/tenant/project-a/cfg_p3",
		ModulePayloads: map[string]any{"quota": map[string]any{"rpm": 10}},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_p3"}); err != nil {
		t.Fatalf("expected nil error for no policy payload, got %v", err)
	}
	if len(store.overlays) != 0 {
		t.Fatalf("expected no allowed_models overlay mutation for no policy payload, got %+v", store.overlays)
	}
	if len(store.roleOverlays) != 0 {
		t.Fatalf("expected no role overlay mutation for no policy payload, got %+v", store.roleOverlays)
	}
	if len(store.providerOverlays) != 0 {
		t.Fatalf("expected no provider overlay mutation for no policy payload, got %+v", store.providerOverlays)
	}
	if len(store.sensitiveOverlays) != 0 {
		t.Fatalf("expected no sensitive overlay mutation for no policy payload, got %+v", store.sensitiveOverlays)
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_AppliesSensitiveRulesFromSensitiveObject(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"sensitive": map[string]any{"rules": []any{
					map[string]any{"pattern": "secret", "action": "block", "enabled": true},
					map[string]any{"pattern": "pii", "action": "block", "enabled": false},
				}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	got := store.sensitiveOverlays["tenant-a"]
	if len(got) != 2 {
		t.Fatalf("expected 2 sensitive rules, got %+v", got)
	}
	if got[0].Pattern != "secret" || got[0].Action != "block" || !got[0].Enabled {
		t.Fatalf("unexpected first sensitive rule: %+v", got[0])
	}
	if got[1].Pattern != "pii" || got[1].Action != "block" || got[1].Enabled {
		t.Fatalf("unexpected second sensitive rule: %+v", got[1])
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_ParsesSensitiveRulesFromLegacyTopLevelKey(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive_legacy",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"sensitive_rules": []any{map[string]any{"pattern": "token", "action": "block", "enabled": true}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive_legacy"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	got := store.sensitiveOverlays["tenant-a"]
	if len(got) != 1 || got[0].Pattern != "token" || got[0].Action != "block" || !got[0].Enabled {
		t.Fatalf("expected legacy sensitive rules applied, got %+v", got)
	}
}


func TestBuildPolicyPayloadDrivenApplyWithResolver_InvalidAllowedModelsReturnsErrorAndNoMutation(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_p4",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models": map[string]any{"allowed_models": []any{"gpt-4o-mini", 1}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_p4"})
	if err == nil {
		t.Fatalf("expected invalid allowed_models to return error")
	}
	if len(store.overlays) != 0 {
		t.Fatalf("expected no mutation on invalid payload, got %+v", store.overlays)
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_AppliesRoleBindingsFromRolesObject(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_role",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"roles": map[string]any{"bindings": []any{
					map[string]any{"subject": "alice", "role": "admin"},
					map[string]any{"subject": "bob", "role": "readonly"},
				}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_role"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	got := store.roleOverlays["tenant-a"]
	if len(got) != 2 {
		t.Fatalf("expected 2 role bindings, got %+v", got)
	}
	if got[0].Subject != "alice" || got[0].Role != "admin" {
		t.Fatalf("unexpected first role binding: %+v", got[0])
	}
	if got[1].Subject != "bob" || got[1].Role != "readonly" {
		t.Fatalf("unexpected second role binding: %+v", got[1])
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_ParsesRoleBindingsFromLegacyTopLevelKey(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_role_legacy",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"role_bindings": []any{map[string]any{"subject": "alice", "role": "operator"}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_role_legacy"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	got := store.roleOverlays["tenant-a"]
	if len(got) != 1 || got[0].Subject != "alice" || got[0].Role != "operator" {
		t.Fatalf("expected legacy role bindings applied, got %+v", got)
	}
}


func TestBuildPolicyPayloadDrivenApplyWithResolver_AppliesProviderPoliciesFromProvidersObject(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_provider",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"providers": map[string]any{"policies": []any{
					map[string]any{"provider": "openai", "mode": "deny", "enabled": true},
					map[string]any{"provider": "anthropic", "mode": "allow", "enabled": true},
				}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_provider"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	got := store.providerOverlays["tenant-a"]
	if len(got) != 2 {
		t.Fatalf("expected 2 provider policies, got %+v", got)
	}
	if got[0].Provider != "openai" || got[0].Mode != "deny" || !got[0].Enabled {
		t.Fatalf("unexpected first provider policy: %+v", got[0])
	}
	if got[1].Provider != "anthropic" || got[1].Mode != "allow" || !got[1].Enabled {
		t.Fatalf("unexpected second provider policy: %+v", got[1])
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_ParsesProviderPoliciesFromLegacyTopLevelKey(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_provider_legacy",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"provider_policies": []any{map[string]any{"provider": "openai", "mode": "deny", "enabled": true}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_provider_legacy"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	got := store.providerOverlays["tenant-a"]
	if len(got) != 1 || got[0].Provider != "openai" || got[0].Mode != "deny" || !got[0].Enabled {
		t.Fatalf("expected legacy provider policies applied, got %+v", got)
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_InvalidRoleBindingsReturnsErrorAndNoPartialMutation(t *testing.T) {
	store := &policyOverlayCaptureStore{
		overlays: map[string][]string{
			"tenant-a": {"baseline-model"},
		},
		roleOverlays: map[string][]policy.TenantRoleBinding{
			"tenant-a": {{TenantID: "tenant-a", Subject: "alice", Role: "admin"}},
		},
		providerOverlays: map[string][]policy.TenantProviderPolicy{
			"tenant-a": {{TenantID: "tenant-a", Provider: "openai", Mode: "allow", Enabled: true}},
		},
		sensitiveOverlays: map[string][]policy.SensitiveRule{
			"tenant-a": {{TenantID: "tenant-a", Pattern: "baseline-secret", Action: "block", Enabled: true}},
		},
	}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_role_invalid",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models":        map[string]any{"allowed_models": []any{"new-model"}},
				"role_bindings": []any{map[string]any{"subject": "alice", "role": "invalid"}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_role_invalid"})
	if err == nil {
		t.Fatalf("expected invalid role bindings to return error")
	}
	if got := store.overlays["tenant-a"]; len(got) != 1 || got[0] != "baseline-model" {
		t.Fatalf("expected allowed_models overlay unchanged on invalid payload, got %+v", got)
	}
	roleGot := store.roleOverlays["tenant-a"]
	if len(roleGot) != 1 || roleGot[0].Subject != "alice" || roleGot[0].Role != "admin" {
		t.Fatalf("expected role overlay unchanged on invalid payload, got %+v", roleGot)
	}
	providerGot := store.providerOverlays["tenant-a"]
	if len(providerGot) != 1 || providerGot[0].Provider != "openai" || providerGot[0].Mode != "allow" {
		t.Fatalf("expected provider overlay unchanged on invalid payload, got %+v", providerGot)
	}
	sensitiveGot := store.sensitiveOverlays["tenant-a"]
	if len(sensitiveGot) != 1 || sensitiveGot[0].Pattern != "baseline-secret" || sensitiveGot[0].Action != "block" {
		t.Fatalf("expected sensitive overlay unchanged on invalid payload, got %+v", sensitiveGot)
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_InvalidProviderPoliciesReturnsErrorAndNoPartialMutation(t *testing.T) {
	store := &policyOverlayCaptureStore{
		overlays: map[string][]string{
			"tenant-a": {"baseline-model"},
		},
		roleOverlays: map[string][]policy.TenantRoleBinding{
			"tenant-a": {{TenantID: "tenant-a", Subject: "alice", Role: "admin"}},
		},
		providerOverlays: map[string][]policy.TenantProviderPolicy{
			"tenant-a": {{TenantID: "tenant-a", Provider: "openai", Mode: "allow", Enabled: true}},
		},
		sensitiveOverlays: map[string][]policy.SensitiveRule{
			"tenant-a": {{TenantID: "tenant-a", Pattern: "baseline-secret", Action: "block", Enabled: true}},
		},
	}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_provider_invalid",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models":            map[string]any{"allowed_models": []any{"new-model"}},
				"provider_policies": []any{map[string]any{"provider": "openai", "mode": "invalid", "enabled": true}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_provider_invalid"})
	if err == nil {
		t.Fatalf("expected invalid provider policies to return error")
	}
	if got := store.overlays["tenant-a"]; len(got) != 1 || got[0] != "baseline-model" {
		t.Fatalf("expected allowed_models overlay unchanged on invalid payload, got %+v", got)
	}
	roleGot := store.roleOverlays["tenant-a"]
	if len(roleGot) != 1 || roleGot[0].Subject != "alice" || roleGot[0].Role != "admin" {
		t.Fatalf("expected role overlay unchanged on invalid payload, got %+v", roleGot)
	}
	providerGot := store.providerOverlays["tenant-a"]
	if len(providerGot) != 1 || providerGot[0].Provider != "openai" || providerGot[0].Mode != "allow" {
		t.Fatalf("expected provider overlay unchanged on invalid payload, got %+v", providerGot)
	}
	sensitiveGot := store.sensitiveOverlays["tenant-a"]
	if len(sensitiveGot) != 1 || sensitiveGot[0].Pattern != "baseline-secret" || sensitiveGot[0].Action != "block" {
		t.Fatalf("expected sensitive overlay unchanged on invalid payload, got %+v", sensitiveGot)
	}
}

func TestBuildPolicyPayloadDrivenApplyWithResolver_InvalidSensitiveRulesReturnsErrorAndNoPartialMutation(t *testing.T) {
	store := &policyOverlayCaptureStore{
		overlays: map[string][]string{
			"tenant-a": {"baseline-model"},
		},
		roleOverlays: map[string][]policy.TenantRoleBinding{
			"tenant-a": {{TenantID: "tenant-a", Subject: "alice", Role: "admin"}},
		},
		providerOverlays: map[string][]policy.TenantProviderPolicy{
			"tenant-a": {{TenantID: "tenant-a", Provider: "openai", Mode: "allow", Enabled: true}},
		},
		sensitiveOverlays: map[string][]policy.SensitiveRule{
			"tenant-a": {{TenantID: "tenant-a", Pattern: "baseline-secret", Action: "block", Enabled: true}},
		},
	}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive_invalid",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models": map[string]any{"allowed_models": []any{"new-model"}},
				"sensitive": map[string]any{"rules": []any{
					map[string]any{"pattern": "new-secret", "action": "block", "enabled": true},
					map[string]any{"pattern": "bad", "action": "mask", "enabled": true},
				}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_sensitive_invalid"})
	if err == nil {
		t.Fatalf("expected invalid sensitive rules to return error")
	}
	if got := store.overlays["tenant-a"]; len(got) != 1 || got[0] != "baseline-model" {
		t.Fatalf("expected allowed_models overlay unchanged on invalid payload, got %+v", got)
	}
	roleGot := store.roleOverlays["tenant-a"]
	if len(roleGot) != 1 || roleGot[0].Subject != "alice" || roleGot[0].Role != "admin" {
		t.Fatalf("expected role overlay unchanged on invalid payload, got %+v", roleGot)
	}
	providerGot := store.providerOverlays["tenant-a"]
	if len(providerGot) != 1 || providerGot[0].Provider != "openai" || providerGot[0].Mode != "allow" {
		t.Fatalf("expected provider overlay unchanged on invalid payload, got %+v", providerGot)
	}
	sensitiveGot := store.sensitiveOverlays["tenant-a"]
	if len(sensitiveGot) != 1 || sensitiveGot[0].Pattern != "baseline-secret" || sensitiveGot[0].Action != "block" {
		t.Fatalf("expected sensitive overlay unchanged on invalid payload, got %+v", sensitiveGot)
	}
}


func TestBuildPolicyPayloadDrivenApplyWithResolver_AppliesAllowedModelsProviderPoliciesAndSensitiveRulesTogether(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_all",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models":    map[string]any{"allowed_models": []any{"gpt-4o-mini"}},
				"roles":     map[string]any{"bindings": []any{map[string]any{"subject": "alice", "role": "admin"}}},
				"providers": map[string]any{"policies": []any{map[string]any{"provider": "openai", "mode": "deny", "enabled": true}}},
				"sensitive": map[string]any{"rules": []any{map[string]any{"pattern": "secret", "action": "block", "enabled": true}}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_all"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if got := store.overlays["tenant-a"]; len(got) != 1 || got[0] != "gpt-4o-mini" {
		t.Fatalf("expected allowed_models overlay applied, got %+v", got)
	}
	if got := store.roleOverlays["tenant-a"]; len(got) != 1 || got[0].Subject != "alice" || got[0].Role != "admin" {
		t.Fatalf("expected role overlay applied, got %+v", got)
	}
	if got := store.providerOverlays["tenant-a"]; len(got) != 1 || got[0].Provider != "openai" || got[0].Mode != "deny" {
		t.Fatalf("expected provider overlay applied, got %+v", got)
	}
	if got := store.sensitiveOverlays["tenant-a"]; len(got) != 1 || got[0].Pattern != "secret" || got[0].Action != "block" {
		t.Fatalf("expected sensitive overlay applied, got %+v", got)
	}
}


func TestBuildPolicyPayloadDrivenApply_BackwardCompatibilityAlias(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_alias",
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models": map[string]any{"allowed_models": []any{"claude-sonnet"}},
			},
		},
	}})

	apply := BuildPolicyPayloadDrivenApply(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-a/cfg_alias"}); err != nil {
		t.Fatalf("alias apply returned error: %v", err)
	}
	got := store.overlays["tenant-a"]
	if len(got) != 1 || got[0] != "claude-sonnet" {
		t.Fatalf("expected alias apply to set policy overlay, got %+v", got)
	}
}

func TestBuildPolicyModulePayload_FromModelsAllowedModels(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"models/allowed_models": `["gpt-4o-mini","claude-sonnet"]`,
		},
	}

	payload, ok := buildPolicyModulePayload(version)
	if !ok {
		t.Fatalf("expected policy module payload")
	}
	modelsRaw, ok := payload["models"].(map[string]any)
	if !ok {
		t.Fatalf("expected models object in payload, got %T", payload["models"])
	}
	allowedRaw, ok := modelsRaw["allowed_models"].([]string)
	if !ok {
		t.Fatalf("expected allowed_models []string in payload, got %T", modelsRaw["allowed_models"])
	}
	if len(allowedRaw) != 2 || allowedRaw[0] != "gpt-4o-mini" || allowedRaw[1] != "claude-sonnet" {
		t.Fatalf("unexpected allowed_models payload %+v", allowedRaw)
	}
}

func TestBuildPolicyModulePayload_IncludesRoleBindingsFromRolesBindingsConfig(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_roles_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"roles/bindings": `[{"subject":"alice","role":"admin"}]`,
		},
	}

	payload, ok := buildPolicyModulePayload(version)
	if !ok {
		t.Fatalf("expected policy module payload")
	}
	rolesRaw, ok := payload["roles"].(map[string]any)
	if !ok {
		t.Fatalf("expected roles object in payload, got %T", payload["roles"])
	}
	bindings, ok := rolesRaw["bindings"].([]policy.TenantRoleBinding)
	if !ok {
		t.Fatalf("expected role bindings []TenantRoleBinding, got %T", rolesRaw["bindings"])
	}
	if len(bindings) != 1 || bindings[0].Subject != "alice" || bindings[0].Role != "admin" {
		t.Fatalf("unexpected role bindings payload %+v", bindings)
	}
}
func TestBuildPolicyModulePayload_IncludesProviderPoliciesFromProvidersPoliciesConfig(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_provider_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"providers/policies": `[{"provider":"openai","mode":"deny","enabled":true}]`,
		},
	}

	payload, ok := buildPolicyModulePayload(version)
	if !ok {
		t.Fatalf("expected policy module payload")
	}
	providersRaw, ok := payload["providers"].(map[string]any)
	if !ok {
		t.Fatalf("expected providers object in payload, got %T", payload["providers"])
	}
	policies, ok := providersRaw["policies"].([]policy.TenantProviderPolicy)
	if !ok {
		t.Fatalf("expected provider policies []TenantProviderPolicy, got %T", providersRaw["policies"])
	}
	if len(policies) != 1 || policies[0].Provider != "openai" || policies[0].Mode != "deny" || !policies[0].Enabled {
		t.Fatalf("unexpected provider policies payload %+v", policies)
	}
}

func TestBuildPolicyModulePayload_IncludesAllowedModelsAndProviderPoliciesTogether(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_both_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"models/allowed_models": ` ["gpt-4o-mini"] `,
			"providers/policies":    `[{"provider":"openai","mode":"deny","enabled":true}]`,
		},
	}

	payload, ok := buildPolicyModulePayload(version)
	if !ok {
		t.Fatalf("expected policy payload")
	}
	if payload["models"] == nil || payload["providers"] == nil {
		t.Fatalf("expected both models and providers in payload, got %+v", payload)
	}
}

func TestBuildPolicyModulePayload_IncludesSensitiveRulesFromSensitiveRulesConfig(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_sensitive_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"sensitive/rules": `[{"pattern":"secret","action":"block","enabled":true}]`,
		},
	}

	payload, ok := buildPolicyModulePayload(version)
	if !ok {
		t.Fatalf("expected policy module payload")
	}
	sensitiveRaw, ok := payload["sensitive"].(map[string]any)
	if !ok {
		t.Fatalf("expected sensitive object in payload, got %T", payload["sensitive"])
	}
	rules, ok := sensitiveRaw["rules"].([]policy.SensitiveRule)
	if !ok {
		t.Fatalf("expected sensitive rules []SensitiveRule, got %T", sensitiveRaw["rules"])
	}
	if len(rules) != 1 || rules[0].Pattern != "secret" || rules[0].Action != "block" || !rules[0].Enabled {
		t.Fatalf("unexpected sensitive rules payload %+v", rules)
	}
}

func TestBuildPolicyModulePayload_MissingModelsProvidersAndSensitiveProducesNoPayload(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_2",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"role_bindings": `[{"subject":"alice","role":"admin"}]`,
		},
	}

	payload, ok := buildPolicyModulePayload(version)
	if !ok {
		t.Fatalf("expected no policy payload when models/providers/sensitive missing, got %+v", payload)
	}
	if payload["roles"] == nil {
		t.Fatalf("expected roles payload from role_bindings config, got %+v", payload)
	}
}

func TestPolicyRuntimeApplyProviderPolicies_SuccessMarksManagerOKAndMutatesOverlay(t *testing.T) {
	store := &policyOverlayCaptureStore{}
	publisher := NewPublisher()
	bus := NewInProcessBus()
	manager := NewManager()

	payloadRef := "released://policy/tenant-a/prod/tenant/project-a/cfg_runtime_provider_ok"
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-a",
		PayloadRef: payloadRef,
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"providers": map[string]any{"policies": []any{map[string]any{"provider": "openai", "mode": "deny", "enabled": true}}},
			},
		},
	}})

	SubscribeManagerApplyBridge(bus, manager, BuildPolicyReloadApply(BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)))
	if err := bus.PublishConfigChange(ConfigChangeEvent{
		Module:     "policy",
		Scope:      "tenant",
		TenantID:   "tenant-a",
		Version:    "cfg_runtime_provider_ok",
		ChangedAt:  time.Now().UTC(),
		PayloadRef: payloadRef,
	}); err != nil {
		t.Fatalf("PublishConfigChange returned error: %v", err)
	}

	status := manager.GetStatus("policy")
	if status.LastSeenEventVersion != "cfg_runtime_provider_ok" || status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager status ok, got %+v", status)
	}
	if got := store.providerOverlays["tenant-a"]; len(got) != 1 || got[0].Provider != "openai" || got[0].Mode != "deny" {
		t.Fatalf("expected provider overlay applied, got %+v", got)
	}
}

func TestPolicyRuntimeApply_AllPolicyOverlaysTogether_SuccessAndReadback(t *testing.T) {
	store := &policy.Store{}
	publisher := NewPublisher()
	bus := NewInProcessBus()
	manager := NewManager()

	payloadRef := "released://policy/tenant-live/prod/tenant/project-a/cfg_runtime_policy_all_ok"
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-live",
		PayloadRef: payloadRef,
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models":    map[string]any{"allowed_models": []any{"gpt-4o-mini"}},
				"roles":     map[string]any{"bindings": []any{map[string]any{"subject": "alice", "role": "admin"}}},
				"providers": map[string]any{"policies": []any{map[string]any{"provider": "openai", "mode": "deny", "enabled": true}}},
				"sensitive": map[string]any{"rules": []any{map[string]any{"pattern": "secret", "action": "block", "enabled": true}}},
			},
		},
	}})

	SubscribeManagerApplyBridge(bus, manager, BuildPolicyReloadApply(BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)))
	if err := bus.PublishConfigChange(ConfigChangeEvent{
		Module:     "policy",
		Scope:      "tenant",
		TenantID:   "tenant-live",
		Version:    "cfg_runtime_policy_all_ok",
		ChangedAt:  time.Now().UTC(),
		PayloadRef: payloadRef,
	}); err != nil {
		t.Fatalf("PublishConfigChange returned error: %v", err)
	}

	status := manager.GetStatus("policy")
	if status.LastSeenEventVersion != "cfg_runtime_policy_all_ok" || status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager status ok, got %+v", status)
	}

	models, err := store.AllowedModels(context.Background(), "tenant-live")
	if err != nil {
		t.Fatalf("AllowedModels returned error: %v", err)
	}
	if len(models) != 1 || models[0] != "gpt-4o-mini" {
		t.Fatalf("unexpected allowed models overlay: %+v", models)
	}

	role, err := store.RoleFor(context.Background(), "tenant-live", "alice")
	if err != nil {
		t.Fatalf("RoleFor returned error: %v", err)
	}
	if role != "admin" {
		t.Fatalf("unexpected role binding overlay readback: %q", role)
	}

	providerPolicies, err := store.ProviderPolicies(context.Background(), "tenant-live")
	if err != nil {
		t.Fatalf("ProviderPolicies returned error: %v", err)
	}
	if len(providerPolicies) != 1 || providerPolicies[0].Provider != "openai" || providerPolicies[0].Mode != "deny" || !providerPolicies[0].Enabled {
		t.Fatalf("unexpected provider policies overlay: %+v", providerPolicies)
	}

	rules, err := store.SensitiveRules(context.Background(), "tenant-live")
	if err != nil {
		t.Fatalf("SensitiveRules returned error: %v", err)
	}
	if len(rules) != 1 || rules[0].Pattern != "secret" || rules[0].Action != "block" || !rules[0].Enabled {
		t.Fatalf("unexpected sensitive rules overlay: %+v", rules)
	}
}

func TestPolicyRuntimeApply_SensitiveRulesInvalidMarksManagerErrorAndNoPartialMutation(t *testing.T) {
	store := &policy.Store{}
	store.SetAllowedModelsOverlay("tenant-live", []string{"baseline-model"})
	store.SetProviderPoliciesOverlay("tenant-live", []policy.TenantProviderPolicy{{Provider: "openai", Mode: "allow", Enabled: true}})
	store.SetSensitiveRulesOverlay("tenant-live", []policy.SensitiveRule{{Pattern: "baseline-secret", Action: "block", Enabled: true}})

	publisher := NewPublisher()
	bus := NewInProcessBus()
	manager := NewManager()

	payloadRef := "released://policy/tenant-live/prod/tenant/project-a/cfg_runtime_policy_sensitive_invalid"
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		TenantID:   "tenant-live",
		PayloadRef: payloadRef,
		ModulePayloads: map[string]any{
			"policy": map[string]any{
				"models": map[string]any{"allowed_models": []any{"new-model"}},
				"sensitive": map[string]any{"rules": []any{
					map[string]any{"pattern": "new-secret", "action": "block", "enabled": true},
					map[string]any{"pattern": "bad", "action": "mask", "enabled": true},
				}},
			},
		},
	}})

	SubscribeManagerApplyBridge(bus, manager, BuildPolicyReloadApply(BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, nil)))
	if err := bus.PublishConfigChange(ConfigChangeEvent{
		Module:     "policy",
		Scope:      "tenant",
		TenantID:   "tenant-live",
		Version:    "cfg_runtime_policy_sensitive_invalid",
		ChangedAt:  time.Now().UTC(),
		PayloadRef: payloadRef,
	}); err != nil {
		t.Fatalf("PublishConfigChange returned error: %v", err)
	}

	status := manager.GetStatus("policy")
	if status.LastSeenEventVersion != "cfg_runtime_policy_sensitive_invalid" {
		t.Fatalf("expected runtime manager to record seen version, got %+v", status)
	}
	if status.LastReloadStatus != "error" || status.LastReloadError == "" {
		t.Fatalf("expected runtime manager reload error, got %+v", status)
	}

	models, err := store.AllowedModels(context.Background(), "tenant-live")
	if err != nil {
		t.Fatalf("AllowedModels returned error: %v", err)
	}
	if len(models) != 1 || models[0] != "baseline-model" {
		t.Fatalf("expected allowed models unchanged on invalid sensitive payload, got %+v", models)
	}

	providerPolicies, err := store.ProviderPolicies(context.Background(), "tenant-live")
	if err != nil {
		t.Fatalf("ProviderPolicies returned error: %v", err)
	}
	if len(providerPolicies) != 1 || providerPolicies[0].Provider != "openai" || providerPolicies[0].Mode != "allow" {
		t.Fatalf("expected provider policies unchanged on invalid sensitive payload, got %+v", providerPolicies)
	}

	rules, err := store.SensitiveRules(context.Background(), "tenant-live")
	if err != nil {
		t.Fatalf("SensitiveRules returned error: %v", err)
	}
	if len(rules) != 1 || rules[0].Pattern != "baseline-secret" || rules[0].Action != "block" {
		t.Fatalf("expected sensitive rules unchanged on invalid sensitive payload, got %+v", rules)
	}
}

