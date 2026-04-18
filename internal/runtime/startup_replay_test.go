package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"llm-gateway/gateway/internal/controlplane"
)

type stubRouterReleasedVersionLister struct {
	versions []controlplane.ConfigVersion
}

func (s *stubRouterReleasedVersionLister) ListVersions(_ context.Context, module, tenantID, environment, scope, projectID string) []controlplane.ConfigVersion {
	return s.versions
}

type startupReplayCaptureBus struct {
	events []ConfigChangeEvent
	err    error
}

func (b *startupReplayCaptureBus) PublishConfigChange(event ConfigChangeEvent) error {
	b.events = append(b.events, event)
	if b.err != nil {
		return b.err
	}
	return nil
}

func (b *startupReplayCaptureBus) SubscribeConfigChange(handler func(ConfigChangeEvent)) {}

func TestReplayCurrentReleasedRouterConfig_NoReleased_NoOp(t *testing.T) {
	lister := &stubRouterReleasedVersionLister{versions: []controlplane.ConfigVersion{
		{Module: "router", Source: controlplane.ConfigStatusDraft, Version: "cfg_draft", CreatedAt: time.Now().UTC()},
	}}
	bus := &startupReplayCaptureBus{}

	err := ReplayCurrentReleasedRouterConfig(context.Background(), lister, bus)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(bus.events) != 0 {
		t.Fatalf("expected no published event when no released version, got %d", len(bus.events))
	}
}

func TestReplayCurrentReleasedRouterConfig_PublishesLatestReleasedRouterEvent(t *testing.T) {
	older := time.Now().UTC().Add(-time.Minute)
	newer := time.Now().UTC()
	lister := &stubRouterReleasedVersionLister{versions: []controlplane.ConfigVersion{
		{
			Module:      "router",
			TenantID:    "tenant-a",
			Environment: "staging",
			Scope:       "tenant",
			ProjectID:   "project-a",
			Version:     "cfg_old",
			Source:      controlplane.ConfigStatusReleased,
			CreatedAt:   older,
		},
		{
			Module:      "router",
			TenantID:    "tenant-a",
			Environment: "prod",
			Scope:       "tenant",
			ProjectID:   "project-a",
			Version:     "cfg_new",
			Source:      controlplane.ConfigStatusReleased,
			CreatedAt:   newer,
		},
	}}
	bus := &startupReplayCaptureBus{}

	err := ReplayCurrentReleasedRouterConfig(context.Background(), lister, bus)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(bus.events) != 1 {
		t.Fatalf("expected one published event, got %d", len(bus.events))
	}
	event := bus.events[0]
	if event.Module != "router" {
		t.Fatalf("expected module router, got %+v", event)
	}
	if event.Version != "cfg_new" {
		t.Fatalf("expected latest released version cfg_new, got %+v", event)
	}
	if event.PayloadRef != "released://router/tenant-a/prod/tenant/project-a/cfg_new" {
		t.Fatalf("expected released payload ref, got %q", event.PayloadRef)
	}
}

func TestReplayCurrentReleasedRouterConfig_PropagatesBusError(t *testing.T) {
	lister := &stubRouterReleasedVersionLister{versions: []controlplane.ConfigVersion{
		{
			Module:      "router",
			TenantID:    "tenant-a",
			Environment: "prod",
			Scope:       "tenant",
			ProjectID:   "project-a",
			Version:     "cfg_new",
			Source:      controlplane.ConfigStatusReleased,
			CreatedAt:   time.Now().UTC(),
		},
	}}
	bus := &startupReplayCaptureBus{err: errors.New("publish failed")}

	err := ReplayCurrentReleasedRouterConfig(context.Background(), lister, bus)
	if err == nil {
		t.Fatalf("expected publish error")
	}
	if err.Error() != "publish failed" {
		t.Fatalf("expected publish failed error, got %v", err)
	}
}

func TestReplayCurrentReleasedModuleConfig_QuotaPublishesLatestReleasedQuotaEvent(t *testing.T) {
	older := time.Now().UTC().Add(-time.Minute)
	newer := time.Now().UTC()
	lister := &stubRouterReleasedVersionLister{versions: []controlplane.ConfigVersion{
		{
			Module:      "quota",
			TenantID:    "tenant-a",
			Environment: "staging",
			Scope:       "tenant",
			ProjectID:   "project-a",
			Version:     "cfg_old",
			Source:      controlplane.ConfigStatusReleased,
			CreatedAt:   older,
		},
		{
			Module:      "quota",
			TenantID:    "tenant-a",
			Environment: "prod",
			Scope:       "tenant",
			ProjectID:   "project-a",
			Version:     "cfg_new",
			Source:      controlplane.ConfigStatusReleased,
			CreatedAt:   newer,
		},
	}}
	bus := &startupReplayCaptureBus{}

	err := ReplayCurrentReleasedModuleConfig(context.Background(), lister, bus, "quota")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(bus.events) != 1 {
		t.Fatalf("expected one published event, got %d", len(bus.events))
	}
	event := bus.events[0]
	if event.Module != "quota" {
		t.Fatalf("expected module quota, got %+v", event)
	}
	if event.Version != "cfg_new" {
		t.Fatalf("expected latest released version cfg_new, got %+v", event)
	}
	if event.PayloadRef != "released://quota/tenant-a/prod/tenant/project-a/cfg_new" {
		t.Fatalf("expected released payload ref, got %q", event.PayloadRef)
	}
}

func TestReplayCurrentReleasedModuleConfig_PolicyPublishesLatestReleasedPolicyEvent(t *testing.T) {
	older := time.Now().UTC().Add(-time.Minute)
	newer := time.Now().UTC()
	lister := &stubRouterReleasedVersionLister{versions: []controlplane.ConfigVersion{
		{
			Module:      "policy",
			TenantID:    "tenant-a",
			Environment: "staging",
			Scope:       "tenant",
			ProjectID:   "project-a",
			Version:     "cfg_old",
			Source:      controlplane.ConfigStatusReleased,
			CreatedAt:   older,
			Config: map[string]string{
				"models/allowed_models": `["gpt-4o-mini"]`,
			},
		},
		{
			Module:      "policy",
			TenantID:    "tenant-a",
			Environment: "prod",
			Scope:       "tenant",
			ProjectID:   "project-a",
			Version:     "cfg_new",
			Source:      controlplane.ConfigStatusReleased,
			CreatedAt:   newer,
			Config: map[string]string{
				"models/allowed_models": `["claude-sonnet"]`,
			},
		},
	}}
	bus := &startupReplayCaptureBus{}

	err := ReplayCurrentReleasedModuleConfig(context.Background(), lister, bus, "policy")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(bus.events) != 1 {
		t.Fatalf("expected one published event, got %d", len(bus.events))
	}
	event := bus.events[0]
	if event.Module != "policy" {
		t.Fatalf("expected module policy, got %+v", event)
	}
	if event.Version != "cfg_new" {
		t.Fatalf("expected latest released version cfg_new, got %+v", event)
	}
	if event.PayloadRef != "released://policy/tenant-a/prod/tenant/project-a/cfg_new" {
		t.Fatalf("expected released payload ref, got %q", event.PayloadRef)
	}
}

