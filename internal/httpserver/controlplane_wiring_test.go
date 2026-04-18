package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
	"llm-gateway/gateway/internal/runtime"
)

func TestControlPlaneRoutesReachableViaMainServer(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	publisher := runtime.NewPublisher()
	manager := runtime.NewManager()

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	unauthorized := httptest.NewRecorder()
	unauthorizedReq := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	s.Handler().ServeHTTP(unauthorized, unauthorizedReq)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without admin auth, got %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	authorizedReq := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	authorizedReq.Header.Set("X-Admin-Key", "k")
	s.Handler().ServeHTTP(authorized, authorizedReq)
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected 200 for runtime events route, got %d body=%s", authorized.Code, authorized.Body.String())
	}
}

func TestControlPlaneConfigVersionRouteUsesMainServerPathValueBridge(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	publisher := runtime.NewPublisher()
	manager := runtime.NewManager()

	version, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/config-versions/"+version.Version+"?module=router&tenant_id=tenant-a&environment=prod&scope=tenant", nil)
	req.Header.Set("X-Admin-Key", "k")
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from config version route, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestControlPlaneReleaseFlowPublishesRuntimeEventsViaMainServer(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	publisher := runtime.NewPublisher()
	manager := runtime.NewManager()

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

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	createDraftBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","source_environment":"staging","target_environment":"prod","reason":"seed prod candidate"}`)
	createDraftReq := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", createDraftBody)
	createDraftReq.Header.Set("X-Admin-Key", "k")
	createDraftReq.Header.Set("Content-Type", "application/json")
	createDraftResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(createDraftResp, createDraftReq)
	if createDraftResp.Code != http.StatusCreated {
		t.Fatalf("expected 201 for inheritance draft, got %d body=%s", createDraftResp.Code, createDraftResp.Body.String())
	}

	var draft versionResponse
	if err := json.Unmarshal(createDraftResp.Body.Bytes(), &draft); err != nil {
		t.Fatalf("failed to decode draft response: %v", err)
	}

	releaseBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"` + draft.VersionID + `","actor":"release-bot","reason":"approve draft"}`)
	releaseReq := httptest.NewRequest(http.MethodPost, "/admin/releases", releaseBody)
	releaseReq.Header.Set("X-Admin-Key", "k")
	releaseReq.Header.Set("Content-Type", "application/json")
	releaseResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(releaseResp, releaseReq)
	if releaseResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for release, got %d body=%s", releaseResp.Code, releaseResp.Body.String())
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	eventsReq.Header.Set("X-Admin-Key", "k")
	eventsResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(eventsResp, eventsReq)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for runtime events, got %d body=%s", eventsResp.Code, eventsResp.Body.String())
	}

	var events []runtime.Event
	if err := json.Unmarshal(eventsResp.Body.Bytes(), &events); err != nil {
		t.Fatalf("failed to decode runtime events response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 runtime event, got %d", len(events))
	}
	if events[0].Version.Version != draft.VersionID {
		t.Fatalf("expected runtime event version %q, got %q", draft.VersionID, events[0].Version.Version)
	}
}

func TestControlPlaneReleaseReplayRoutePublishesRuntimeEventsViaMainServer(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	publisher := runtime.NewPublisher()
	manager := runtime.NewManager()

	released, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_replay",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released for replay",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	replayBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"` + released.Version + `"}`)
	replayReq := httptest.NewRequest(http.MethodPost, "/admin/releases/replay", replayBody)
	replayReq.Header.Set("X-Admin-Key", "k")
	replayReq.Header.Set("Content-Type", "application/json")
	replayResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(replayResp, replayReq)
	if replayResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for release replay, got %d body=%s", replayResp.Code, replayResp.Body.String())
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	eventsReq.Header.Set("X-Admin-Key", "k")
	eventsResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(eventsResp, eventsReq)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for runtime events route, got %d body=%s", eventsResp.Code, eventsResp.Body.String())
	}

	var events []runtime.Event
	if err := json.Unmarshal(eventsResp.Body.Bytes(), &events); err != nil {
		t.Fatalf("failed to decode runtime events response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 runtime event after replay route, got %d", len(events))
	}
	if events[0].Version.Version != released.Version {
		t.Fatalf("expected replayed runtime event version %q, got %q", released.Version, events[0].Version.Version)
	}
	if events[0].Apply.PayloadRef != "released://router/tenant-a/prod/tenant//"+released.Version {
		t.Fatalf("expected released payload ref for replay event, got %q", events[0].Apply.PayloadRef)
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != released.Version {
		t.Fatalf("expected runtime manager to sync replayed version %q, got %+v", released.Version, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager reload status ok after replay route, got %+v", status)
	}
}

func TestControlPlaneCompensationReplayRoutePublishesRuntimeEventsViaMainServer(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	publisher := runtime.NewPublisher()
	manager := runtime.NewManager()

	released, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "project",
		ProjectID:   "project-a",
		Version:     "cfg_rel_compensate_main",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released for compensation replay",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	replayBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","version":"` + released.Version + `"}`)
	replayReq := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", replayBody)
	replayReq.Header.Set("X-Admin-Key", "k")
	replayReq.Header.Set("Content-Type", "application/json")
	replayResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(replayResp, replayReq)
	if replayResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for compensation replay, got %d body=%s", replayResp.Code, replayResp.Body.String())
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	eventsReq.Header.Set("X-Admin-Key", "k")
	eventsResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(eventsResp, eventsReq)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for runtime events route, got %d body=%s", eventsResp.Code, eventsResp.Body.String())
	}

	var events []runtime.Event
	if err := json.Unmarshal(eventsResp.Body.Bytes(), &events); err != nil {
		t.Fatalf("failed to decode runtime events response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 runtime event after compensation replay route, got %d", len(events))
	}
	if events[0].Version.Version != released.Version {
		t.Fatalf("expected replayed runtime event version %q, got %q", released.Version, events[0].Version.Version)
	}
	if events[0].Version.Scope != "project" || events[0].Version.ProjectID != "project-a" {
		t.Fatalf("expected inferred scope/project for compensation replay, got scope=%q project=%q", events[0].Version.Scope, events[0].Version.ProjectID)
	}
}

func TestControlPlaneReleaseFlowAppliesRuntimeManagerViaEventBridge(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	bus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(bus)
	manager := runtime.NewManager()
	runtime.SubscribeManagerApplyBridge(bus, manager, nil)

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

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	createDraftBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","source_environment":"staging","target_environment":"prod","reason":"seed prod candidate"}`)
	createDraftReq := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", createDraftBody)
	createDraftReq.Header.Set("X-Admin-Key", "k")
	createDraftReq.Header.Set("Content-Type", "application/json")
	createDraftResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(createDraftResp, createDraftReq)
	if createDraftResp.Code != http.StatusCreated {
		t.Fatalf("expected 201 for inheritance draft, got %d body=%s", createDraftResp.Code, createDraftResp.Body.String())
	}

	var draft versionResponse
	if err := json.Unmarshal(createDraftResp.Body.Bytes(), &draft); err != nil {
		t.Fatalf("failed to decode draft response: %v", err)
	}

	releaseBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"` + draft.VersionID + `","actor":"release-bot","reason":"approve draft"}`)
	releaseReq := httptest.NewRequest(http.MethodPost, "/admin/releases", releaseBody)
	releaseReq.Header.Set("X-Admin-Key", "k")
	releaseReq.Header.Set("Content-Type", "application/json")
	releaseResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(releaseResp, releaseReq)
	if releaseResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for release, got %d body=%s", releaseResp.Code, releaseResp.Body.String())
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != draft.VersionID {
		t.Fatalf("expected runtime manager to receive release version %q, got %+v", draft.VersionID, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager reload status ok, got %+v", status)
	}
}

func TestControlPlaneCompensationReplayRoutePublishesRuntimeEventsAndSyncsManagerViaMainServer(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	publisher := runtime.NewPublisher()
	manager := runtime.NewManager()

	released, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "project",
		ProjectID:   "project-a",
		Version:     "cfg_rel_comp_replay",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released for compensation replay",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	replayBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","version":"` + released.Version + `"}`)
	replayReq := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", replayBody)
	replayReq.Header.Set("X-Admin-Key", "k")
	replayReq.Header.Set("Content-Type", "application/json")
	replayResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(replayResp, replayReq)
	if replayResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for compensation replay, got %d body=%s", replayResp.Code, replayResp.Body.String())
	}

	var replayVersion versionResponse
	if err := json.Unmarshal(replayResp.Body.Bytes(), &replayVersion); err != nil {
		t.Fatalf("failed to decode compensation replay response: %v", err)
	}
	if replayVersion.VersionID != released.Version || replayVersion.Status != controlplane.ConfigStatusReleased {
		t.Fatalf("unexpected compensation replay response: %+v", replayVersion)
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	eventsReq.Header.Set("X-Admin-Key", "k")
	eventsResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(eventsResp, eventsReq)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for runtime events route, got %d body=%s", eventsResp.Code, eventsResp.Body.String())
	}

	var events []runtime.Event
	if err := json.Unmarshal(eventsResp.Body.Bytes(), &events); err != nil {
		t.Fatalf("failed to decode runtime events response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 runtime event after compensation replay route, got %d", len(events))
	}
	if events[0].Version.Version != released.Version {
		t.Fatalf("expected replayed runtime event version %q, got %q", released.Version, events[0].Version.Version)
	}
	if events[0].Version.Scope != "project" || events[0].Version.ProjectID != "project-a" {
		t.Fatalf("expected scope inference to target project release, got scope=%q project=%q", events[0].Version.Scope, events[0].Version.ProjectID)
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != released.Version {
		t.Fatalf("expected runtime manager to sync compensation replayed version %q, got %+v", released.Version, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager reload status ok after compensation replay route, got %+v", status)
	}
}

func TestControlPlaneRollbackRoutePublishesRuntimeEventsAndSyncsManagerViaMainServer(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	publisher := runtime.NewPublisher()
	manager := runtime.NewManager()

	target, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_rollback_target",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "released target for rollback",
		Config: map[string]string{
			"policy": `{"type":"direct","model":"gpt-4o-mini"}`,
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"` + target.Version + `","actor":"release-bot","reason":"rollback to target"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases/rollback", body)
	req.Header.Set("X-Admin-Key", "k")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for rollback, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode rollback response: %v", err)
	}
	if resp.Status != controlplane.ConfigStatusReleased {
		t.Fatalf("expected released rollback response, got %+v", resp)
	}
	if resp.VersionID == target.Version {
		t.Fatalf("expected rollback to create new released version id")
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	eventsReq.Header.Set("X-Admin-Key", "k")
	eventsResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(eventsResp, eventsReq)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for runtime events route, got %d body=%s", eventsResp.Code, eventsResp.Body.String())
	}

	var events []runtime.Event
	if err := json.Unmarshal(eventsResp.Body.Bytes(), &events); err != nil {
		t.Fatalf("failed to decode runtime events response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 runtime event after rollback route, got %d", len(events))
	}
	if events[0].Version.Version != resp.VersionID {
		t.Fatalf("expected rollback runtime event version %q, got %q", resp.VersionID, events[0].Version.Version)
	}
	if events[0].Version.SourceVersion != target.Version {
		t.Fatalf("expected rollback runtime event source version %q, got %q", target.Version, events[0].Version.SourceVersion)
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != resp.VersionID {
		t.Fatalf("expected runtime manager to sync rollback version %q, got %+v", resp.VersionID, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager reload status ok after rollback route, got %+v", status)
	}
}

func TestControlPlaneReplayReleasedVersionAppliesRuntimeManagerViaEventBridge(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	bus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(bus)
	manager := runtime.NewManager()
	runtime.SubscribeManagerApplyBridge(bus, manager, nil)

	released, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "quota",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_quota_prod",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod quota released",
		Config: map[string]string{
			"rpm": "120",
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	replayBody := bytes.NewBufferString(`{"module":"quota","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"` + released.Version + `"}`)
	replayReq := httptest.NewRequest(http.MethodPost, "/admin/releases/replay", replayBody)
	replayReq.Header.Set("X-Admin-Key", "k")
	replayReq.Header.Set("Content-Type", "application/json")
	replayResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(replayResp, replayReq)
	if replayResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for release replay, got %d body=%s", replayResp.Code, replayResp.Body.String())
	}

	status := manager.GetStatus("quota")
	if status.LastSeenEventVersion != released.Version {
		t.Fatalf("expected runtime manager to receive replay version %q, got %+v", released.Version, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager reload status ok, got %+v", status)
	}
}

func TestControlPlaneReleaseFlowAppliesRouterRuntimeReloadFromReleasedPayload_NotBootstrapFile(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k", DefaultProvider: "openai", DefaultModel: "gpt-4o-mini"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	bus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(bus)
	manager := runtime.NewManager()
	liveRouter := router.New(cfg.DefaultProvider, cfg.DefaultModel)

	bootstrapPath := filepath.Join(t.TempDir(), "router-bootstrap.json")
	bootstrapModel := "from-bootstrap-file"
	if err := os.WriteFile(bootstrapPath, []byte(`{"policy":{"type":"direct","model":"`+bootstrapModel+`"}}`), 0o600); err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}
	if err := liveRouter.BootstrapFromFile(bootstrapPath); err != nil {
		t.Fatalf("bootstrap live router from startup file: %v", err)
	}

	releasedModel := "from-released-payload"
	runtime.SubscribeManagerApplyBridge(bus, manager, runtime.BuildRouterReloadApply(func(event runtime.ConfigChangeEvent) error {
		if event.Module != "router" {
			return nil
		}
		payloadRef := strings.TrimSpace(event.PayloadRef)
		if !strings.HasPrefix(payloadRef, "released://router/") {
			return fmt.Errorf("unexpected payload ref %q", payloadRef)
		}
		parts := strings.Split(payloadRef, "/")
		if len(parts) < 8 {
			return fmt.Errorf("invalid payload ref %q", payloadRef)
		}
		versionID := parts[len(parts)-1]
		releasedVersion, err := svc.GetVersion(context.Background(), event.Module, event.TenantID, event.Environment, event.Scope, event.ProjectID, versionID)
		if err != nil {
			return err
		}
		policyJSON := strings.TrimSpace(releasedVersion.Config["policy"])
		if policyJSON == "" {
			return fmt.Errorf("released version %s missing router policy", versionID)
		}
		raw := []byte(`{"policy":` + policyJSON + `}`)
		return liveRouter.BootstrapFromJSON(raw)
	}))

	draftVersion, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_router_candidate",
		Status:      controlplane.ConfigStatusDraft,
		Summary:     "router prod candidate",
		Config: map[string]string{
			"policy": `{"type":"direct","model":"` + releasedModel + `"}`,
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	s := New(cfg, nil, nil, liveRouter, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	releaseBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"` + draftVersion.Version + `","actor":"release-bot","reason":"approve draft"}`)
	releaseReq := httptest.NewRequest(http.MethodPost, "/admin/releases", releaseBody)
	releaseReq.Header.Set("X-Admin-Key", "k")
	releaseReq.Header.Set("Content-Type", "application/json")
	releaseResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(releaseResp, releaseReq)
	if releaseResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for release, got %d body=%s", releaseResp.Code, releaseResp.Body.String())
	}

	decision := liveRouter.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}})
	if decision.RouteMode != "policy" || decision.Model != releasedModel {
		t.Fatalf("expected live router policy reload to released payload model %q, got mode=%s model=%s", releasedModel, decision.RouteMode, decision.Model)
	}
	if decision.Model == bootstrapModel {
		t.Fatalf("expected runtime apply from released payload, but router still uses startup bootstrap model %q", bootstrapModel)
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != draftVersion.Version {
		t.Fatalf("expected runtime manager to receive release version %q, got %+v", draftVersion.Version, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager reload status ok, got %+v", status)
	}
}

func TestControlPlaneWiring_RouterApplyResolverSourceOfTruthOverridesPublisherCache(t *testing.T) {
	cfg := config.Config{DefaultProvider: "openai", DefaultModel: "gpt-4o-mini"}
	svc := controlplane.NewService()
	bus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(bus)
	manager := runtime.NewManager()
	liveRouter := router.New(cfg.DefaultProvider, cfg.DefaultModel)

	bootstrapPath := filepath.Join(t.TempDir(), "router-bootstrap.json")
	bootstrapModel := "from-bootstrap-file"
	if err := os.WriteFile(bootstrapPath, []byte(`{"policy":{"type":"direct","model":"`+bootstrapModel+`"}}`), 0o600); err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}
	if err := liveRouter.BootstrapFromFile(bootstrapPath); err != nil {
		t.Fatalf("bootstrap live router from startup file: %v", err)
	}

	runtime.SubscribeManagerApplyBridge(bus, manager, runtime.BuildRouterReloadApply(
		runtime.BuildRouterPayloadDrivenApplyWithResolver(liveRouter, publisher, svc, bootstrapPath),
	))

	versionID := "cfg_resolver_truth"
	resolverModel := "from-controlplane-source-of-truth"
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     versionID,
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "released by controlplane",
		Config: map[string]string{
			"policy": `{"type":"direct","model":"` + resolverModel + `"}`,
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	cacheModel := "from-publisher-cache"
	published := publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     versionID,
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"policy": `{"type":"direct","model":"` + cacheModel + `"}`,
		},
	})
	if !published {
		t.Fatalf("expected stale cache payload to be published")
	}

	bus.PublishConfigChange(runtime.ConfigChangeEvent{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     versionID,
		ChangedAt:   time.Now().UTC(),
		PayloadRef:  "released://router/tenant-a/prod/tenant/project-a/" + versionID,
	})

	decision := liveRouter.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}})
	if decision.RouteMode != "policy" || decision.Model != resolverModel {
		t.Fatalf("expected resolver-first source-of-truth model %q, got mode=%s model=%s", resolverModel, decision.RouteMode, decision.Model)
	}
	if decision.Model == cacheModel {
		t.Fatalf("expected resolver-first apply to ignore publisher cache model %q", cacheModel)
	}
	if decision.Model == bootstrapModel {
		t.Fatalf("expected runtime apply from released source-of-truth, but router still uses bootstrap model %q", bootstrapModel)
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != versionID {
		t.Fatalf("expected runtime manager to receive version %q, got %+v", versionID, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime manager reload status ok, got %+v", status)
	}
}

func TestControlPlaneReleaseFlowRouterRuntimeReloadInvalidPayloadIsSafe(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k", DefaultProvider: "openai", DefaultModel: "gpt-4o-mini"}
	svc := controlplane.NewService()
	auditor := audit.NewRecorder()
	bus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(bus)
	manager := runtime.NewManager()
	liveRouter := router.New(cfg.DefaultProvider, cfg.DefaultModel)
	runtime.SubscribeManagerApplyBridge(bus, manager, runtime.BuildRouterReloadApply(func(event runtime.ConfigChangeEvent) error {
		if event.Module != "router" {
			return nil
		}
		return liveRouter.BootstrapFromJSON([]byte(`{"policy":{"type":"direct","model":"claude-sonnet"}}`))
	}))

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

	s := New(cfg, nil, nil, liveRouter, nil, nil, nil, nil, nil, nil, nil).
		WithControlPlane(svc, auditor, publisher, manager)

	createDraftBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","source_environment":"staging","target_environment":"prod","reason":"seed prod candidate"}`)
	createDraftReq := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", createDraftBody)
	createDraftReq.Header.Set("X-Admin-Key", "k")
	createDraftReq.Header.Set("Content-Type", "application/json")
	createDraftResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(createDraftResp, createDraftReq)
	if createDraftResp.Code != http.StatusCreated {
		t.Fatalf("expected 201 for inheritance draft, got %d body=%s", createDraftResp.Code, createDraftResp.Body.String())
	}

	var draft versionResponse
	if err := json.Unmarshal(createDraftResp.Body.Bytes(), &draft); err != nil {
		t.Fatalf("failed to decode draft response: %v", err)
	}

	releaseBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"` + draft.VersionID + `","actor":"release-bot","reason":"approve draft"}`)
	releaseReq := httptest.NewRequest(http.MethodPost, "/admin/releases", releaseBody)
	releaseReq.Header.Set("X-Admin-Key", "k")
	releaseReq.Header.Set("Content-Type", "application/json")
	releaseResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(releaseResp, releaseReq)
	if releaseResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for release, got %d body=%s", releaseResp.Code, releaseResp.Body.String())
	}

	bus.PublishConfigChange(runtime.ConfigChangeEvent{
		Module:     "router",
		Scope:      "tenant",
		TenantID:   "tenant-a",
		Version:    "cfg_invalid",
		ChangedAt:  manager.GetStatus("router").LastReloadAt,
		PayloadRef: "http://unexpected",
	})

	decision := liveRouter.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}})
	if decision.RouteMode != "policy" || decision.Model != "claude-sonnet" {
		t.Fatalf("expected live router config unchanged after invalid payload, got mode=%s model=%s", decision.RouteMode, decision.Model)
	}

	status := manager.GetStatus("router")
	if status.LastReloadStatus != "error" {
		t.Fatalf("expected runtime manager reload status error for invalid payload, got %+v", status)
	}
	if status.LastReloadError == "" {
		t.Fatalf("expected runtime manager reload error detail for invalid payload, got %+v", status)
	}
}
