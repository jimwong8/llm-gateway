package main

import (
	"fmt"
	"os"
	"time"

	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/runtime"
)

func main() {
	verifyPublishAndManagerSync()
	verifyReloadFailureProducesCompensation()

	fmt.Println("verify result: PASS runtime_bus")
}

func verifyPublishAndManagerSync() {
	bus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(bus)
	manager := runtime.NewManager()

	runtime.SubscribeManagerApplyBridge(bus, manager, nil)

	version := controlplane.ConfigVersion{
		Module:      "policy",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_runtime_bus_ok",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC),
	}

	if !publisher.PublishIfReleased(version) {
		fail("publish released event", fmt.Errorf("expected released version to be published"))
	}

	status := manager.GetStatus("policy")
	if status.LastSeenEventVersion != version.Version {
		fail("manager last seen version", fmt.Errorf("expected %q, got %+v", version.Version, status))
	}
	if status.LastReloadStatus != "ok" {
		fail("manager reload status", fmt.Errorf("expected ok, got %+v", status))
	}
	fmt.Println("runtime bus publish sync: PASS version=", version.Version)
}

func verifyReloadFailureProducesCompensation() {
	bus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(bus)
	manager := runtime.NewManager()

	runtime.SubscribeManagerApplyBridge(bus, manager, func(event runtime.ConfigChangeEvent) error {
		return fmt.Errorf("verify forced reload failure for %s", event.Module)
	})

	version := controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_runtime_bus_fail",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 4, 19, 12, 1, 0, 0, time.UTC),
	}

	if !publisher.PublishIfReleased(version) {
		fail("publish released failure event", fmt.Errorf("expected released version to be published"))
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != version.Version {
		fail("failure path last seen version", fmt.Errorf("expected %q, got %+v", version.Version, status))
	}
	if status.LastReloadStatus != "error" {
		fail("failure path reload status", fmt.Errorf("expected error, got %+v", status))
	}
	if status.LastReloadError == "" {
		fail("failure path reload error", fmt.Errorf("expected non-empty reload error"))
	}

	records := manager.CompensationRecords()
	if len(records) == 0 {
		fail("compensation records", fmt.Errorf("expected compensation record after reload failure"))
	}
	latest := records[len(records)-1]
	if latest.Version != version.Version || latest.FailedStage != controlplane.FailedStageReload {
		fail("compensation record contents", fmt.Errorf("unexpected record %+v", latest))
	}
	fmt.Println("runtime bus compensation: PASS version=", version.Version)
}

func fail(step string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "verify failed at %s: %v\n", step, err)
	os.Exit(1)
}
