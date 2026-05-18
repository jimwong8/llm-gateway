package router

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExecuteWithRetries_SuccessFirstAttempt(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 3}
	pool := &StaticKeyPool{Keys: []ProviderKey{{ID: "k1", Provider: "openai", Weight: 1, Enabled: true}}}
	callCount := 0
	result, trace, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		callCount++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok, got %s", result)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
	if len(trace.Attempts) != 1 {
		t.Fatalf("expected 1 attempt trace, got %d", len(trace.Attempts))
	}
	if trace.FinalKeyID != "k1" {
		t.Fatalf("expected final key k1, got %s", trace.FinalKeyID)
	}
}

func TestExecuteWithRetries_RateLimitRotatesKey(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 3}
	pool := &StaticKeyPool{Keys: []ProviderKey{
		{ID: "ka", Provider: "openai", Weight: 1, Enabled: true},
		{ID: "kb", Provider: "openai", Weight: 1, Enabled: true},
	}}
	callCount := 0
	result, trace, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		callCount++
		if key.ID == "ka" {
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
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
	if len(trace.Attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(trace.Attempts))
	}
	if trace.Attempts[0].ErrorClass != ErrorClassRateLimit {
		t.Fatalf("expected rate_limit class, got %s", trace.Attempts[0].ErrorClass)
	}
	if trace.FinalKeyID != "kb" {
		t.Fatalf("expected final key kb, got %s", trace.FinalKeyID)
	}
}

func TestExecuteWithRetries_NetworkErrorRetries(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 0}
	pool := &StaticKeyPool{Keys: []ProviderKey{{ID: "k1", Provider: "openai", Weight: 1, Enabled: true}}}
	callCount := 0
	result, _, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		callCount++
		if callCount < 2 {
			return "", newFakeHTTPError(503, "unavailable")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok, got %s", result)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
}

func TestExecuteWithRetries_AuthErrorStopsImmediately(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 3}
	pool := &StaticKeyPool{Keys: []ProviderKey{{ID: "k1", Provider: "openai", Weight: 1, Enabled: true}}}
	callCount := 0
	_, _, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		callCount++
		return "", newFakeHTTPError(401, "invalid key")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
}

func TestExecuteWithRetries_ContextCancelledStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 0}
	pool := &StaticKeyPool{Keys: []ProviderKey{{ID: "k1", Provider: "openai", Weight: 1, Enabled: true}}}
	callCount := 0
	_, _, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		callCount++
		if callCount == 1 {
			cancel()
		}
		return "", newFakeHTTPError(503, "unavailable")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call after cancel, got %d", callCount)
	}
}

func TestExecuteWithRetries_AllKeysExhaustedResets(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 4, BaseDelay: 0}
	pool := &StaticKeyPool{Keys: []ProviderKey{
		{ID: "ka", Provider: "openai", Weight: 1, Enabled: true},
		{ID: "kb", Provider: "openai", Weight: 1, Enabled: true},
	}}
	callCount := 0
	result, trace, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		callCount++
		if callCount < 4 {
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
	if callCount != 4 {
		t.Fatalf("expected 4 calls, got %d", callCount)
	}
	if len(trace.Attempts) != 4 {
		t.Fatalf("expected 4 attempts, got %d", len(trace.Attempts))
	}
}

func TestExecuteWithRetries_NilPoolUsesSyntheticKey(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 1}
	callCount := 0
	result, _, err := ExecuteWithRetries(ctx, cfg, nil, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		callCount++
		if key.ID == "" {
			t.Fatal("expected non-empty key ID")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok, got %s", result)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
}

func TestExecuteWithRetries_ZeroMaxAttemptsBecomesOne(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 0}
	pool := &StaticKeyPool{Keys: []ProviderKey{{ID: "k1", Provider: "openai", Weight: 1, Enabled: true}}}
	callCount := 0
	_, _, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		callCount++
		return "", errors.New("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
}

func TestExecuteWithRetries_RecordsLatency(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 1}
	pool := &StaticKeyPool{Keys: []ProviderKey{{ID: "k1", Provider: "openai", Weight: 1, Enabled: true}}}
	_, trace, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		time.Sleep(5 * time.Millisecond)
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trace.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(trace.Attempts))
	}
	if trace.Attempts[0].Latency <= 0 {
		t.Fatal("expected positive latency")
	}
}
