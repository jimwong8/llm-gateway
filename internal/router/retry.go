package router

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func redactKeyID(id string) string {
	if id == "" {
		return ""
	}
	if len(id) <= 4 {
		return "****"
	}
	return "****" + id[len(id)-1:]
}

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

type AttemptTrace struct {
	Attempt    int           `json:"attempt"`
	Provider   string        `json:"provider"`
	Model      string        `json:"model,omitempty"`
	Channel    string        `json:"channel,omitempty"`
	KeyID      string        `json:"key_id,omitempty"`
	ErrorClass ErrorClass    `json:"error_class"`
	Error      string        `json:"error,omitempty"`
	Latency    time.Duration `json:"latency"`
}

type RetryResult struct {
	Attempts      []AttemptTrace `json:"attempts"`
	FinalProvider string         `json:"final_provider"`
	FinalModel    string         `json:"final_model,omitempty"`
	FinalKeyID    string         `json:"final_key_id,omitempty"`
}

type ExecuteWithKeyFunc[T any] func(ctx context.Context, key ProviderKey) (T, error)

// extractRetryAfter attempts to extract the Retry-After header value from an error.
// Returns the duration to wait, or 0 if not available.
func extractRetryAfter(err error) time.Duration {
	if err == nil {
		return 0
	}
	// Try to extract HTTP response headers from the error
	type headerAccessor interface {
		HTTPResponseHeaders() map[string]string
	}
	if ha, ok := err.(headerAccessor); ok {
		headers := ha.HTTPResponseHeaders()
		for k, v := range headers {
			if http.CanonicalHeaderKey(k) == "Retry-After" {
				if secs, parseErr := strconv.Atoi(v); parseErr == nil {
					return time.Duration(secs+2) * time.Second // +2s buffer
				}
			}
		}
	}
	return 0
}

// isRateLimitError checks if the error is a 429 rate limit error.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	type statusCoder interface {
		HTTPStatusCode() int
	}
	if sc, ok := err.(statusCoder); ok {
		return sc.HTTPStatusCode() == http.StatusTooManyRequests
	}
	lower := fmt.Sprintf("%v", err)
	return containsLower(lower, "rate limit") || containsLower(lower, "too many requests")
}

func containsLower(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(lower(s), substr)
}

func lower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		result[i] = c
	}
	return string(result)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func ExecuteWithRetries[T any](
	ctx context.Context,
	cfg RetryConfig,
	pool KeyPool,
	provider string,
	channel string,
	fn ExecuteWithKeyFunc[T],
) (T, RetryResult, error) {
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	baseDelay := cfg.BaseDelay
	if baseDelay < 0 {
		baseDelay = 0
	}
	maxDelay := cfg.MaxDelay
	if maxDelay < baseDelay {
		maxDelay = baseDelay
	}

	var used map[string]bool
	if pool != nil {
		used = map[string]bool{}
	}

	var zero T
	var lastErr error
	result := RetryResult{}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var key ProviderKey
		if pool != nil {
			var err error
			key, err = pool.Next(ctx, provider, channel, used)
			if err != nil {
				return zero, result, err
			}
		} else {
			key = ProviderKey{ID: fmt.Sprintf("%s-default", provider), Provider: provider}
		}

		started := time.Now()
		resp, err := fn(ctx, key)
		latency := time.Since(started)

		trace := AttemptTrace{
			Attempt:  attempt,
			Provider: provider,
			Channel:  channel,
			KeyID:    redactKeyID(key.ID),
			Latency:  latency,
		}

		if err == nil {
			trace.ErrorClass = ErrorClassNone
			result.Attempts = append(result.Attempts, trace)
			result.FinalProvider = provider
			result.FinalKeyID = key.ID
			return resp, result, nil
		}

		lastErr = err
		classified := ClassifyError(ctx, provider, err)
		trace.ErrorClass = classified.Class
		trace.Error = err.Error()
		result.Attempts = append(result.Attempts, trace)

		if classified.Class == ErrorClassClientCancelled || ctx.Err() != nil {
			return zero, result, err
		}
		if classified.Class == ErrorClassAuth || classified.Class == ErrorClassBadRequest {
			return zero, result, err
		}
		if classified.RotateKey && pool != nil {
			used[key.ID] = true
		}

		if attempt < maxAttempts {
			// P0-1: Use Retry-After header for rate limit errors
			var delay time.Duration
			if isRateLimitError(err) {
				retryAfter := extractRetryAfter(err)
				if retryAfter > 0 {
					delay = retryAfter
				} else {
					delay = min(time.Duration(attempt)*baseDelay, maxDelay)
				}
			} else {
				delay = min(time.Duration(attempt)*baseDelay, maxDelay)
			}
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return zero, result, ctx.Err()
				}
			}
		}
	}

	return zero, result, lastErr
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
