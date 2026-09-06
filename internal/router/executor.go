package router

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type RouteTarget struct {
	Provider string
	Model    string
	Channel  string
	Reason   string
}

type ExecuteRouteFunc[T any] func(ctx context.Context, target RouteTarget, key ProviderKey) (T, error)

func TargetsFromDecision(decision Decision) []RouteTarget {
	seen := map[string]bool{}
	var targets []RouteTarget
	add := func(provider, model, channel, reason string) {
		key := strings.ToLower(provider) + "/" + strings.ToLower(model)
		if seen[key] {
			return
		}
		seen[key] = true
		targets = append(targets, RouteTarget{Provider: provider, Model: model, Channel: channel, Reason: reason})
	}
	add(decision.Provider, decision.Model, decision.Channel, "primary")
	for _, fb := range decision.FallbackChain {
		add(fb.Provider, fb.Model, "", fb.Reason)
	}
	return targets
}

// ExecuteDecisionWithCooldown wraps ExecuteDecision with per-model cooldown and circuit breaking.
func ExecuteDecisionWithCooldown[T any](
	ctx context.Context,
	decision Decision,
	cfg RetryConfig,
	pool KeyPool,
	fn ExecuteRouteFunc[T],
	modelCooldown *ModelCooldown,
	circuitBreaker *ModelCircuitBreaker,
) (T, RetryResult, error) {
	targets := TargetsFromDecision(decision)
	var zero T
	var lastErr error
	combined := RetryResult{}

	for _, target := range targets {
		// P0-2: Skip models on cooldown
		if modelCooldown.IsCooling(target.Model) {
			lastErr = fmt.Errorf("model %s on cooldown (remaining %s)", target.Model, modelCooldown.Remaining(target.Model))
			continue
		}
		// P2-2: Skip models with open circuit
		if circuitBreaker.IsCircuitOpen(target.Model) {
			lastErr = fmt.Errorf("model %s circuit open", target.Model)
			continue
		}

		keyProvider := target.Provider
		if keyProvider == "" {
			keyProvider = decision.Provider
		}
		resp, trace, err := ExecuteWithRetries(ctx, cfg, pool, keyProvider, target.Channel, func(ctx context.Context, key ProviderKey) (T, error) {
			return fn(ctx, target, key)
		})
		combined.Attempts = append(combined.Attempts, trace.Attempts...)
		combined.FinalProvider = target.Provider
		combined.FinalModel = target.Model
		combined.FinalKeyID = trace.FinalKeyID
		if err == nil {
			// Success: clear cooldown and circuit breaker
			modelCooldown.Clear(target.Model)
			circuitBreaker.RecordSuccess(target.Model)
			return resp, combined, nil
		}
		lastErr = err
		classified := ClassifyError(ctx, target.Provider, err)
		if classified.Class == ErrorClassAuth || classified.Class == ErrorClassBadRequest || classified.Class == ErrorClassClientCancelled {
			return zero, combined, err
		}
		// P0-2: Set cooldown on 429
		if classified.Class == ErrorClassRateLimit {
			modelCooldown.SetCooldown(target.Model, 7200*time.Second)
			circuitBreaker.RecordFailure(target.Model)
		}
	}

	return zero, combined, lastErr
}

func ExecuteDecision[T any](
	ctx context.Context,
	decision Decision,
	cfg RetryConfig,
	pool KeyPool,
	fn ExecuteRouteFunc[T],
) (T, RetryResult, error) {
	// Use the new cooldown-aware executor with default instances
	modelCooldown := NewModelCooldown()
	circuitBreaker := NewModelCircuitBreaker()
	return ExecuteDecisionWithCooldown(ctx, decision, cfg, pool, fn, modelCooldown, circuitBreaker)
}
