package controlplane

import (
	"errors"
	"testing"
)

type stubHook struct {
	name string
	err  error
}

func (s stubHook) Name() string    { return s.name }
func (s stubHook) Validate() error { return s.err }

func TestGateAndHookShape(t *testing.T) {
	results := RunValidationHooks([]ValidationHook{stubHook{name: "smoke", err: nil}})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "smoke" || !results[0].Passed {
		t.Fatalf("unexpected result: %+v", results[0])
	}
}

func TestPromotionStopsAtExplicitFailpoint(t *testing.T) {
	results := RunValidationHooks([]ValidationHook{stubHook{name: "hook1", err: errors.New("failed")}})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Fatalf("expected failed validation result: %+v", results[0])
	}
	if results[0].Error == "" {
		t.Fatalf("expected error message in result: %+v", results[0])
	}
}
