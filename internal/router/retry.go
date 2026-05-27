package router

import (
	"context"
	"fmt"
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
			delay := baseDelay
			if maxDelay > 0 {
				d := time.Duration(attempt) * baseDelay
				if d > maxDelay {
					d = maxDelay
				}
				delay = d
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
