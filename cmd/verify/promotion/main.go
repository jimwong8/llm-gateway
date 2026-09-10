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
	publisher := runtime.NewPublisher()
	manager := runtime.NewManager()
	policyStore := &policy.Store{}
	svc := controlplane.NewService().
		WithAuditRecorder(auditor).
		WithReleasePublisher(publisher)
	server := httpserver.New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, policyStore).
		WithControlPlane(svc, auditor, publisher, manager)
	handler := server.Handler()

	verifyPromotion(handler, svc)

	fmt.Println("verify result: PASS promotion")
}

func verifyPromotion(handler http.Handler, svc *controlplane.Service) {
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_verify_promotion_source",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify promotion source",
	})
	if err != nil {
		fail("create promotion source", err)
	}

	resp, status := postVersion(handler, "/admin/promotions", map[string]any{
		"module":             "router",
		"tenant_id":          "tenant-a",
		"source_environment": "staging",
		"target_environment": "prod",
		"scope":              "tenant",
		"source_version":     "cfg_verify_promotion_source",
		"actor":              "verify",
		"reason":             "promote to prod",
	})
	if status != http.StatusOK {
		fail("promotion status", fmt.Errorf("expected 200, got %d", status))
	}
	if resp.VersionID == "" || resp.VersionID == "cfg_verify_promotion_source" {
		fail("promotion response", fmt.Errorf("expected new released version id, got %+v", resp))
	}
	if resp.Status != controlplane.ConfigStatusReleased {
		fail("promotion response status", fmt.Errorf("unexpected response %+v", resp))
	}
	if resp.Environment != "prod" {
		fail("promotion response environment", fmt.Errorf("unexpected response %+v", resp))
	}

	fmt.Println("promotion: PASS", resp.VersionID, "target_environment:", resp.Environment)
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
