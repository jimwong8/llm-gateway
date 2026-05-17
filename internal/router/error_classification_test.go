package router

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyError(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name          string
		err           error
		wantClass     ErrorClass
		wantRetryable bool
		wantRotateKey bool
	}{
		{name: "nil", err: nil, wantClass: ErrorClassNone},
		{name: "rate limit status", err: ProviderHTTPError{StatusCode: 429, Message: "too many requests"}, wantClass: ErrorClassRateLimit, wantRetryable: true, wantRotateKey: true},
		{name: "rate limit text", err: errors.New("provider rate limit exceeded"), wantClass: ErrorClassRateLimit, wantRetryable: true, wantRotateKey: true},
		{name: "bad gateway", err: ProviderHTTPError{StatusCode: 502, Message: "bad gateway"}, wantClass: ErrorClassRetryableUpstream, wantRetryable: true},
		{name: "service unavailable", err: ProviderHTTPError{StatusCode: 503, Message: "unavailable"}, wantClass: ErrorClassRetryableUpstream, wantRetryable: true},
		{name: "unauthorized", err: ProviderHTTPError{StatusCode: 401, Message: "invalid api key"}, wantClass: ErrorClassAuth},
		{name: "bad request", err: ProviderHTTPError{StatusCode: 400, Message: "model required"}, wantClass: ErrorClassBadRequest},
		{name: "unknown", err: errors.New("boom"), wantClass: ErrorClassUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(ctx, "openai", tt.err)
			if got.Class != tt.wantClass {
				t.Fatalf("class = %s, want %s", got.Class, tt.wantClass)
			}
			if got.Retryable != tt.wantRetryable {
				t.Fatalf("retryable = %v, want %v", got.Retryable, tt.wantRetryable)
			}
			if got.RotateKey != tt.wantRotateKey {
				t.Fatalf("rotateKey = %v, want %v", got.RotateKey, tt.wantRotateKey)
			}
		})
	}
}

func TestClassifyError_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := ClassifyError(ctx, "openai", context.Canceled)
	if got.Class != ErrorClassClientCancelled {
		t.Fatalf("class = %s, want %s", got.Class, ErrorClassClientCancelled)
	}
	if got.Retryable {
		t.Fatalf("client cancellation must not retry")
	}
}
