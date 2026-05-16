package providers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"llm-gateway/gateway/internal/config"
)

type ProviderHealth struct {
	Name         string
	Type         string
	Enabled      bool
	Status       string
	FailureCount int
	LastError    string
	LatencyMS    int64
	CheckedAt    time.Time
	OpenedUntil  time.Time
}

type providerState struct {
	ProviderHealth
}

type Registry struct {
	providers        map[string]Provider
	fallback         Provider
	states           map[string]*providerState
	maxRetries       int
	failureThreshold int
	openInterval     time.Duration
	mu               sync.RWMutex
}

func NewRegistry(cfg config.Config, fallback Provider, items ...Provider) *Registry {
	out := &Registry{
		providers:        map[string]Provider{},
		states:           map[string]*providerState{},
		fallback:         fallback,
		maxRetries:       max(0, cfg.ProviderMaxRetries),
		failureThreshold: max(1, cfg.ProviderFailureThreshold),
		openInterval:     time.Duration(max(1, cfg.ProviderOpenSeconds)) * time.Second,
	}
	for _, item := range items {
		out.add(item)
	}
	out.add(fallback)
	return out
}

func (r *Registry) add(item Provider) {
	if item == nil {
		return
	}
	name := strings.ToLower(strings.TrimSpace(item.Name()))
	if name == "" {
		return
	}
	r.providers[name] = item
	if _, ok := r.states[name]; !ok {
		r.states[name] = &providerState{ProviderHealth: ProviderHealth{Name: item.Name(), Type: providerType(item.Name()), Enabled: true, Status: "ok"}}
	}
}

func (r *Registry) Resolve(name string) Provider {
	if provider, ok := r.providers[strings.ToLower(strings.TrimSpace(name))]; ok {
		return provider
	}
	return r.fallback
}

func (r *Registry) ChatCompletion(ctx context.Context, providerName string, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	provider := r.Resolve(providerName)
	if provider == nil {
		return ChatCompletionResponse{}, fmt.Errorf("provider not found: %s", providerName)
	}
	name := strings.ToLower(strings.TrimSpace(provider.Name()))
	if err := r.beforeCall(name); err != nil {
		return ChatCompletionResponse{}, err
	}

	attempts := r.maxRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		started := time.Now()
		resp, err := provider.ChatCompletion(ctx, req)
		latency := time.Since(started)
		if err == nil {
			r.recordSuccess(name, latency)
			return resp, nil
		}
		lastErr = err
		r.recordFailure(name, latency, err)
		if attempt == attempts || ctx.Err() != nil || !shouldRetry(name) {
			break
		}
		time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		if err := r.beforeCall(name); err != nil {
			return ChatCompletionResponse{}, err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("provider request failed: %s", name)
	}
	return ChatCompletionResponse{}, lastErr
}

func (r *Registry) beforeCall(name string) error {
	r.mu.RLock()
	state := r.states[name]
	if state == nil {
		r.mu.RUnlock()
		return nil
	}
	openedUntil := state.OpenedUntil
	r.mu.RUnlock()
	if !openedUntil.IsZero() && time.Now().Before(openedUntil) {
		return fmt.Errorf("provider circuit open: %s until %s", name, openedUntil.UTC().Format(time.RFC3339))
	}
	return nil
}

func (r *Registry) recordSuccess(name string, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.ensureStateLocked(name)
	state.Status = "ok"
	state.Enabled = true
	state.FailureCount = 0
	state.LastError = ""
	state.LatencyMS = latency.Milliseconds()
	state.CheckedAt = time.Now().UTC()
	state.OpenedUntil = time.Time{}
}

func (r *Registry) recordFailure(name string, latency time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.ensureStateLocked(name)
	state.Enabled = true
	state.FailureCount++
	state.LastError = err.Error()
	state.LatencyMS = latency.Milliseconds()
	state.CheckedAt = time.Now().UTC()
	state.Status = "error"
	if state.FailureCount >= r.failureThreshold {
		state.Status = "open"
		state.OpenedUntil = time.Now().UTC().Add(r.openInterval)
	}
}

func (r *Registry) RecordProbe(name string, enabled bool, status string, detail string, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.ensureStateLocked(name)
	state.Enabled = enabled
	state.Status = strings.TrimSpace(status)
	if state.Status == "" {
		state.Status = "unknown"
	}
	state.LastError = strings.TrimSpace(detail)
	state.LatencyMS = latency.Milliseconds()
	state.CheckedAt = time.Now().UTC()
	if state.Status == "ok" || state.Status == "disabled" {
		state.FailureCount = 0
		if state.Status == "ok" {
			state.OpenedUntil = time.Time{}
		}
	}
}

func (r *Registry) HealthStatuses() []ProviderHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProviderHealth, 0, len(r.states))
	for _, state := range r.states {
		out = append(out, state.ProviderHealth)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (r *Registry) ensureStateLocked(name string) *providerState {
	key := strings.ToLower(strings.TrimSpace(name))
	if state, ok := r.states[key]; ok {
		return state
	}
	state := &providerState{ProviderHealth: ProviderHealth{Name: name, Type: providerType(name), Enabled: true, Status: "unknown"}}
	r.states[key] = state
	return state
}

func providerType(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(lower, "mock"):
		return "mock"
	case strings.Contains(lower, "openai"):
		return "openai"
	default:
		return "provider"
	}
}

func shouldRetry(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return !strings.HasPrefix(lower, "mock")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type FallbackRoute struct {
	Model    string
	Provider string
	Reason   string
}

type FallbackResult struct {
	UsedFallback  bool
	Attempts      int
	FinalProvider string
	FinalModel    string
}

func (r *Registry) ChatCompletionWithFallback(ctx context.Context, chain []FallbackRoute, req ChatCompletionRequest) (ChatCompletionResponse, FallbackResult, error) {
	result := FallbackResult{}
	if len(chain) == 0 {
		return ChatCompletionResponse{}, result, fmt.Errorf("fallback chain is empty")
	}
	var lastErr error
	for i, route := range chain {
		providerName := route.Provider
		if providerName == "" {
			providerName = r.Resolve(route.Model).Name()
		}
		provider := r.Resolve(providerName)
		if provider == nil {
			lastErr = fmt.Errorf("provider not found: %s", providerName)
			continue
		}
		name := strings.ToLower(strings.TrimSpace(provider.Name()))
		if err := r.beforeCall(name); err != nil {
			lastErr = err
			continue
		}
		attempts := r.maxRetries + 1
		if attempts < 1 {
			attempts = 1
		}
		for attempt := 1; attempt <= attempts; attempt++ {
			started := time.Now()
			resp, err := provider.ChatCompletion(ctx, req)
			latency := time.Since(started)
			if err == nil {
				r.recordSuccess(name, latency)
				result.Attempts = i + 1
				result.FinalProvider = provider.Name()
				result.FinalModel = route.Model
				result.UsedFallback = i > 0
				return resp, result, nil
			}
			lastErr = err
			r.recordFailure(name, latency, err)
			if attempt == attempts || ctx.Err() != nil || !shouldRetry(name) {
				break
			}
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
			if err := r.beforeCall(name); err != nil {
				lastErr = err
				break
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all fallback providers failed")
	}
	return ChatCompletionResponse{}, result, lastErr
}
