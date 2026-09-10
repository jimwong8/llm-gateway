package main

import (
	"os"
	"strings"
	"testing"
)

func TestServerMainWiring_UsesResolverFirstRouterApply(t *testing.T) {
	raw, err := os.ReadFile("server_main.go")
	if err != nil {
		t.Fatalf("read server_main.go: %v", err)
	}
	src := string(raw)

	if !strings.Contains(src, "BuildModuleRuntimeApplyDispatcher(map[string]runtime.ModuleRuntimeApplier{") {
		t.Fatalf("expected legacy server entrypoint to use module runtime dispatcher wiring")
	}
	if !strings.Contains(src, "BuildRouterPayloadDrivenApplyWithResolver(") {
		t.Fatalf("expected legacy server entrypoint to use resolver-first router apply wiring")
	}
	if !strings.Contains(src, "BuildQuotaPayloadDrivenApplyWithResolver(limiter, runtimePublisher, controlPlaneService)") {
		t.Fatalf("expected legacy server entrypoint to wire quota runtime applier with resolver and publisher fallback")
	}
	if !strings.Contains(src, "BuildQuotaReloadApply(") {
		t.Fatalf("expected legacy server entrypoint to include quota reload apply wrapper")
	}
	if !strings.Contains(src, "BuildPolicyReloadApply(") {
		t.Fatalf("expected legacy server entrypoint to include policy reload apply wrapper")
	}
	if !strings.Contains(src, "BuildPolicyPayloadDrivenApplyWithResolver(policyStore, runtimePublisher, controlPlaneService)") {
		t.Fatalf("expected legacy server entrypoint to wire policy runtime applier with resolver and publisher fallback")
	}
	if !strings.Contains(src, "BuildRouterPayloadDrivenApplyWithResolver(modelRouter, runtimePublisher, controlPlaneService, cfg.RouterBootstrapPath)") {
		t.Fatalf("expected resolver-first apply call with controlPlaneService and fallback chain parameters")
	}
	if !strings.Contains(src, "WithControlPlane(controlPlaneService, controlPlaneAudit, runtimePublisher, runtimeManager)") {
		t.Fatalf("expected legacy entrypoint to pass runtime publisher/manager into httpserver control-plane wiring")
	}
	if !strings.Contains(src, "ReplayCurrentReleasedRouterConfig(context.Background(), controlPlaneService, runtimeBus)") {
		t.Fatalf("expected startup replay wiring to reuse runtime bus apply path")
	}
	if !strings.Contains(src, "ReplayCurrentReleasedModuleConfig(context.Background(), controlPlaneService, runtimeBus, \"quota\")") {
		t.Fatalf("expected startup replay wiring to include quota module replay")
	}
	if !strings.Contains(src, "ReplayCurrentReleasedModuleConfig(context.Background(), controlPlaneService, runtimeBus, \"policy\")") {
		t.Fatalf("expected startup replay wiring to include policy module replay")
	}
	if strings.Contains(src, "BuildRouterPayloadDrivenApply(modelRouter, runtimePublisher, cfg.RouterBootstrapPath)") {
		t.Fatalf("unexpected old BuildRouterPayloadDrivenApply wiring in legacy server entrypoint")
	}
}

func TestServerMainWiring_RegistersQuotaRuntimeApply(t *testing.T) {
	raw, err := os.ReadFile("server_main.go")
	if err != nil {
		t.Fatalf("read server_main.go: %v", err)
	}
	src := string(raw)

	if !strings.Contains(src, "BuildQuotaReloadApply(") {
		t.Fatalf("expected legacy server entrypoint to register quota runtime apply gate")
	}
	if !strings.Contains(src, "BuildQuotaPayloadDrivenApplyWithResolver(limiter, runtimePublisher, controlPlaneService)") {
		t.Fatalf("expected legacy entrypoint quota payload-driven apply wiring with limiter + publisher + resolver")
	}
}
