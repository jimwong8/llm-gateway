package router

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// AdaptiveRateLimiter implements adaptive RPM (requests per minute) limiting.
// On 429, it caps to the effective rate and gradually recovers on success.
type AdaptiveRateLimiter struct {
	mu         sync.Mutex
	window     time.Duration // 60s
	initialRPM int           // 40
	currentRPM int           // current effective RPM
	cappedRPM  *int          // nil = not capped
	successCnt int           // success counter
	rpmReset   int           // recover after N successes
	requests   *list.List    // timestamp queue
}

func NewAdaptiveRateLimiter(rpm int) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		window:     60 * time.Second,
		initialRPM: rpm,
		currentRPM: rpm,
		rpmReset:   5,
		requests:   list.New(),
	}
}

func (r *AdaptiveRateLimiter) prune() {
	cutoff := time.Now().Add(-r.window)
	for e := r.requests.Front(); e != nil; {
		next := e.Next()
		if e.Value.(time.Time).Before(cutoff) {
			r.requests.Remove(e)
		} else {
			break
		}
		e = next
	}
}

func (r *AdaptiveRateLimiter) CanSend() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prune()
	return r.requests.Len() < r.currentRPM
}

func (r *AdaptiveRateLimiter) Record() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests.PushBack(time.Now())
}

func (r *AdaptiveRateLimiter) OnRateLimited() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.successCnt = 0
	r.prune()
	effectiveCount := r.requests.Len()
	effectiveRPM := intMax(1, intMin(effectiveCount, r.currentRPM))

	if r.cappedRPM == nil {
		r.cappedRPM = &effectiveRPM
		r.currentRPM = effectiveRPM
	} else if r.currentRPM == *r.cappedRPM {
		if *r.cappedRPM > 1 {
			*r.cappedRPM--
			r.currentRPM = *r.cappedRPM
		}
	}
}

func (r *AdaptiveRateLimiter) OnSuccess() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.rpmReset <= 0 {
		return
	}
	r.successCnt++
	if r.successCnt >= r.rpmReset {
		r.successCnt = 0
		if r.cappedRPM != nil {
			if r.currentRPM < r.initialRPM {
				r.currentRPM++
				*r.cappedRPM = r.currentRPM
			} else {
				r.cappedRPM = nil
			}
		}
	}
}

func (r *AdaptiveRateLimiter) WaitUntilCanSend(ctx context.Context) error {
	for !r.CanSend() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}

func (r *AdaptiveRateLimiter) CurrentRPM() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentRPM
}

// KeyRPMTracker tracks per-key RPM with adaptive ceiling.
type KeyRPMTracker struct {
	mu              sync.Mutex
	window          time.Duration
	maxRPM          int // current ceiling (initial 28)
	observedCeiling int // inferred from Retry-After
	timestamps      []time.Time
}

func NewKeyRPMTracker() *KeyRPMTracker {
	return &KeyRPMTracker{
		window: 60 * time.Second,
		maxRPM: 28, // 30% margin under 40
	}
}

func (k *KeyRPMTracker) LearnFromRetryAfter(retryAfterSec float64) {
	k.mu.Lock()
	defer k.mu.Unlock()
	implied := intMax(1, int(60.0/retryAfterSec))
	if k.observedCeiling == 0 || implied < k.observedCeiling {
		k.observedCeiling = implied
	}
}

func (k *KeyRPMTracker) prune() {
	cutoff := time.Now().Add(-k.window)
	valid := k.timestamps[:0]
	for _, t := range k.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	k.timestamps = valid
}

func (k *KeyRPMTracker) CanSend() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.prune()
	ceiling := k.maxRPM
	if k.observedCeiling > 0 {
		ceiling = intMin(ceiling, k.observedCeiling)
	}
	return len(k.timestamps) < ceiling
}

func (k *KeyRPMTracker) Record() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.timestamps = append(k.timestamps, time.Now())
}

func (k *KeyRPMTracker) OnRateLimited() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.maxRPM = intMax(14, int(float64(k.maxRPM)*0.5)) // floor 14
}

func (k *KeyRPMTracker) OnSuccess() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.maxRPM = intMin(28, k.maxRPM+10) // +10 recovery, max 28
	if k.maxRPM >= 28 {
		k.observedCeiling = 0
	}
}

// AccountWideDetector detects account-wide rate limits (multiple keys 429 simultaneously).
type AccountWideDetector struct {
	mu                 sync.Mutex
	recent429s         []struct {
		timestamp time.Time
		keyID     string
	}
	poolThrottledUntil time.Time
}

const (
	pool429Window      = 10 * time.Second
	pool429Distinct    = 3
	poolThrottlePause   = 15 * time.Second
)

func NewAccountWideDetector() *AccountWideDetector {
	return &AccountWideDetector{}
}

func (a *AccountWideDetector) NoteRateLimited(keyID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	a.recent429s = append(a.recent429s, struct {
		timestamp time.Time
		keyID     string
	}{now, keyID})

	cutoff := now.Add(-pool429Window)
	valid := a.recent429s[:0]
	for _, r := range a.recent429s {
		if r.timestamp.After(cutoff) {
			valid = append(valid, r)
		}
	}
	a.recent429s = valid

	if now.Before(a.poolThrottledUntil) {
		return false
	}

	distinct := make(map[string]bool)
	for _, r := range a.recent429s {
		distinct[r.keyID] = true
	}

	if len(distinct) >= pool429Distinct {
		a.poolThrottledUntil = now.Add(poolThrottlePause)
		return true
	}
	return false
}

func (a *AccountWideDetector) IsPoolThrottled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Now().Before(a.poolThrottledUntil)
}

func (a *AccountWideDetector) ThrottleRemaining() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	remaining := time.Until(a.poolThrottledUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
