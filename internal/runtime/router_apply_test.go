package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
)

type stubReleasedVersionResolver struct {
	version controlplane.ConfigVersion
	err     error
	calls   int
}

func (s *stubReleasedVersionResolver) GetVersion(_ context.Context, module, tenantID, environment, scope, projectID, versionID string) (controlplane.ConfigVersion, error) {
	s.calls++
	if s.err != nil {
		return controlplane.ConfigVersion{}, s.err
	}
	return s.version, nil
}

type routerApplyRecorder struct {
	jsonCalls     [][]byte
	fileCalls     []string
	bootstrapJSON error
	bootstrapFile error
}

func (r *routerApplyRecorder) BootstrapFromJSON(raw []byte) error {
	r.jsonCalls = append(r.jsonCalls, append([]byte(nil), raw...))
	if r.bootstrapJSON != nil {
		return r.bootstrapJSON
	}
	return nil
}

func (r *routerApplyRecorder) BootstrapFromFile(path string) error {
	r.fileCalls = append(r.fileCalls, path)
	if r.bootstrapFile != nil {
		return r.bootstrapFile
	}
	return nil
}

func TestBuildRouterPayloadDrivenApply_UsesReleasedPayloadFirst(t *testing.T) {
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_123",
		ModulePayloads: map[string]any{
			"router": map[string]any{
				"policy": map[string]any{"type": "direct", "model": "claude-sonnet"},
			},
		},
	}})

	recorder := &routerApplyRecorder{}
	apply := BuildRouterPayloadDrivenApply(recorder, publisher, "/tmp/router-bootstrap.json")

	err := apply(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_123"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(recorder.jsonCalls) != 1 {
		t.Fatalf("expected BootstrapFromJSON to be called once, got %d", len(recorder.jsonCalls))
	}
	if len(recorder.fileCalls) != 0 {
		t.Fatalf("expected BootstrapFromFile not to be called, got %d", len(recorder.fileCalls))
	}
}

func TestBuildRouterPayloadDrivenApply_FallbacksToBootstrapFileWhenPayloadMissing(t *testing.T) {
	publisher := NewPublisher()
	recorder := &routerApplyRecorder{}
	apply := BuildRouterPayloadDrivenApply(recorder, publisher, "/tmp/router-bootstrap.json")

	err := apply(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_404"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(recorder.jsonCalls) != 0 {
		t.Fatalf("expected BootstrapFromJSON not to be called, got %d", len(recorder.jsonCalls))
	}
	if len(recorder.fileCalls) != 1 || recorder.fileCalls[0] != "/tmp/router-bootstrap.json" {
		t.Fatalf("expected fallback BootstrapFromFile call with bootstrap path, got %+v", recorder.fileCalls)
	}
}

func TestBuildRouterPayloadDrivenApply_InvalidRouterPayloadDoesNotFallbackAndReturnsError(t *testing.T) {
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_bad",
		ModulePayloads: map[string]any{
			"router": "invalid-type",
		},
	}})

	recorder := &routerApplyRecorder{}
	apply := BuildRouterPayloadDrivenApply(recorder, publisher, "/tmp/router-bootstrap.json")

	err := apply(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_bad"})
	if err == nil {
		t.Fatalf("expected error for invalid router payload type")
	}
	if len(recorder.jsonCalls) != 0 {
		t.Fatalf("expected BootstrapFromJSON not to be called, got %d", len(recorder.jsonCalls))
	}
	if len(recorder.fileCalls) != 0 {
		t.Fatalf("expected BootstrapFromFile not to be called for invalid payload, got %d", len(recorder.fileCalls))
	}
}

func TestBuildRouterPayloadDrivenApply_PropagatesBootstrapFromJSONErrorWithoutFallback(t *testing.T) {
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_parse_err",
		ModulePayloads: map[string]any{
			"router": map[string]any{
				"policy": map[string]any{"type": "direct"},
			},
		},
	}})

	recorder := &routerApplyRecorder{bootstrapJSON: errors.New("parse router bootstrap policy: direct policy requires model")}
	apply := BuildRouterPayloadDrivenApply(recorder, publisher, "/tmp/router-bootstrap.json")

	err := apply(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_parse_err"})
	if err == nil {
		t.Fatalf("expected error from BootstrapFromJSON")
	}
	if len(recorder.jsonCalls) != 1 {
		t.Fatalf("expected BootstrapFromJSON to be called once, got %d", len(recorder.jsonCalls))
	}
	if len(recorder.fileCalls) != 0 {
		t.Fatalf("expected BootstrapFromFile not to be called on payload apply error, got %d", len(recorder.fileCalls))
	}
}

func TestBuildRouterPayloadDrivenApply_LiveRouterChangesByReleasedPayloadNotBootstrapFile(t *testing.T) {
	r := router.New("openai", "gpt-4o-mini")

	bootstrapPath := filepath.Join(t.TempDir(), "router-bootstrap.json")
	if err := os.WriteFile(bootstrapPath, []byte(`{"policy":{"type":"direct","model":"gpt-4o-mini"}}`), 0o600); err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}
	if err := r.BootstrapFromFile(bootstrapPath); err != nil {
		t.Fatalf("bootstrap from file: %v", err)
	}

	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_live_1",
		ModulePayloads: map[string]any{
			"router": map[string]any{
				"policy": map[string]any{"type": "direct", "model": "claude-sonnet"},
			},
		},
	}})

	apply := BuildRouterPayloadDrivenApply(r, publisher, bootstrapPath)
	if err := apply(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_live_1"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	decision := r.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}})
	if decision.RouteMode != "policy" {
		t.Fatalf("expected policy route mode, got %q", decision.RouteMode)
	}
	if decision.Model != "claude-sonnet" {
		t.Fatalf("expected live router model from released payload claude-sonnet, got %q", decision.Model)
	}
}

func TestBuildRouterPayloadDrivenApply_InvalidPayloadDoesNotPartiallyMutateLiveRouter(t *testing.T) {
	r := router.New("openai", "gpt-4o-mini")
	bootstrapPath := filepath.Join(t.TempDir(), "router-bootstrap.json")
	if err := os.WriteFile(bootstrapPath, []byte(`{"policy":{"type":"direct","model":"gpt-4o-mini"}}`), 0o600); err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}
	if err := r.BootstrapFromFile(bootstrapPath); err != nil {
		t.Fatalf("bootstrap from file: %v", err)
	}

	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_live_bad",
		ModulePayloads: map[string]any{
			"router": map[string]any{
				"policy": map[string]any{"type": "direct"},
			},
		},
	}})

	apply := BuildRouterPayloadDrivenApply(r, publisher, bootstrapPath)
	err := apply(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_live_bad"})
	if err == nil {
		t.Fatalf("expected invalid payload apply to return error")
	}

	decision := r.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}})
	if decision.RouteMode != "policy" || decision.Model != "gpt-4o-mini" {
		t.Fatalf("expected live router remain on old bootstrap policy after failed payload apply, got mode=%s model=%s", decision.RouteMode, decision.Model)
	}
}

func TestBuildRouterPayloadDrivenApplyWithResolver_UsesControlplaneVersionFirst(t *testing.T) {
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_123",
		ModulePayloads: map[string]any{
			"router": map[string]any{
				"policy": map[string]any{"type": "direct", "model": "gpt-4o-mini"},
			},
		},
	}})

	resolver := &stubReleasedVersionResolver{version: controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     "cfg_123",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"policy": `{"type":"direct","model":"claude-sonnet"}`,
		},
	}}

	recorder := &routerApplyRecorder{}
	apply := BuildRouterPayloadDrivenApplyWithResolver(recorder, publisher, resolver, "/tmp/router-bootstrap.json")

	err := apply(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_123"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver to be called once, got %d", resolver.calls)
	}
	if len(recorder.jsonCalls) != 1 {
		t.Fatalf("expected BootstrapFromJSON to be called once, got %d", len(recorder.jsonCalls))
	}
	if got := string(recorder.jsonCalls[0]); !strings.Contains(got, "claude-sonnet") {
		t.Fatalf("expected resolved payload from controlplane version, got %s", got)
	}
	if len(recorder.fileCalls) != 0 {
		t.Fatalf("expected BootstrapFromFile not to be called, got %d", len(recorder.fileCalls))
	}
}

func TestBuildRouterPayloadDrivenApplyWithResolver_FallbacksToPublisherOnResolverError(t *testing.T) {
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_404",
		ModulePayloads: map[string]any{
			"router": map[string]any{
				"policy": map[string]any{"type": "direct", "model": "gpt-4o-mini"},
			},
		},
	}})

	resolver := &stubReleasedVersionResolver{err: controlplane.ErrVersionNotFound}
	recorder := &routerApplyRecorder{}
	apply := BuildRouterPayloadDrivenApplyWithResolver(recorder, publisher, resolver, "/tmp/router-bootstrap.json")

	err := apply(ConfigChangeEvent{Module: "router", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_404"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver to be called once, got %d", resolver.calls)
	}
	if len(recorder.jsonCalls) != 1 {
		t.Fatalf("expected BootstrapFromJSON to be called once from publisher fallback, got %d", len(recorder.jsonCalls))
	}
	if got := string(recorder.jsonCalls[0]); !strings.Contains(got, "gpt-4o-mini") {
		t.Fatalf("expected payload from publisher fallback, got %s", got)
	}
	if len(recorder.fileCalls) != 0 {
		t.Fatalf("expected BootstrapFromFile not to be called, got %d", len(recorder.fileCalls))
	}
}

func TestStartupRouterReleasedReplay_UsesResolverFirstApplyAndUpdatesRuntimeStatus(t *testing.T) {
	liveRouter := router.New("openai", "gpt-4o-mini")
	bootstrapPath := filepath.Join(t.TempDir(), "router-bootstrap.json")
	bootstrapModel := "from-startup-bootstrap"
	if err := os.WriteFile(bootstrapPath, []byte(`{"policy":{"type":"direct","model":"`+bootstrapModel+`"}}`), 0o600); err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}
	if err := liveRouter.BootstrapFromFile(bootstrapPath); err != nil {
		t.Fatalf("bootstrap from startup file: %v", err)
	}

	svc := controlplane.NewService()
	versionID := "cfg_startup_replay_1"
	releasedModel := "from-released-source-of-truth"
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     versionID,
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "released for startup replay",
		Config: map[string]string{
			"policy": `{"type":"direct","model":"` + releasedModel + `"}`,
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	publisher := NewPublisher()
	staleModel := "from-stale-publisher-cache"
	published := publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     versionID,
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"policy": `{"type":"direct","model":"` + staleModel + `"}`,
		},
	})
	if !published {
		t.Fatalf("expected stale publisher cache payload to be published")
	}

	bus := NewInProcessBus()
	manager := NewManager()
	SubscribeManagerApplyBridge(bus, manager, BuildRouterReloadApply(
		BuildRouterPayloadDrivenApplyWithResolver(liveRouter, publisher, svc, bootstrapPath),
	))

	changedAt := time.Date(2026, 4, 16, 9, 30, 0, 0, time.UTC)
	err = bus.PublishConfigChange(ConfigChangeEvent{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     versionID,
		ChangedAt:   changedAt,
		PayloadRef:  "released://router/tenant-a/prod/tenant/project-a/" + versionID,
	})
	if err != nil {
		t.Fatalf("PublishConfigChange returned error: %v", err)
	}

	decision := liveRouter.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}})
	if decision.RouteMode != "policy" || decision.Model != releasedModel {
		t.Fatalf("expected startup replay to apply released resolver model %q, got mode=%s model=%s", releasedModel, decision.RouteMode, decision.Model)
	}
	if decision.Model == bootstrapModel {
		t.Fatalf("expected startup replay to override bootstrap model %q", bootstrapModel)
	}
	if decision.Model == staleModel {
		t.Fatalf("expected resolver-first replay to ignore stale publisher cache model %q", staleModel)
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != versionID {
		t.Fatalf("expected last seen event version %q, got %+v", versionID, status)
	}
	if !status.LastSeenEventAt.Equal(changedAt) {
		t.Fatalf("expected last seen event time %v, got %+v", changedAt, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime replay reload status ok, got %+v", status)
	}
	if status.LastReloadError != "" {
		t.Fatalf("expected empty runtime replay reload error, got %+v", status)
	}
}

func TestStartupRouterReleasedReplay_NoReleasedConfigIsNoOpAndStatusOK(t *testing.T) {
	liveRouter := router.New("openai", "gpt-4o-mini")
	startupModel := "startup-live-model"
	if err := liveRouter.BootstrapFromJSON([]byte(`{"policy":{"type":"direct","model":"` + startupModel + `"}}`)); err != nil {
		t.Fatalf("bootstrap startup live router: %v", err)
	}

	svc := controlplane.NewService()
	publisher := NewPublisher()
	bus := NewInProcessBus()
	manager := NewManager()
	SubscribeManagerApplyBridge(bus, manager, BuildRouterReloadApply(
		BuildRouterPayloadDrivenApplyWithResolver(liveRouter, publisher, svc, ""),
	))

	missingVersion := "cfg_startup_missing"
	changedAt := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	err := bus.PublishConfigChange(ConfigChangeEvent{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     missingVersion,
		ChangedAt:   changedAt,
		PayloadRef:  "released://router/tenant-a/prod/tenant/project-a/" + missingVersion,
	})
	if err != nil {
		t.Fatalf("PublishConfigChange returned error: %v", err)
	}

	decision := liveRouter.Decide(providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}})
	if decision.RouteMode != "policy" || decision.Model != startupModel {
		t.Fatalf("expected startup replay no-op to keep startup model %q, got mode=%s model=%s", startupModel, decision.RouteMode, decision.Model)
	}

	status := manager.GetStatus("router")
	if status.LastSeenEventVersion != missingVersion {
		t.Fatalf("expected last seen event version %q, got %+v", missingVersion, status)
	}
	if !status.LastSeenEventAt.Equal(changedAt) {
		t.Fatalf("expected last seen event time %v, got %+v", changedAt, status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected runtime replay status ok for no-op path, got %+v", status)
	}
	if status.LastReloadError != "" {
		t.Fatalf("expected empty runtime replay error for no-op path, got %+v", status)
	}
}
