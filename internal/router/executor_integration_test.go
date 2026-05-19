package router

import (
	"context"
	"testing"
)

// TestExecuteDecision_WithRealProviderInterface verifies that the v2 executor
// works with the actual providers.Provider interface signature via adapter.
func TestExecuteDecision_WithRealProviderInterface(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 2, BaseDelay: 0}

	// Create a fake provider that fails primary, succeeds on fallback
	callCount := 0
	fakeProvider := &fakeCallProvider{
		fn: func(model string) error {
			callCount++
			if model == "primary-model" {
				return newFakeHTTPError(503, "unavailable")
			}
			return nil
		},
	}

	primaryDecision := Decision{
		Provider: "test-provider",
		Model:    "primary-model",
		FallbackChain: []FallbackRoute{
			{Provider: "test-provider", Model: "fallback-model"},
		},
	}

	result, trace, err := ExecuteDecision(ctx, primaryDecision, cfg, nil, func(ctx context.Context, target RouteTarget, key ProviderKey) (string, error) {
		if callErr := fakeProvider.fn(target.Model); callErr != nil {
			return "", callErr
		}
		return "success:" + target.Model, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success:fallback-model" {
		t.Fatalf("expected success:fallback-model, got %s", result)
	}
	if trace.FinalProvider != "test-provider" {
		t.Fatalf("expected final provider test-provider, got %s", trace.FinalProvider)
	}
	if len(trace.Attempts) < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", len(trace.Attempts))
	}
}

// TestExecuteDecision_RateLimitWithKeyRotation verifies key rotation works
// end-to-end with the v2 executor.
func TestExecuteDecision_RateLimitWithKeyRotation(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 0}
	pool := &StaticKeyPool{Keys: []ProviderKey{
		{ID: "k1", Provider: "openai", Weight: 1, Enabled: true},
		{ID: "k2", Provider: "openai", Weight: 1, Enabled: true},
	}}

	callCount := 0
	rateLimitDecision := Decision{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	}
	result, trace, err := ExecuteDecision(ctx, rateLimitDecision, cfg, pool, func(ctx context.Context, target RouteTarget, key ProviderKey) (string, error) {
		callCount++
		if key.ID == "k1" {
			return "", newFakeHTTPError(429, "rate limited")
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok, got %s", result)
	}
	if trace.FinalKeyID != "k2" {
		t.Fatalf("expected final key k2, got %s", trace.FinalKeyID)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
}

// fakeCallProvider is a test helper that records calls.
type fakeCallProvider struct {
	fn func(model string) error
}

// Verify that our error types work with the providers package error classification.
func TestClassifyError_ProviderUpstreamError(t *testing.T) {
	ctx := context.Background()

	// Simulate the error type that providers package now returns
	err := newFakeHTTPError(429, "rate limited")
	got := ClassifyError(ctx, "openai", err)
	if got.Class != ErrorClassRateLimit {
		t.Fatalf("expected rate_limit, got %s", got.Class)
	}
	if !got.RotateKey {
		t.Fatal("expected rotate key for rate limit")
	}

	err503 := newFakeHTTPError(503, "service unavailable")
	got503 := ClassifyError(ctx, "openai", err503)
	if got503.Class != ErrorClassRetryableUpstream {
		t.Fatalf("expected retryable_upstream, got %s", got503.Class)
	}

	err401 := newFakeHTTPError(401, "invalid key")
	got401 := ClassifyError(ctx, "openai", err401)
	if got401.Class != ErrorClassAuth {
		t.Fatalf("expected auth, got %s", got401.Class)
	}
}
