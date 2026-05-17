package router

import (
	"context"
	"testing"
)

func TestTargetsFromDecision_IncludesPrimaryAndFallbacks(t *testing.T) {
	decision := Decision{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Channel:  "primary",
		FallbackChain: []FallbackRoute{
			{Provider: "anthropic", Model: "claude-sonnet", Reason: "second"},
			{Provider: "google", Model: "gemini", Reason: "third"},
		},
	}
	targets := TargetsFromDecision(decision)
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
	if targets[0].Provider != "openai" || targets[0].Model != "gpt-4o-mini" {
		t.Fatalf("expected primary openai/gpt-4o-mini, got %s/%s", targets[0].Provider, targets[0].Model)
	}
	if targets[1].Provider != "anthropic" || targets[1].Model != "claude-sonnet" {
		t.Fatalf("expected fallback anthropic/claude-sonnet, got %s/%s", targets[1].Provider, targets[1].Model)
	}
	if targets[2].Provider != "google" || targets[2].Model != "gemini" {
		t.Fatalf("expected fallback google/gemini, got %s/%s", targets[2].Provider, targets[2].Model)
	}
}

func TestTargetsFromDecision_DeduplicatesPrimaryFallback(t *testing.T) {
	decision := Decision{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		FallbackChain: []FallbackRoute{
			{Provider: "openai", Model: "gpt-4o-mini", Reason: "same-as-primary"},
			{Provider: "anthropic", Model: "claude-sonnet", Reason: "different"},
		},
	}
	targets := TargetsFromDecision(decision)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets after dedup, got %d", len(targets))
	}
	if targets[0].Provider != "openai" {
		t.Fatalf("expected first target openai, got %s", targets[0].Provider)
	}
	if targets[1].Provider != "anthropic" {
		t.Fatalf("expected second target anthropic, got %s", targets[1].Provider)
	}
}

func TestExecuteDecision_PrimarySuccessDoesNotFallback(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 1}
	decision := Decision{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		FallbackChain: []FallbackRoute{
			{Provider: "anthropic", Model: "claude-sonnet"},
		},
	}
	callCount := 0
	result, trace, err := ExecuteDecision(ctx, decision, cfg, nil, func(ctx context.Context, target RouteTarget, key ProviderKey) (string, error) {
		callCount++
		return "primary-ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "primary-ok" {
		t.Fatalf("expected primary-ok, got %s", result)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
	if trace.FinalProvider != "openai" {
		t.Fatalf("expected final provider openai, got %s", trace.FinalProvider)
	}
}

func TestExecuteDecision_PrimaryRetryExhaustedFallsBack(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 2, BaseDelay: 0}
	decision := Decision{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		FallbackChain: []FallbackRoute{
			{Provider: "anthropic", Model: "claude-sonnet"},
		},
	}
	callCount := 0
	result, trace, err := ExecuteDecision(ctx, decision, cfg, nil, func(ctx context.Context, target RouteTarget, key ProviderKey) (string, error) {
		callCount++
		if target.Provider == "openai" {
			return "", newFakeHTTPError(503, "unavailable")
		}
		return "fallback-ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fallback-ok" {
		t.Fatalf("expected fallback-ok, got %s", result)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls (2 primary + 1 fallback), got %d", callCount)
	}
	if trace.FinalProvider != "anthropic" {
		t.Fatalf("expected final provider anthropic, got %s", trace.FinalProvider)
	}
	if len(trace.Attempts) != 3 {
		t.Fatalf("expected 3 attempt traces, got %d", len(trace.Attempts))
	}
}

func TestExecuteDecision_BadRequestDoesNotFallback(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 1}
	decision := Decision{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		FallbackChain: []FallbackRoute{
			{Provider: "anthropic", Model: "claude-sonnet"},
		},
	}
	callCount := 0
	_, _, err := ExecuteDecision(ctx, decision, cfg, nil, func(ctx context.Context, target RouteTarget, key ProviderKey) (string, error) {
		callCount++
		return "", newFakeHTTPError(400, "bad request")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call (no fallback on bad request), got %d", callCount)
	}
}

func TestExecuteDecision_AllTargetsFailReturnsTrace(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 1}
	decision := Decision{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		FallbackChain: []FallbackRoute{
			{Provider: "anthropic", Model: "claude-sonnet"},
		},
	}
	callCount := 0
	_, trace, err := ExecuteDecision(ctx, decision, cfg, nil, func(ctx context.Context, target RouteTarget, key ProviderKey) (string, error) {
		callCount++
		return "", newFakeHTTPError(503, "unavailable")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
	if len(trace.Attempts) != 2 {
		t.Fatalf("expected 2 attempt traces, got %d", len(trace.Attempts))
	}
	if trace.FinalProvider != "anthropic" {
		t.Fatalf("expected final provider anthropic (last tried), got %s", trace.FinalProvider)
	}
}
