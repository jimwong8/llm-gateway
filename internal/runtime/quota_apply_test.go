package runtime

import (
	"context"
	"errors"
	"testing"

	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/quota"
)

type stubQuotaReleasedVersionResolver struct {
	version controlplane.ConfigVersion
	err     error
	calls   int
}

func (s *stubQuotaReleasedVersionResolver) GetVersion(_ context.Context, module, tenantID, environment, scope, projectID, versionID string) (controlplane.ConfigVersion, error) {
	s.calls++
	if s.err != nil {
		return controlplane.ConfigVersion{}, s.err
	}
	return s.version, nil
}

func TestBuildQuotaPayloadDrivenApplyWithResolver_UsesResolverPayloadFirst(t *testing.T) {
	limiter := quota.New("127.0.0.1:6379", 10)
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_q1",
		ModulePayloads: map[string]any{
			"quota": map[string]any{"rpm": 99},
		},
	}})

	resolver := &stubQuotaReleasedVersionResolver{version: controlplane.ConfigVersion{
		Module:      "quota",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     "cfg_q1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"rpm": "88",
		},
	}}

	apply := BuildQuotaPayloadDrivenApplyWithResolver(limiter, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "quota", PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_q1"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if got := limiter.RPM(); got != 88 {
		t.Fatalf("expected rpm from resolver payload 88, got %d", got)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
}

func TestBuildQuotaPayloadDrivenApplyWithResolver_FallbacksToPublisherWhenResolverUnavailable(t *testing.T) {
	limiter := quota.New("127.0.0.1:6379", 10)
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_q2",
		ModulePayloads: map[string]any{
			"quota": map[string]any{"rpm": 77},
		},
	}})
	resolver := &stubQuotaReleasedVersionResolver{err: errors.New("resolver unavailable")}

	apply := BuildQuotaPayloadDrivenApplyWithResolver(limiter, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "quota", PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_q2"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if got := limiter.RPM(); got != 77 {
		t.Fatalf("expected rpm from publisher fallback 77, got %d", got)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
}

func TestBuildQuotaPayloadDrivenApplyWithResolver_InvalidQuotaPayloadReturnsErrorAndNoMutation(t *testing.T) {
	limiter := quota.New("127.0.0.1:6379", 15)
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_bad",
		ModulePayloads: map[string]any{
			"quota": "invalid-type",
		},
	}})

	apply := BuildQuotaPayloadDrivenApplyWithResolver(limiter, publisher, nil)
	err := apply(ConfigChangeEvent{Module: "quota", PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_bad"})
	if err == nil {
		t.Fatalf("expected error for invalid quota payload")
	}
	if got := limiter.RPM(); got != 15 {
		t.Fatalf("expected limiter rpm unchanged on invalid payload, got %d", got)
	}
}

func TestBuildQuotaPayloadDrivenApplyWithResolver_ResolverErrorNoPayloadNoMutation(t *testing.T) {
	limiter := quota.New("127.0.0.1:6379", 20)
	publisher := NewPublisher()
	resolver := &stubQuotaReleasedVersionResolver{err: errors.New("resolver unavailable")}

	apply := BuildQuotaPayloadDrivenApplyWithResolver(limiter, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "quota", PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_miss"}); err != nil {
		t.Fatalf("expected nil error when resolver fallback unavailable, got %v", err)
	}
	if got := limiter.RPM(); got != 20 {
		t.Fatalf("expected limiter rpm unchanged when no payload and resolver error, got %d", got)
	}
}

func TestBuildQuotaPayloadDrivenApplyWithResolver_LiveLimiterMutationByReleasedPayload(t *testing.T) {
	limiter := quota.New("127.0.0.1:6379", 30)
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_live_1",
		ModulePayloads: map[string]any{
			"quota": map[string]any{"rpm": 42},
		},
	}})

	apply := BuildQuotaPayloadDrivenApplyWithResolver(limiter, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "quota", PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_live_1"}); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if got := limiter.RPM(); got != 42 {
		t.Fatalf("expected live limiter rpm to change to 42, got %d", got)
	}
}

func TestBuildQuotaPayloadDrivenApply_BackwardCompatibilityAlias(t *testing.T) {
	limiter := quota.New("127.0.0.1:6379", 10)
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_alias",
		ModulePayloads: map[string]any{
			"quota": map[string]any{"rpm": 55},
		},
	}})

	apply := BuildQuotaPayloadDrivenApply(limiter, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "quota", PayloadRef: "released://quota/tenant-a/prod/tenant/project-a/cfg_alias"}); err != nil {
		t.Fatalf("alias apply returned error: %v", err)
	}
	if got := limiter.RPM(); got != 55 {
		t.Fatalf("expected alias apply to update rpm to 55, got %d", got)
	}
}
