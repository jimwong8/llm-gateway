package controlplane

import (
	"context"
	"testing"
	"time"

	"llm-gateway/gateway/internal/audit"
)

type stubReleasePublisher struct {
	events []ConfigVersion
}

func (p *stubReleasePublisher) PublishIfReleased(version ConfigVersion) bool {
	if version.Source != ConfigStatusReleased {
		return false
	}
	p.events = append(p.events, version)
	return true
}

func TestCreateInheritanceDraftDoesNotReleaseTargetEnvironment(t *testing.T) {
	svc := NewService()
	baseTime := time.Date(2026, 3, 24, 19, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return baseTime }

	sourceReleased, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      ConfigStatusReleased,
		Summary:     "staging released",
		Config: map[string]string{
			"model": "gpt-4.1",
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion source returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(time.Minute) }
	targetReleased, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Status:      ConfigStatusReleased,
		Summary:     "prod released",
		Config: map[string]string{
			"model": "gpt-4o-mini",
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion target returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(2 * time.Minute) }
	draft, err := svc.CreateInheritanceDraft(context.Background(), CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
		Reason:            "seed prod candidate from staging",
		Actor:             "architect",
	})
	if err != nil {
		t.Fatalf("CreateInheritanceDraft returned error: %v", err)
	}

	if draft.Source != SourceTypeInheritance {
		t.Fatalf("expected draft source %q, got %q", SourceTypeInheritance, draft.Source)
	}
	if draft.SourceEnvironment != sourceReleased.Environment {
		t.Fatalf("expected source environment %q, got %q", sourceReleased.Environment, draft.SourceEnvironment)
	}
	if draft.SourceVersion != sourceReleased.Version {
		t.Fatalf("expected source version %q, got %q", sourceReleased.Version, draft.SourceVersion)
	}
	if draft.Environment != "prod" {
		t.Fatalf("expected target environment prod, got %q", draft.Environment)
	}

	current, err := svc.CurrentReleased(context.Background(), "router", "tenant-a", "prod", "tenant", "")
	if err != nil {
		t.Fatalf("CurrentReleased returned error: %v", err)
	}
	if current.Version != targetReleased.Version {
		t.Fatalf("expected prod released version %q, got %q", targetReleased.Version, current.Version)
	}
	if current.Version == sourceReleased.Version {
		t.Fatalf("inheritance draft must not become released automatically")
	}
}

func TestReleaseDraftPromotesTargetEnvironmentReleasedPointer(t *testing.T) {
	svc := NewService()
	baseTime := time.Date(2026, 3, 24, 19, 30, 0, 0, time.UTC)
	svc.now = func() time.Time { return baseTime }

	_, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion source returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(time.Minute) }
	initialReleased, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_v1",
		Status:      ConfigStatusReleased,
		Summary:     "prod released v1",
	})
	if err != nil {
		t.Fatalf("CreateVersion target returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(2 * time.Minute) }
	draft, err := svc.CreateInheritanceDraft(context.Background(), CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
		Reason:            "seed prod candidate from staging",
		Actor:             "architect",
	})
	if err != nil {
		t.Fatalf("CreateInheritanceDraft returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(3 * time.Minute) }
	released, err := svc.ReleaseDraft(context.Background(), "router", "tenant-a", "prod", "tenant", "", draft.Version, "release-bot", "approve prod draft")
	if err != nil {
		t.Fatalf("ReleaseDraft returned error: %v", err)
	}

	if released.Version != draft.Version {
		t.Fatalf("expected released version %q, got %q", draft.Version, released.Version)
	}
	if released.Source != ConfigStatusReleased {
		t.Fatalf("expected released source marker %q, got %q", ConfigStatusReleased, released.Source)
	}
	if released.SourceEnvironment != draft.SourceEnvironment {
		t.Fatalf("expected source environment %q to be preserved, got %q", draft.SourceEnvironment, released.SourceEnvironment)
	}
	if released.SourceVersion != draft.SourceVersion {
		t.Fatalf("expected source version %q to be preserved, got %q", draft.SourceVersion, released.SourceVersion)
	}

	current, err := svc.CurrentReleased(context.Background(), "router", "tenant-a", "prod", "tenant", "")
	if err != nil {
		t.Fatalf("CurrentReleased returned error: %v", err)
	}
	if current.Version != draft.Version {
		t.Fatalf("expected current released version %q, got %q", draft.Version, current.Version)
	}
	if current.Version == initialReleased.Version {
		t.Fatalf("expected initial released version %q to be replaced", initialReleased.Version)
	}
}

func TestReleaseDraftPublishesAndAuditsSideEffects(t *testing.T) {
	recorder := audit.NewRecorder()
	publisher := &stubReleasePublisher{}

	svc := NewService().WithAuditRecorder(recorder).WithReleasePublisher(publisher)
	baseTime := time.Date(2026, 3, 24, 19, 35, 0, 0, time.UTC)
	svc.now = func() time.Time { return baseTime }

	_, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion source returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(time.Minute) }
	_, err = svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_v1",
		Status:      ConfigStatusReleased,
		Summary:     "prod released v1",
	})
	if err != nil {
		t.Fatalf("CreateVersion target returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(2 * time.Minute) }
	draft, err := svc.CreateInheritanceDraft(context.Background(), CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
		Reason:            "seed prod candidate from staging",
		Actor:             "architect",
	})
	if err != nil {
		t.Fatalf("CreateInheritanceDraft returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(3 * time.Minute) }
	released, err := svc.ReleaseDraft(context.Background(), "router", "tenant-a", "prod", "tenant", "", draft.Version, "release-bot", "approve prod draft")
	if err != nil {
		t.Fatalf("ReleaseDraft returned error: %v", err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Type != audit.ControlPlaneEventTypeRelease {
		t.Fatalf("expected audit event type %q, got %q", audit.ControlPlaneEventTypeRelease, events[0].Type)
	}
	if events[0].VersionID != released.Version {
		t.Fatalf("expected audit version %q, got %q", released.Version, events[0].VersionID)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 runtime event, got %d", len(publisher.events))
	}
	if publisher.events[0].Version != released.Version {
		t.Fatalf("expected published version %q, got %q", released.Version, publisher.events[0].Version)
	}
}

func TestPromoteReleasedCreatesNewReleasedVersionInTargetEnvironment(t *testing.T) {
	svc := NewService()
	baseTime := time.Date(2026, 3, 24, 20, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return baseTime }

	sourceReleased, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      ConfigStatusReleased,
		Summary:     "staging released",
		Config: map[string]string{
			"model": "gpt-4.1",
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion source returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(time.Minute) }
	previousTargetReleased, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_v1",
		Status:      ConfigStatusReleased,
		Summary:     "prod released v1",
		Config: map[string]string{
			"model": "gpt-4o-mini",
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion target returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(2 * time.Minute) }
	promoted, err := svc.PromoteReleased(context.Background(), "router", "tenant-a", "staging", "prod", "tenant", "", "release-bot", "promote staging to prod")
	if err != nil {
		t.Fatalf("PromoteReleased returned error: %v", err)
	}

	if promoted.Version == sourceReleased.Version {
		t.Fatalf("expected promoted released version to get a new version id")
	}
	if promoted.Source != ConfigStatusReleased {
		t.Fatalf("expected promoted source marker %q, got %q", ConfigStatusReleased, promoted.Source)
	}
	if promoted.Environment != "prod" {
		t.Fatalf("expected target environment prod, got %q", promoted.Environment)
	}
	if promoted.SourceEnvironment != sourceReleased.Environment {
		t.Fatalf("expected source environment %q, got %q", sourceReleased.Environment, promoted.SourceEnvironment)
	}
	if promoted.SourceVersion != sourceReleased.Version {
		t.Fatalf("expected source version %q, got %q", sourceReleased.Version, promoted.SourceVersion)
	}

	current, err := svc.CurrentReleased(context.Background(), "router", "tenant-a", "prod", "tenant", "")
	if err != nil {
		t.Fatalf("CurrentReleased returned error: %v", err)
	}
	if current.Version != promoted.Version {
		t.Fatalf("expected current released version %q, got %q", promoted.Version, current.Version)
	}
	if current.Version == previousTargetReleased.Version {
		t.Fatalf("expected previous target released version %q to be replaced", previousTargetReleased.Version)
	}
}

func TestPromoteReleasedPublishesAndAuditsSideEffects(t *testing.T) {
	recorder := audit.NewRecorder()
	publisher := &stubReleasePublisher{}

	svc := NewService().WithAuditRecorder(recorder).WithReleasePublisher(publisher)
	baseTime := time.Date(2026, 3, 24, 20, 10, 0, 0, time.UTC)
	svc.now = func() time.Time { return baseTime }

	sourceReleased, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion source returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(time.Minute) }
	_, err = svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_v1",
		Status:      ConfigStatusReleased,
		Summary:     "prod released v1",
	})
	if err != nil {
		t.Fatalf("CreateVersion target returned error: %v", err)
	}

	svc.now = func() time.Time { return baseTime.Add(2 * time.Minute) }
	promoted, err := svc.PromoteReleased(context.Background(), "router", "tenant-a", "staging", "prod", "tenant", "", "release-bot", "promote staging to prod")
	if err != nil {
		t.Fatalf("PromoteReleased returned error: %v", err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Type != audit.ControlPlaneEventTypeRelease {
		t.Fatalf("expected audit event type %q, got %q", audit.ControlPlaneEventTypeRelease, events[0].Type)
	}
	if events[0].VersionID != promoted.Version {
		t.Fatalf("expected audit version %q, got %q", promoted.Version, events[0].VersionID)
	}
	if events[0].Environment != "prod" {
		t.Fatalf("expected audit environment prod, got %q", events[0].Environment)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 runtime event, got %d", len(publisher.events))
	}
	if publisher.events[0].Version != promoted.Version {
		t.Fatalf("expected published version %q, got %q", promoted.Version, publisher.events[0].Version)
	}
	if publisher.events[0].SourceVersion != sourceReleased.Version {
		t.Fatalf("expected published source version %q, got %q", sourceReleased.Version, publisher.events[0].SourceVersion)
	}
}

func TestPromoteReleasedRequiresSourceReleased(t *testing.T) {
	svc := NewService()
	_, err := svc.PromoteReleased(context.Background(), "router", "tenant-a", "staging", "prod", "tenant", "", "release-bot", "promote staging to prod")
	if err == nil {
		t.Fatalf("expected error when source released config is missing")
	}
}

func TestReleaseDraftRequiresExistingDraft(t *testing.T) {
	svc := NewService()
	_, err := svc.ReleaseDraft(context.Background(), "router", "tenant-a", "prod", "tenant", "", "missing", "release-bot", "approve prod draft")
	if err == nil {
		t.Fatalf("expected error when draft version is missing")
	}
}

func TestCreateInheritanceDraftRequiresSourceReleased(t *testing.T) {
	svc := NewService()
	_, err := svc.CreateInheritanceDraft(context.Background(), CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
	})
	if err == nil {
		t.Fatalf("expected error when source released config is missing")
	}
}

func TestResolveConfigKeepsSingleEnvironmentPriority(t *testing.T) {
	svc := NewService()
	svc.now = func() time.Time { return time.Date(2026, 3, 24, 19, 0, 0, 0, time.UTC) }
	_, err := svc.CreateVersion(context.Background(), CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Status:      ConfigStatusReleased,
		Summary:     "prod released",
		Config: map[string]string{
			"model": "prod-model",
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	resolved, err := svc.ResolveConfig(
		context.Background(),
		"router",
		"tenant-a",
		"prod",
		"tenant",
		"",
		map[string]string{"timeout": "1s", "model": "project-model"},
		map[string]string{"timeout": "2s", "region": "tenant-region"},
		map[string]string{"timeout": "3s", "region": "template-region", "tier": "template-tier"},
		map[string]string{"timeout": "4s", "region": "default-region", "tier": "default-tier"},
	)
	if err != nil {
		t.Fatalf("ResolveConfig returned error: %v", err)
	}

	if got := resolved["model"]; got != "project-model" {
		t.Fatalf("expected project override model, got %q", got)
	}
	if got := resolved["timeout"]; got != "1s" {
		t.Fatalf("expected project override timeout, got %q", got)
	}
	if got := resolved["region"]; got != "tenant-region" {
		t.Fatalf("expected tenant override region, got %q", got)
	}
	if got := resolved["tier"]; got != "template-tier" {
		t.Fatalf("expected tenant template tier, got %q", got)
	}
}
