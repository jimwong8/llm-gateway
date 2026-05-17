package router

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	name       string
	statusCode int
	body       string
}

func (f *fakeProvider) ChatCompletion(ctx context.Context) error {
	if f.statusCode >= 400 {
		return newFakeHTTPError(f.statusCode, f.body)
	}
	return nil
}

type fakeHTTPError struct {
	code    int
	message string
}

func (e fakeHTTPError) Error() string    { return e.message }
func (e fakeHTTPError) HTTPStatusCode() int { return e.code }

func newFakeHTTPError(code int, msg string) fakeHTTPError {
	return fakeHTTPError{code: code, message: msg}
}

func TestClassifyError_WithHTTPStatusInterface(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name          string
		err           error
		wantClass     ErrorClass
		wantRetryable bool
		wantRotateKey bool
	}{
		{name: "429", err: newFakeHTTPError(429, "rate limited"), wantClass: ErrorClassRateLimit, wantRetryable: true, wantRotateKey: true},
		{name: "401", err: newFakeHTTPError(401, "invalid key"), wantClass: ErrorClassAuth},
		{name: "400", err: newFakeHTTPError(400, "bad request"), wantClass: ErrorClassBadRequest},
		{name: "503", err: newFakeHTTPError(503, "unavailable"), wantClass: ErrorClassRetryableUpstream, wantRetryable: true},
		{name: "502", err: newFakeHTTPError(502, "bad gateway"), wantClass: ErrorClassRetryableUpstream, wantRetryable: true},
		{name: "unknown", err: newFakeHTTPError(418, "teapot"), wantClass: ErrorClassUnknown},
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

func TestClassifyError_WrappedHTTPStatusError(t *testing.T) {
	ctx := context.Background()
	wrapped := errors.New("outer: " + newFakeHTTPError(418, "teapot").Error())
	got := ClassifyError(ctx, "openai", wrapped)
	if got.Class != ErrorClassUnknown {
		t.Fatalf("expected unknown for non-interface error, got %s", got.Class)
	}
}
