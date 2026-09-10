package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/httpserver"
	"llm-gateway/gateway/internal/policy"
	"llm-gateway/gateway/internal/runtime"
)

const adminToken = "admin-secret"

type versionResponse struct {
	VersionID   string `json:"version_id"`
	Status      string `json:"status"`
	Environment string `json:"environment"`
}

func main() {
	ctx := context.Background()
	cfg := config.Config{AdminAPIKey: adminToken}
	auditor := audit.NewRecorder()
	runtimeBus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(runtimeBus)
	manager := runtime.NewManager()
	policyStore := &policy.Store{}
	svc := controlplane.NewService().
		WithAuditRecorder(auditor).
		WithReleasePublisher(publisher)
	runtime.SubscribeManagerApplyBridge(runtimeBus, manager, runtime.BuildModuleRuntimeApplyDispatcher(map[string]runtime.ModuleRuntimeApplier{
		"policy": runtime.BuildPolicyReloadApply(
			runtime.BuildPolicyPayloadDrivenApplyWithResolver(policyStore, publisher, svc),
		),
	}))
	server := httpserver.New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, policyStore).
		WithControlPlane(svc, auditor, publisher, manager)
	handler := server.Handler()

	verifyReplay(ctx, svc, handler, manager)
	verifyCompensationReplay(ctx, svc, handler, manager)
	verifyRollback(ctx, svc, handler, manager)
	verifyPolicyReplay(ctx, svc, handler, manager, policyStore)
	verifyPolicyRoleReplay(ctx, svc, handler, manager, policyStore)
	verifyPolicyProviderReplay(ctx, svc, handler, manager, policyStore)
	verifyPolicySensitiveReplay(ctx, svc, handler, manager, policyStore)

	fmt.Println("verify result: PASS controlplane runtime replay/compensation/rollback/policy/policy-role/policy-provider/policy-sensitive")
}

func verifyReplay(ctx context.Context, svc *controlplane.Service, handler http.Handler, manager *runtime.Manager) {
	released, err := svc.CreateVersion(ctx, controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_verify_replay",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify replay target",
	})
	if err != nil {
		fail("create replay target", err)
	}

	resp, status := postVersion(handler, "/admin/releases/replay", map[string]any{
		"module":      "router",
		"tenant_id":   "tenant-a",
		"environment": "prod",
		"scope":       "tenant",
		"version_id":  released.Version,
	})
	if status != http.StatusOK {
		fail("replay released version", fmt.Errorf("unexpected status %d", status))
	}
	if resp.VersionID != released.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("verify replay response", fmt.Errorf("unexpected response %+v", resp))
	}

	events, err := getRuntimeEvents(handler)
	if err != nil {
		fail("read runtime events after replay", err)
	}
	if len(events) != 1 || events[0].Version.Version != released.Version {
		fail("verify replay runtime event", fmt.Errorf("unexpected events %+v", events))
	}
	if status := manager.GetStatus("router"); status.LastSeenEventVersion != released.Version || status.LastReloadStatus != "ok" {
		fail("verify replay manager sync", fmt.Errorf("unexpected manager status %+v", status))
	}
	fmt.Println("replay: PASS", released.Version)
}

func verifyCompensationReplay(ctx context.Context, svc *controlplane.Service, handler http.Handler, manager *runtime.Manager) {
	released, err := svc.CreateVersion(ctx, controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "project",
		ProjectID:   "project-a",
		Version:     "cfg_verify_comp_replay",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify compensation replay target",
	})
	if err != nil {
		fail("create compensation replay target", err)
	}

	resp, status := postVersion(handler, "/admin/control-plane/compensations/replay", map[string]any{
		"module":      "router",
		"tenant_id":   "tenant-a",
		"environment": "prod",
		"version":     released.Version,
	})
	if status != http.StatusOK {
		fail("compensation replay released version", fmt.Errorf("unexpected status %d", status))
	}
	if resp.VersionID != released.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("verify compensation replay response", fmt.Errorf("unexpected response %+v", resp))
	}

	events, err := getRuntimeEvents(handler)
	if err != nil {
		fail("read runtime events after compensation replay", err)
	}
	latest := events[0]
	if latest.Version.Version != released.Version || latest.Version.Scope != "project" || latest.Version.ProjectID != "project-a" {
		fail("verify compensation replay runtime event", fmt.Errorf("unexpected latest event %+v", latest))
	}
	if status := manager.GetStatus("router"); status.LastSeenEventVersion != released.Version || status.LastReloadStatus != "ok" {
		fail("verify compensation replay manager sync", fmt.Errorf("unexpected manager status %+v", status))
	}
	fmt.Println("compensation replay: PASS", released.Version)
}

func verifyRollback(ctx context.Context, svc *controlplane.Service, handler http.Handler, manager *runtime.Manager) {
	target, err := svc.CreateVersion(ctx, controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_verify_rollback_target",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify rollback target",
		Config: map[string]string{
			"policy": `{"type":"direct","model":"gpt-4o-mini"}`,
		},
	})
	if err != nil {
		fail("create rollback target", err)
	}

	resp, status := postVersion(handler, "/admin/releases/rollback", map[string]any{
		"module":      "router",
		"tenant_id":   "tenant-a",
		"environment": "prod",
		"scope":       "tenant",
		"version_id":  target.Version,
		"actor":       "verify",
		"reason":      "rollback to known good",
	})
	if status != http.StatusOK {
		fail("rollback released version", fmt.Errorf("unexpected status %d", status))
	}
	if resp.VersionID == target.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("verify rollback response", fmt.Errorf("unexpected response %+v", resp))
	}

	events, err := getRuntimeEvents(handler)
	if err != nil {
		fail("read runtime events after rollback", err)
	}
	latest := events[0]
	if latest.Version.Version != resp.VersionID || latest.Version.SourceVersion != target.Version {
		fail("verify rollback runtime event", fmt.Errorf("unexpected latest event %+v", latest))
	}
	if status := manager.GetStatus("router"); status.LastSeenEventVersion != resp.VersionID || status.LastReloadStatus != "ok" {
		fail("verify rollback manager sync", fmt.Errorf("unexpected manager status %+v", status))
	}
	fmt.Println("rollback: PASS", resp.VersionID, "source_version:", target.Version)
}

func verifyPolicyReplay(ctx context.Context, svc *controlplane.Service, handler http.Handler, manager *runtime.Manager, policyStore *policy.Store) {
	released, err := svc.CreateVersion(ctx, controlplane.CreateVersionInput{
		Module:      "policy",
		TenantID:    "tenant-policy",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_verify_policy_replay",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify policy replay target",
		Config: map[string]string{
			"models/allowed_models": `["gpt-4o-mini","claude-sonnet"]`,
		},
	})
	if err != nil {
		fail("create policy replay target", err)
	}

	resp, status := postVersion(handler, "/admin/releases/replay", map[string]any{
		"module":      "policy",
		"tenant_id":   "tenant-policy",
		"environment": "prod",
		"scope":       "tenant",
		"version_id":  released.Version,
	})
	if status != http.StatusOK {
		fail("replay released policy version", fmt.Errorf("unexpected status %d", status))
	}
	if resp.VersionID != released.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("verify policy replay response", fmt.Errorf("unexpected response %+v", resp))
	}

	allowedModels, err := policyStore.AllowedModels(context.Background(), "tenant-policy")
	if err != nil {
		fail("read policy overlay after replay", err)
	}
	if len(allowedModels) != 2 || allowedModels[0] != "gpt-4o-mini" || allowedModels[1] != "claude-sonnet" {
		fail("verify policy overlay mutation", fmt.Errorf("unexpected allowed models %+v", allowedModels))
	}

	events, err := getRuntimeEvents(handler)
	if err != nil {
		fail("read runtime events after policy replay", err)
	}
	latest := events[0]
	if latest.Version.Module != "policy" || latest.Version.Version != released.Version {
		fail("verify policy replay runtime event", fmt.Errorf("unexpected latest event %+v", latest))
	}
	if status := manager.GetStatus("policy"); status.LastSeenEventVersion != released.Version || status.LastReloadStatus != "ok" {
		fail("verify policy replay manager sync", fmt.Errorf("unexpected manager status %+v", status))
	}
	if status := manager.GetStatus("router"); status.LastSeenEventVersion == released.Version {
		fail("verify policy replay isolation", fmt.Errorf("policy replay unexpectedly touched router status %+v", status))
	}
	if status := manager.GetStatus("quota"); status.LastSeenEventVersion == released.Version {
		fail("verify policy replay isolation", fmt.Errorf("policy replay unexpectedly touched quota status %+v", status))
	}

	fmt.Println("policy replay: PASS", released.Version, "allowed_models:", allowedModels)
}

func verifyPolicyRoleReplay(ctx context.Context, svc *controlplane.Service, handler http.Handler, manager *runtime.Manager, policyStore *policy.Store) {
	released, err := svc.CreateVersion(ctx, controlplane.CreateVersionInput{
		Module:      "policy",
		TenantID:    "tenant-role",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_verify_policy_role_replay",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify policy role replay target",
		Config: map[string]string{
			"roles/bindings": `[{"subject":"alice","role":"admin"},{"subject":"bob","role":"readonly"}]`,
		},
	})
	if err != nil {
		fail("create policy role replay target", err)
	}

	resp, status := postVersion(handler, "/admin/releases/replay", map[string]any{
		"module":      "policy",
		"tenant_id":   "tenant-role",
		"environment": "prod",
		"scope":       "tenant",
		"version_id":  released.Version,
	})
	if status != http.StatusOK {
		fail("replay released policy role version", fmt.Errorf("unexpected status %d", status))
	}
	if resp.VersionID != released.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("verify policy role replay response", fmt.Errorf("unexpected response %+v", resp))
	}

	aliceRole, err := policyStore.RoleFor(context.Background(), "tenant-role", "alice")
	if err != nil {
		fail("read policy role overlay for alice after replay", err)
	}
	if aliceRole != "admin" {
		fail("verify policy role overlay alice mutation", fmt.Errorf("unexpected role %q", aliceRole))
	}
	bobRole, err := policyStore.RoleFor(context.Background(), "tenant-role", "bob")
	if err != nil {
		fail("read policy role overlay for bob after replay", err)
	}
	if bobRole != "readonly" {
		fail("verify policy role overlay bob mutation", fmt.Errorf("unexpected role %q", bobRole))
	}

	events, err := getRuntimeEvents(handler)
	if err != nil {
		fail("read runtime events after policy role replay", err)
	}
	latest := events[0]
	if latest.Version.Module != "policy" || latest.Version.Version != released.Version {
		fail("verify policy role replay runtime event", fmt.Errorf("unexpected latest event %+v", latest))
	}
	if status := manager.GetStatus("policy"); status.LastSeenEventVersion != released.Version || status.LastReloadStatus != "ok" {
		fail("verify policy role replay manager sync", fmt.Errorf("unexpected manager status %+v", status))
	}

	fmt.Println("policy role replay: PASS", released.Version, "roles:", map[string]string{"alice": aliceRole, "bob": bobRole})
}

func verifyPolicyProviderReplay(ctx context.Context, svc *controlplane.Service, handler http.Handler, manager *runtime.Manager, policyStore *policy.Store) {
	released, err := svc.CreateVersion(ctx, controlplane.CreateVersionInput{
		Module:      "policy",
		TenantID:    "tenant-provider",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_verify_policy_provider_replay",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify policy provider replay target",
		Config: map[string]string{
			"providers/policies": `[{"provider":"openai","mode":"deny","enabled":true},{"provider":"anthropic","mode":"allow","enabled":true}]`,
		},
	})
	if err != nil {
		fail("create policy provider replay target", err)
	}

	resp, status := postVersion(handler, "/admin/releases/replay", map[string]any{
		"module":      "policy",
		"tenant_id":   "tenant-provider",
		"environment": "prod",
		"scope":       "tenant",
		"version_id":  released.Version,
	})
	if status != http.StatusOK {
		fail("replay released policy provider version", fmt.Errorf("unexpected status %d", status))
	}
	if resp.VersionID != released.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("verify policy provider replay response", fmt.Errorf("unexpected response %+v", resp))
	}

	providerPolicies, err := policyStore.ProviderPolicies(context.Background(), "tenant-provider")
	if err != nil {
		fail("read policy provider overlay after replay", err)
	}
	if len(providerPolicies) != 2 {
		fail("verify policy provider overlay mutation", fmt.Errorf("unexpected provider policies %+v", providerPolicies))
	}
	if providerPolicies[0].Provider != "openai" || providerPolicies[0].Mode != "deny" || !providerPolicies[0].Enabled {
		fail("verify first provider policy", fmt.Errorf("unexpected provider policy %+v", providerPolicies[0]))
	}
	if providerPolicies[1].Provider != "anthropic" || providerPolicies[1].Mode != "allow" || !providerPolicies[1].Enabled {
		fail("verify second provider policy", fmt.Errorf("unexpected provider policy %+v", providerPolicies[1]))
	}

	events, err := getRuntimeEvents(handler)
	if err != nil {
		fail("read runtime events after policy provider replay", err)
	}
	latest := events[0]
	if latest.Version.Module != "policy" || latest.Version.Version != released.Version {
		fail("verify policy provider replay runtime event", fmt.Errorf("unexpected latest event %+v", latest))
	}
	if status := manager.GetStatus("policy"); status.LastSeenEventVersion != released.Version || status.LastReloadStatus != "ok" {
		fail("verify policy provider replay manager sync", fmt.Errorf("unexpected manager status %+v", status))
	}

	fmt.Println("policy provider replay: PASS", released.Version, "provider_policies:", providerPolicies)
}

func verifyPolicySensitiveReplay(ctx context.Context, svc *controlplane.Service, handler http.Handler, manager *runtime.Manager, policyStore *policy.Store) {
	released, err := svc.CreateVersion(ctx, controlplane.CreateVersionInput{
		Module:      "policy",
		TenantID:    "tenant-sensitive",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_verify_policy_sensitive_replay",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify policy sensitive replay target",
		Config: map[string]string{
			"sensitive/rules": `[{"pattern":"secret","action":"block","enabled":true},{"pattern":"pii","action":"block","enabled":false}]`,
		},
	})
	if err != nil {
		fail("create policy sensitive replay target", err)
	}

	resp, status := postVersion(handler, "/admin/releases/replay", map[string]any{
		"module":      "policy",
		"tenant_id":   "tenant-sensitive",
		"environment": "prod",
		"scope":       "tenant",
		"version_id":  released.Version,
	})
	if status != http.StatusOK {
		fail("replay released policy sensitive version", fmt.Errorf("unexpected status %d", status))
	}
	if resp.VersionID != released.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("verify policy sensitive replay response", fmt.Errorf("unexpected response %+v", resp))
	}

	rules, err := policyStore.SensitiveRules(context.Background(), "tenant-sensitive")
	if err != nil {
		fail("read policy sensitive overlay after replay", err)
	}
	if len(rules) != 2 {
		fail("verify policy sensitive overlay mutation", fmt.Errorf("unexpected sensitive rules %+v", rules))
	}
	if rules[0].Pattern != "secret" || rules[0].Action != "block" || !rules[0].Enabled {
		fail("verify first sensitive rule", fmt.Errorf("unexpected sensitive rule %+v", rules[0]))
	}
	if rules[1].Pattern != "pii" || rules[1].Action != "block" || rules[1].Enabled {
		fail("verify second sensitive rule", fmt.Errorf("unexpected sensitive rule %+v", rules[1]))
	}

	events, err := getRuntimeEvents(handler)
	if err != nil {
		fail("read runtime events after policy sensitive replay", err)
	}
	latest := events[0]
	if latest.Version.Module != "policy" || latest.Version.Version != released.Version {
		fail("verify policy sensitive replay runtime event", fmt.Errorf("unexpected latest event %+v", latest))
	}
	if status := manager.GetStatus("policy"); status.LastSeenEventVersion != released.Version || status.LastReloadStatus != "ok" {
		fail("verify policy sensitive replay manager sync", fmt.Errorf("unexpected manager status %+v", status))
	}

	fmt.Println("policy sensitive replay: PASS", released.Version, "sensitive_rules:", rules)
}

func postVersion(handler http.Handler, path string, payload map[string]any) (versionResponse, int) {
	body, err := json.Marshal(payload)
	if err != nil {
		fail("marshal request body", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		return versionResponse{}, rr.Code
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		fail("decode version response", err)
	}
	return resp, rr.Code
}

func getRuntimeEvents(handler http.Handler) ([]runtime.Event, error) {
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	authorizeAdminRequest(req)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", rr.Code)
	}
	var events []runtime.Event
	if err := json.Unmarshal(rr.Body.Bytes(), &events); err != nil {
		return nil, err
	}
	return events, nil
}

func authorizeAdminRequest(req *http.Request) {
	req.Header.Set("X-Admin-Key", adminToken)
}

func fail(step string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "verify failed at %s: %v\n", step, err)
	os.Exit(1)
}
