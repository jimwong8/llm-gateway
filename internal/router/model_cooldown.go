package router

import (
	"sync"
	"time"
)

// ModelCooldown provides per-model cooldown isolation.
// When a model triggers a rate limit, only that model is isolated,
// other models continue to serve requests normally.
type ModelCooldown struct {
	mu         sync.RWMutex
	cooldowns  map[string]time.Time // model -> cooldown expiry
	defaultDur time.Duration
}

func NewModelCooldown() *ModelCooldown {
	return &ModelCooldown{
		cooldowns:  make(map[string]time.Time),
		defaultDur: 7200 * time.Second, // 2 hours
	}
}

func (m *ModelCooldown) IsCooling(model string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	expiry, ok := m.cooldowns[model]
	if !ok {
		return false
	}
	return time.Now().Before(expiry)
}

func (m *ModelCooldown) SetCooldown(model string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if duration <= 0 {
		duration = m.defaultDur
	}
	m.cooldowns[model] = time.Now().Add(duration)
}

func (m *ModelCooldown) Clear(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cooldowns, model)
}

func (m *ModelCooldown) Remaining(model string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	expiry, ok := m.cooldowns[model]
	if !ok {
		return 0
	}
	remaining := time.Until(expiry)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ModelCircuitBreaker provides per-model circuit breaking.
// After consecutive failures, the model is skipped for a period.
type ModelCircuitBreaker struct {
	mu                  sync.RWMutex
	consecutiveFailures map[string]int
	circuitOpenedAt     map[string]time.Time
	openAfter           int
	resetAfter          time.Duration
}

func NewModelCircuitBreaker() *ModelCircuitBreaker {
	return &ModelCircuitBreaker{
		consecutiveFailures: make(map[string]int),
		circuitOpenedAt:     make(map[string]time.Time),
		openAfter:           3,
		resetAfter:          120 * time.Second,
	}
}

func (m *ModelCircuitBreaker) IsCircuitOpen(model string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	failures, ok := m.consecutiveFailures[model]
	if !ok || failures < m.openAfter {
		return false
	}
	openedAt := m.circuitOpenedAt[model]
	if time.Since(openedAt) > m.resetAfter {
		return false
	}
	return true
}

func (m *ModelCircuitBreaker) RecordFailure(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consecutiveFailures[model]++
	if m.consecutiveFailures[model] == m.openAfter {
		m.circuitOpenedAt[model] = time.Now()
	}
}

func (m *ModelCircuitBreaker) RecordSuccess(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.consecutiveFailures, model)
	delete(m.circuitOpenedAt, model)
}
