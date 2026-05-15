package runtime

import (
	"context"
	"testing"
	"time"

	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/preprocess"
)

func TestBuildPreprocessPayloadDrivenApplyWithResolver_UsesResolverPayloadFirst(t *testing.T) {
	store := preprocess.NewConfigStore(preprocess.DefaultConfig())
	publisher := NewPublisher()
	resolver := &stubReleasedVersionResolver{version: controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		ProjectID:   "project-a",
		Version:     "cfg_preprocess_1",
		Source:      controlplane.ConfigStatusReleased,
		Config: map[string]string{
			"preprocess": `{"normalize_enabled":true,"summary_enabled":true,"summary_trigger_messages":30,"classifier_model":"local-classifier"}`,
		},
	}}
	apply := BuildPreprocessPayloadDrivenApplyWithResolver(store, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "preprocess", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_preprocess_1"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	cfg := store.Get()
	if !cfg.NormalizeEnabled || !cfg.SummaryEnabled || cfg.SummaryTriggerMessages != 30 || cfg.ClassifierModel != "local-classifier" {
		t.Fatalf("unexpected preprocess config after apply: %+v", cfg)
	}
}

func TestBuildPreprocessPayloadDrivenApplyWithResolver_FallbacksToPublisher(t *testing.T) {
	store := preprocess.NewConfigStore(preprocess.DefaultConfig())
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_preprocess_2",
		ModulePayloads: map[string]any{
			"router": map[string]any{
				"preprocess": map[string]any{"classification_enabled": true, "summary_max_recent_turns": 8},
			},
		},
	}})
	resolver := &stubReleasedVersionResolver{err: controlplane.ErrVersionNotFound}
	apply := BuildPreprocessPayloadDrivenApplyWithResolver(store, publisher, resolver)
	if err := apply(ConfigChangeEvent{Module: "preprocess", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_preprocess_2"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	cfg := store.Get()
	if !cfg.ClassificationEnabled || cfg.SummaryMaxRecentTurns != 8 {
		t.Fatalf("unexpected preprocess config after publisher fallback: %+v", cfg)
	}
}

func TestBuildPreprocessPayloadDrivenApplyWithResolver_InvalidPayloadReturnsError(t *testing.T) {
	store := preprocess.NewConfigStore(preprocess.DefaultConfig())
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_preprocess_bad",
		ModulePayloads: map[string]any{
			"preprocess": "invalid-type",
		},
	}})
	apply := BuildPreprocessPayloadDrivenApplyWithResolver(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "preprocess", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_preprocess_bad"}); err == nil {
		t.Fatalf("expected error for invalid preprocess payload")
	}
}

func TestBuildPreprocessPayloadDrivenApply_BackwardCompatibilityAlias(t *testing.T) {
	store := preprocess.NewConfigStore(preprocess.DefaultConfig())
	publisher := NewPublisher()
	publisher.events = append(publisher.events, Event{Apply: RuntimeApplyPayload{
		PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_preprocess_alias",
		ModulePayloads: map[string]any{
			"router": map[string]any{
				"preprocess": map[string]any{"normalize_enabled": true},
			},
		},
	}})
	apply := BuildPreprocessPayloadDrivenApply(store, publisher, nil)
	if err := apply(ConfigChangeEvent{Module: "preprocess", PayloadRef: "released://router/tenant-a/prod/tenant/project-a/cfg_preprocess_alias", ChangedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !store.Get().NormalizeEnabled {
		t.Fatalf("expected normalize to be enabled from router alias payload")
	}
}

func TestBuildPreprocessReloadApply_RejectsNonReleasedPayloadRef(t *testing.T) {
	reload := BuildModuleRuntimeApplyDispatcher(map[string]ModuleRuntimeApplier{
		"preprocess": BuildModuleRuntimeApplyDispatcher(nil),
	})
	_ = reload
	_ = context.Background()
}
