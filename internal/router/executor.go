package router

import (
	"context"
	"strings"
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

func ExecuteDecision[T any](
	ctx context.Context,
	decision Decision,
	cfg RetryConfig,
	pool KeyPool,
	fn ExecuteRouteFunc[T],
) (T, RetryResult, error) {
	targets := TargetsFromDecision(decision)
	var zero T
	var lastErr error
	combined := RetryResult{}

	for _, target := range targets {
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
			return resp, combined, nil
		}
		lastErr = err
		classified := ClassifyError(ctx, target.Provider, err)
		if classified.Class == ErrorClassAuth || classified.Class == ErrorClassBadRequest || classified.Class == ErrorClassClientCancelled {
			return zero, combined, err
		}
	}

	return zero, combined, lastErr
}
