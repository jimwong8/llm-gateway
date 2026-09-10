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

	verifyCompensationReplay(handler, svc, manager)

	fmt.Println("verify result: PASS compensation replay")
}

func verifyCompensationReplay(handler http.Handler, svc *controlplane.Service, manager *runtime.Manager) {
	released, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "project",
		ProjectID:   "project-a",
		Version:     "cfg_verify_compensation_only",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify compensation replay target",
	})
	if err != nil {
		fail("create compensation target", err)
	}

	resp, status := postVersion(handler, "/admin/control-plane/compensations/replay", map[string]any{
		"module":      "router",
		"tenant_id":   "tenant-a",
		"environment": "prod",
		"version":     released.Version,
	})
	if status != http.StatusOK {
		fail("compensation replay status", fmt.Errorf("expected 200, got %d", status))
	}
	if resp.VersionID != released.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("compensation replay response", fmt.Errorf("unexpected response %+v", resp))
	}

	statusSnapshot := manager.GetStatus("router")
	if statusSnapshot.LastSeenEventVersion != released.Version {
		fail("compensation replay manager sync", fmt.Errorf("unexpected manager status %+v", statusSnapshot))
	}
	if statusSnapshot.LastReloadStatus != "ok" {
		fail("compensation replay manager reload status", fmt.Errorf("unexpected manager status %+v", statusSnapshot))
	}

	fmt.Println("compensation replay: PASS", released.Version)
}

func postVersion(handler http.Handler, path string, payload map[string]any) (versionResponse, int) {
	body, err := json.Marshal(payload)
	if err != nil {
		fail("marshal admin request body", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", adminToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		return versionResponse{}, rr.Code
	}
	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		fail("decode admin version response", err)
	}
	return resp, rr.Code
}

func fail(step string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "verify failed at %s: %v\n", step, err)
	os.Exit(1)
}
