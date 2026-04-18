package runtime

import (
	"testing"
	"time"
)

func TestSubscribeManagerApplyBridgeNoOpReloadMarksStatusOK(t *testing.T) {
	bus := NewInProcessBus()
	manager := NewManager()
	SubscribeManagerApplyBridge(bus, manager, nil)

	err := bus.PublishConfigChange(ConfigChangeEvent{
		Module:    "router",
		Scope:     "tenant",
		TenantID:  "tenant-a",
		Version:   "cfg_rel_prod",
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("PublishConfigChange returned error: %v", err)
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != "cfg_rel_prod" {
		t.Fatalf("expected last seen version cfg_rel_prod, got %+v", status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected last reload status ok, got %+v", status)
	}
}

func TestSubscribeManagerApplyBridgeUsesCustomReload(t *testing.T) {
	bus := NewInProcessBus()
	manager := NewManager()
	called := false
	SubscribeManagerApplyBridge(bus, manager, func(event ConfigChangeEvent) error {
		called = true
		if event.Module != "policy" {
			t.Fatalf("expected policy module, got %+v", event)
		}
		return nil
	})

	err := bus.PublishConfigChange(ConfigChangeEvent{
		Module:    "policy",
		Scope:     "tenant",
		TenantID:  "tenant-a",
		Version:   "v1",
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("PublishConfigChange returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected custom reload to be called")
	}
}

func TestBuildRouterReloadApply_RejectsNonReleasedPayloadRef(t *testing.T) {
	reload := BuildRouterReloadApply(func(ConfigChangeEvent) error { return nil })
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "router", PayloadRef: "http://unexpected"})
	if err == nil {
		t.Fatalf("expected error for invalid payload ref")
	}
}

func TestBuildRouterReloadApply_AppliesReleasedRouterEvent(t *testing.T) {
	called := false
	reload := BuildRouterReloadApply(func(event ConfigChangeEvent) error {
		called = true
		if event.Module != "router" {
			t.Fatalf("expected router module, got %+v", event)
		}
		return nil
	})
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-1/cfg_123"})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if !called {
		t.Fatalf("expected applier to be called")
	}
}

func TestBuildQuotaReloadApply_RejectsNonReleasedPayloadRef(t *testing.T) {
	reload := BuildQuotaReloadApply(func(ConfigChangeEvent) error { return nil })
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "quota", PayloadRef: "http://unexpected"})
	if err == nil {
		t.Fatalf("expected error for invalid quota payload ref")
	}
}

func TestBuildQuotaReloadApply_AppliesReleasedQuotaEvent(t *testing.T) {
	called := false
	reload := BuildQuotaReloadApply(func(event ConfigChangeEvent) error {
		called = true
		if event.Module != "quota" {
			t.Fatalf("expected quota module, got %+v", event)
		}
		return nil
	})
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "quota", PayloadRef: "released://quota/tenant-a/prod/tenant/project-1/cfg_123"})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if !called {
		t.Fatalf("expected quota applier to be called")
	}
}

func TestBuildPolicyReloadApply_RejectsNonReleasedPayloadRef(t *testing.T) {
	reload := BuildPolicyReloadApply(func(ConfigChangeEvent) error { return nil })
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "policy", PayloadRef: "http://unexpected"})
	if err == nil {
		t.Fatalf("expected error for invalid policy payload ref")
	}
}

func TestBuildPolicyReloadApply_AppliesReleasedPolicyEvent(t *testing.T) {
	called := false
	reload := BuildPolicyReloadApply(func(event ConfigChangeEvent) error {
		called = true
		if event.Module != "policy" {
			t.Fatalf("expected policy module, got %+v", event)
		}
		return nil
	})
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-1/cfg_123"})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if !called {
		t.Fatalf("expected policy applier to be called")
	}
}

func TestBuildPolicyReloadApply_SkipsNonPolicyModule(t *testing.T) {
	called := false
	reload := BuildPolicyReloadApply(func(event ConfigChangeEvent) error {
		called = true
		return nil
	})
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-1/cfg_123"})
	if err != nil {
		t.Fatalf("expected nil err for non-policy module, got %v", err)
	}
	if called {
		t.Fatalf("expected policy applier not to be called for non-policy module")
	}
}

func TestBuildQuotaReloadApply_SkipsNonQuotaModule(t *testing.T) {
	called := false
	reload := BuildQuotaReloadApply(func(event ConfigChangeEvent) error {
		called = true
		return nil
	})
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-1/cfg_123"})
	if err != nil {
		t.Fatalf("expected nil err for non-quota module, got %v", err)
	}
	if called {
		t.Fatalf("expected quota applier not to be called for non-quota module")
	}
}

func TestBuildRouterReloadApply_SkipsNonRouterModule(t *testing.T) {
	called := false
	reload := BuildRouterReloadApply(func(event ConfigChangeEvent) error {
		called = true
		return nil
	})
	if reload == nil {
		t.Fatalf("expected reload func")
	}
	err := reload(ConfigChangeEvent{Module: "policy", PayloadRef: "released://policy/tenant-a/prod/tenant/project-1/cfg_123"})
	if err != nil {
		t.Fatalf("expected nil err for non-router module, got %v", err)
	}
	if called {
		t.Fatalf("expected router applier not to be called for non-router module")
	}
}

func TestBuildModuleRuntimeApplyDispatcher_DispatchesRegisteredModuleOnly(t *testing.T) {
	routerCalled := false
	policyCalled := false

	dispatch := BuildModuleRuntimeApplyDispatcher(map[string]ModuleRuntimeApplier{
		"router": func(event ConfigChangeEvent) error {
			routerCalled = true
			if event.Module != "ROUTER" {
				t.Fatalf("expected original module value preserved, got %q", event.Module)
			}
			return nil
		},
		"policy": func(ConfigChangeEvent) error {
			policyCalled = true
			return nil
		},
	})

	if err := dispatch(ConfigChangeEvent{Module: "ROUTER"}); err != nil {
		t.Fatalf("dispatch router failed: %v", err)
	}
	if !routerCalled {
		t.Fatalf("expected router handler to be called")
	}
	if policyCalled {
		t.Fatalf("expected policy handler not to be called")
	}
}

func TestBuildModuleRuntimeApplyDispatcher_SkipsUnknownOrEmptyModule(t *testing.T) {
	called := false
	dispatch := BuildModuleRuntimeApplyDispatcher(map[string]ModuleRuntimeApplier{
		"router": func(ConfigChangeEvent) error {
			called = true
			return nil
		},
	})

	if err := dispatch(ConfigChangeEvent{Module: "quota"}); err != nil {
		t.Fatalf("expected unknown module to be no-op, got %v", err)
	}
	if err := dispatch(ConfigChangeEvent{Module: "   "}); err != nil {
		t.Fatalf("expected empty module to be no-op, got %v", err)
	}
	if called {
		t.Fatalf("expected registered handler not to be called")
	}
}
