package runtime

import (
	"context"
	"testing"
	"time"

	"llm-gateway/gateway/internal/controlplane"
	policy "llm-gateway/gateway/internal/policy"
)

type captureBus struct {
	events []ConfigChangeEvent
}

func (b *captureBus) PublishConfigChange(event ConfigChangeEvent) error {
	b.events = append(b.events, event)
	return nil
}

func (b *captureBus) SubscribeConfigChange(handler func(ConfigChangeEvent)) {
	if handler == nil {
		return
	}
}

func TestPublishIfReleasedSkipsInheritanceDraft(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	draft, err := svc.CreateInheritanceDraft(context.Background(), controlplane.CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
		Reason:            "seed prod candidate from staging",
	})
	if err != nil {
		t.Fatalf("CreateInheritanceDraft returned error: %v", err)
	}

	publisher := NewPublisher()
	published := publisher.PublishIfReleased(draft)
	if published {
		t.Fatalf("expected inheritance draft not to publish")
	}
	if got := len(publisher.Events()); got != 0 {
		t.Fatalf("expected 0 events, got %d", got)
	}
}

func TestPublishIfReleasedPublishesReleasedVersion(t *testing.T) {
	released := controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Source:      controlplane.ConfigStatusReleased,
	}

	publisher := NewPublisher()
	published := publisher.PublishIfReleased(released)
	if !published {
		t.Fatalf("expected released version to publish")
	}
	if got := len(publisher.Events()); got != 1 {
		t.Fatalf("expected 1 event, got %d", got)
	}
	if got := publisher.Events()[0].Version.Version; got != released.Version {
		t.Fatalf("expected published version %q, got %q", released.Version, got)
	}
}

func TestPublishIfReleasedPublishesConfigChangeToBus(t *testing.T) {
	released := controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     "cfg_rel_prod",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
	}

	bus := &captureBus{}
	publisher := NewPublisher().WithBus(bus)
	published := publisher.PublishIfReleased(released)
	if !published {
		t.Fatalf("expected released version to publish")
	}
	if got := len(bus.events); got != 1 {
		t.Fatalf("expected 1 bus event, got %d", got)
	}
	event := bus.events[0]
	if event.Module != released.Module || event.TenantID != released.TenantID || event.Environment != released.Environment {
		t.Fatalf("unexpected bus event identity: %+v", event)
	}
	if event.Scope != released.Scope || event.ProjectID != released.ProjectID || event.Version != released.Version {
		t.Fatalf("unexpected bus event routing fields: %+v", event)
	}
	if !event.ChangedAt.Equal(released.CreatedAt) {
		t.Fatalf("expected changed_at to equal release timestamp, got %v want %v", event.ChangedAt, released.CreatedAt)
	}
}

func TestBuildRuntimeApplyPayloadFromVersion(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-1",
		Version:     "cfg_123",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"channels":  `[{"id":"code-primary","provider":"mock-code","model":"deepseek-coder","task":"code","enabled":true,"priority":1,"weight":1}]`,
			"abilities": `[{"id":"tenant-code","task":"code","channel_ids":["code-primary"],"enabled":true,"priority":1}]`,
			"policy":    `{"type":"direct","model":"claude-sonnet"}`,
		},
	}

	payload := BuildRuntimeApplyPayload(version)

	if payload.Module != version.Module || payload.TenantID != version.TenantID || payload.Environment != version.Environment {
		t.Fatalf("unexpected identity fields in payload: %+v", payload)
	}
	if payload.Version != version.Version {
		t.Fatalf("expected payload version %q, got %q", version.Version, payload.Version)
	}
	if payload.PayloadRef == "" {
		t.Fatalf("expected non-empty payload ref")
	}
	if payload.PayloadRef != "released://router/tenant-a/prod/tenant/project-1/cfg_123" {
		t.Fatalf("unexpected payload ref: %q", payload.PayloadRef)
	}
	if len(payload.ModulePayloads) != 1 {
		t.Fatalf("expected router module payload only, got %+v", payload.ModulePayloads)
	}
	raw, ok := payload.ModulePayloads["router"]
	if !ok {
		t.Fatalf("expected router module payload, got %+v", payload.ModulePayloads)
	}
	routerPayload, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected router payload map, got %T", raw)
	}
	if routerPayload["channels"] == nil || routerPayload["abilities"] == nil || routerPayload["policy"] == nil {
		t.Fatalf("expected router payload to include channels/abilities/policy, got %+v", routerPayload)
	}
}

func TestBuildRuntimeApplyPayloadIncludesQuotaModulePayloadFromRPM(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "quota",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_quota_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"rpm": "75",
		},
	}

	payload := BuildRuntimeApplyPayload(version)

	raw, ok := payload.ModulePayloads["quota"]
	if !ok {
		t.Fatalf("expected quota payload in module_payloads, got %+v", payload.ModulePayloads)
	}
	quotaPayload, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected quota payload map, got %T", raw)
	}
	if got, ok := quotaPayload["rpm"].(int); !ok || got != 75 {
		t.Fatalf("expected quota payload rpm=75, got %#v", quotaPayload["rpm"])
	}
}

func TestBuildRuntimeApplyPayloadIncludesPolicyModulePayloadFromAllowedModels(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"models/allowed_models": ` ["gpt-4o-mini","claude-sonnet"] `,
		},
	}

	payload := BuildRuntimeApplyPayload(version)

	raw, ok := payload.ModulePayloads["policy"]
	if !ok {
		t.Fatalf("expected policy payload in module_payloads, got %+v", payload.ModulePayloads)
	}
	policyPayload, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload map, got %T", raw)
	}
	modelsRaw, ok := policyPayload["models"].(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload models object, got %T", policyPayload["models"])
	}
	allowedRaw, ok := modelsRaw["allowed_models"].([]string)
	if !ok {
		t.Fatalf("expected policy allowed_models []string, got %T", modelsRaw["allowed_models"])
	}
	if len(allowedRaw) != 2 || allowedRaw[0] != "gpt-4o-mini" || allowedRaw[1] != "claude-sonnet" {
		t.Fatalf("unexpected policy allowed_models payload: %+v", allowedRaw)
	}
}

func TestBuildRuntimeApplyPayloadIncludesPolicyModulePayloadRoleBindings(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_role_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"roles/bindings": `[{"subject":"alice","role":"admin"}]`,
		},
	}

	payload := BuildRuntimeApplyPayload(version)

	raw, ok := payload.ModulePayloads["policy"]
	if !ok {
		t.Fatalf("expected policy payload in module_payloads, got %+v", payload.ModulePayloads)
	}
	policyPayload, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload map, got %T", raw)
	}
	rolesRaw, ok := policyPayload["roles"].(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload roles object, got %T", policyPayload["roles"])
	}
	bindingsRaw, ok := rolesRaw["bindings"].([]policy.TenantRoleBinding)
	if !ok {
		t.Fatalf("expected policy roles bindings []TenantRoleBinding, got %T", rolesRaw["bindings"])
	}
	if len(bindingsRaw) != 1 || bindingsRaw[0].Subject != "alice" || bindingsRaw[0].Role != "admin" {
		t.Fatalf("unexpected policy role bindings payload: %+v", bindingsRaw)
	}
}

func TestBuildRuntimeApplyPayloadIncludesPolicyModulePayloadProviderPolicies(t *testing.T) {
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

	payload := BuildRuntimeApplyPayload(version)
	raw, ok := payload.ModulePayloads["policy"]
	if !ok {
		t.Fatalf("expected policy payload in module_payloads, got %+v", payload.ModulePayloads)
	}
	policyPayload, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload map, got %T", raw)
	}
	providersRaw, ok := policyPayload["providers"].(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload providers object, got %T", policyPayload["providers"])
	}
	policiesRaw, ok := providersRaw["policies"].([]policy.TenantProviderPolicy)
	if !ok {
		t.Fatalf("expected policy providers policies []TenantProviderPolicy, got %T", providersRaw["policies"])
	}
	if len(policiesRaw) != 1 || policiesRaw[0].Provider != "openai" || policiesRaw[0].Mode != "deny" || !policiesRaw[0].Enabled {
		t.Fatalf("unexpected policy provider policies payload: %+v", policiesRaw)
	}
}


func TestBuildRuntimeApplyPayloadIncludesPolicyModulePayloadSensitiveRules(t *testing.T) {
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

	payload := BuildRuntimeApplyPayload(version)
	raw, ok := payload.ModulePayloads["policy"]
	if !ok {
		t.Fatalf("expected policy payload in module_payloads, got %+v", payload.ModulePayloads)
	}
	policyPayload, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload map, got %T", raw)
	}
	sensitiveRaw, ok := policyPayload["sensitive"].(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload sensitive object, got %T", policyPayload["sensitive"])
	}
	rulesRaw, ok := sensitiveRaw["rules"].([]policy.SensitiveRule)
	if !ok {
		t.Fatalf("expected policy sensitive rules []SensitiveRule, got %T", sensitiveRaw["rules"])
	}
	if len(rulesRaw) != 1 || rulesRaw[0].Pattern != "secret" || rulesRaw[0].Action != "block" || !rulesRaw[0].Enabled {
		t.Fatalf("unexpected policy sensitive rules payload: %+v", rulesRaw)
	}
}

func TestBuildRuntimeApplyPayloadIncludesPolicyModulePayloadAllPolicyOverlaysTogether(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_policy_all_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"models/allowed_models": `["gpt-4o-mini"]`,
			"roles/bindings":       `[{"subject":"alice","role":"admin"}]`,
			"providers/policies":    `[{"provider":"openai","mode":"deny","enabled":true}]`,
			"sensitive/rules":       `[{"pattern":"secret","action":"block","enabled":true}]`,
		},
	}

	payload := BuildRuntimeApplyPayload(version)
	raw, ok := payload.ModulePayloads["policy"]
	if !ok {
		t.Fatalf("expected policy payload in module_payloads, got %+v", payload.ModulePayloads)
	}
	policyPayload, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected policy payload map, got %T", raw)
	}
	if policyPayload["models"] == nil || policyPayload["roles"] == nil || policyPayload["providers"] == nil || policyPayload["sensitive"] == nil {
		t.Fatalf("expected models/roles/providers/sensitive all present, got %+v", policyPayload)
	}
}

func TestBuildRuntimeApplyPayloadQuotaMissingRPMProducesNoPayload(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "quota",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_quota_missing_rpm",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"limit": "33",
		},
	}

	payload := BuildRuntimeApplyPayload(version)
	if _, ok := payload.ModulePayloads["quota"]; ok {
		t.Fatalf("expected quota payload to be absent when rpm missing, got %+v", payload.ModulePayloads)
	}
}

func TestBuildRuntimeApplyPayloadQuotaInvalidConfigProducesNoQuotaPayload(t *testing.T) {
	version := controlplane.ConfigVersion{
		Module:      "quota",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_quota_bad",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"rpm": "invalid",
		},
	}

	payload := BuildRuntimeApplyPayload(version)
	if _, ok := payload.ModulePayloads["quota"]; ok {
		t.Fatalf("expected invalid quota config to produce no payload, got %+v", payload.ModulePayloads)
	}
}

func TestPublishIfReleasedBuildsRuntimeApplyPayload(t *testing.T) {
	released := controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Source:      controlplane.ConfigStatusReleased,
	}

	publisher := NewPublisher()
	published := publisher.PublishIfReleased(released)
	if !published {
		t.Fatalf("expected released version to publish")
	}
	events := publisher.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Apply.Module != "router" {
		t.Fatalf("expected apply payload module router, got %q", events[0].Apply.Module)
	}
	if events[0].Apply.Version != released.Version {
		t.Fatalf("expected apply payload version %q, got %q", released.Version, events[0].Apply.Version)
	}
	if events[0].Apply.PayloadRef == "" {
		t.Fatalf("expected apply payload ref to be set")
	}
}

func TestFindApplyPayloadByRefReturnsLatestMatch(t *testing.T) {
	publisher := NewPublisher()
	publisher.events = append(publisher.events,
		Event{Apply: RuntimeApplyPayload{PayloadRef: "released://router/t/prod/tenant/p/cfg_1", Version: "cfg_1"}},
		Event{Apply: RuntimeApplyPayload{PayloadRef: "released://router/t/prod/tenant/p/cfg_2", Version: "cfg_2"}},
		Event{Apply: RuntimeApplyPayload{PayloadRef: "released://router/t/prod/tenant/p/cfg_2", Version: "cfg_2_newer"}},
	)

	payload, ok := publisher.FindApplyPayloadByRef("released://router/t/prod/tenant/p/cfg_2")
	if !ok {
		t.Fatalf("expected payload to be found")
	}
	if payload.Version != "cfg_2_newer" {
		t.Fatalf("expected latest payload version cfg_2_newer, got %q", payload.Version)
	}
}

func TestFindApplyPayloadByRefReturnsFalseForUnknownRef(t *testing.T) {
	publisher := NewPublisher()
	payload, ok := publisher.FindApplyPayloadByRef("released://router/t/prod/tenant/p/missing")
	if ok {
		t.Fatalf("expected payload not found, got %+v", payload)
	}
}
