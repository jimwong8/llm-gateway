package router

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRetryResult_JSONSerialization(t *testing.T) {
	result := RetryResult{
		Attempts: []AttemptTrace{
			{Attempt: 1, Provider: "openai", Model: "gpt-4o-mini", ErrorClass: ErrorClassRateLimit, Error: "rate limited", Latency: 1000000},
			{Attempt: 2, Provider: "openai", Model: "gpt-4o-mini", ErrorClass: ErrorClassNone, Latency: 500000},
		},
		FinalProvider: "openai",
		FinalModel:    "gpt-4o-mini",
		FinalKeyID:    "****abcd",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	var parsed RetryResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	if parsed.FinalProvider != "openai" {
		t.Fatalf("expected final provider openai, got %s", parsed.FinalProvider)
	}
	if len(parsed.Attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(parsed.Attempts))
	}
	if parsed.Attempts[0].ErrorClass != ErrorClassRateLimit {
		t.Fatalf("expected rate_limit class, got %s", parsed.Attempts[0].ErrorClass)
	}
}

func TestRedactKeyID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"ab", "****"},
		{"abcd", "****"},
		{"abcde", "****e"},
		{"sk-1234567890abcdef", "****f"},
	}
	for _, tt := range tests {
		got := redactKeyID(tt.input)
		if got != tt.expected {
			t.Fatalf("redactKeyID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExecuteWithRetries_RedactsKeyIDInTrace(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{MaxAttempts: 1}
	pool := &StaticKeyPool{Keys: []ProviderKey{
		{ID: "sk-1234567890abcdef", Provider: "openai", Weight: 1, Enabled: true},
	}}

	_, trace, err := ExecuteWithRetries(ctx, cfg, pool, "openai", "", func(ctx context.Context, key ProviderKey) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trace.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(trace.Attempts))
	}
	if trace.Attempts[0].KeyID != "****f" {
		t.Fatalf("expected redacted key ID ****f, got %s", trace.Attempts[0].KeyID)
	}
	if strings.Contains(trace.Attempts[0].KeyID, "1234567890") {
		t.Fatal("key ID should not contain plaintext secret")
	}
}
