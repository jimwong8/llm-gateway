// MOD1 scored channel selection (2026-09-07).
//
// Upgrades mod1 channel selection from pure Redis round-robin to
// EMA-score-weighted selection, grounded in the ORCH paper (Frontiers 2026)
// and the revived model-matrix v2 scoring (live_scoring.py). Scores come
// from two inputs:
//   1. Real usage stats (usage_events, 7-day window): success rate and
//      average latency per upstream model — refreshed from the DB by the
//      background refresher below (every 5 minutes).
//   2. Static role affinity: each mod1 channel has a role (arch/build/
//      craft/scout) encoded in its id; role weights mirror the Python
//      scoring engine (fast/cheap dominate scout, reasoning dominates arch).
//
// Selection: weighted random over live scores (power-of-choice style).
// Channels with score <= 0 (frozen, or success < 50%) are excluded and the
// round-robin fallback applies only when no scored data exists yet.
package httpserver

import (
	"log/slog"
	"context"
	"math/rand"
	"strings"
	"sync"
	"time"

	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/providers"
)

// mod1ChannelScore is the live EMA score of one mod1 channel.
type mod1ChannelScore struct {
	channelID string
	upstream  string
	role      string
	score     float64 // 0..1 (1 = best)
	updatedAt time.Time
}

type mod1Scorer struct {
	mu        sync.RWMutex
	scores    map[string]*mod1ChannelScore // channelID -> score
	rand      *rand.Rand
	store     usageStatsStore // where we read per-model stats from
	registry  *providers.Registry
	stop      chan struct{}
	stopped   sync.WaitGroup
}

// usageStatsStore is the minimal interface the scorer needs; satisfied by
// *billing.Store (ModelUsageStats added 2026-09-07).
type usageStatsStore interface {
	ModelUsageStats(ctx context.Context, days int) (map[string]billing.ModelUsageStat, error)
}

// mod1RoleWeights maps the channel role suffix to the dimension emphasis
// (mirrors live_scoring.py ROLE_WEIGHTS, normalized).
var mod1RoleWeights = map[string]struct{ fast, cheap, reasoning float64 }{
	"arch":  {fast: 0.1, cheap: 0.1, reasoning: 0.8},
	"build": {fast: 0.2, cheap: 0.2, reasoning: 0.6},
	"craft": {fast: 0.3, cheap: 0.3, reasoning: 0.4},
	"scout": {fast: 0.6, cheap: 0.3, reasoning: 0.1},
}

func mod1RoleOf(channelID string) string {
	id := strings.ToLower(strings.TrimSpace(channelID))
	if !strings.HasPrefix(id, "mod1-") {
		return ""
	}
	rest := strings.TrimPrefix(id, "mod1-")
	rest = strings.TrimSuffix(rest, "-x5")
	rest = strings.TrimSuffix(rest, "-bai")
	return rest
}

func newMod1Scorer(store usageStatsStore) *mod1Scorer {
	return &mod1Scorer{
		scores: make(map[string]*mod1ChannelScore),
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
		store:  store,
		stop:   make(chan struct{}),
	}
}

// Start launches the background refresher (usage stats -> channel scores).
func (m *mod1Scorer) Start(interval time.Duration) {
	m.stopped.Add(1)
	go func() {
		defer m.stopped.Done()
		t := time.NewTicker(interval)
		defer t.Stop()
		m.refreshOnce(context.Background())
		for {
			select {
			case <-m.stop:
				return
			case <-t.C:
				m.refreshOnce(context.Background())
			}
		}
	}()
}

func (m *mod1Scorer) Stop() {
	close(m.stop)
	m.stopped.Wait()
}

// refreshOnce pulls usage stats and recomputes every known mod1 channel's
// score. Channels are discovered lazily via UpdateChannels (called from the
// server when it registers mod1 channels).
func (m *mod1Scorer) refreshOnce(ctx context.Context) {
	if m.store == nil {
		return
	}
	// Re-discover channels each refresh (registry may be populated after init).
	m.UpdateChannels(m.registry)
	stats, err := m.store.ModelUsageStats(ctx, 7)
	if err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cs := range m.scores {
		usage, ok := modelUsageFor(stats, cs.upstream)
		if !ok {
			// No traffic in 7-day window: penalize to below threshold so
			// unproven channels are not selected over known-good ones.
			cs.score = 0.15
		} else if usage.Requests < 10 {
			// Too few requests to trust the score: penalize below threshold.
			cs.score = 0.15
		} else {
			cs.score = mod1ComputeScore(usage, cs.role)
		}
		cs.updatedAt = time.Now()
	}
}

// modelUsageFor matches an upstream name against usage stats keys which may
// carry a provider prefix (e.g. "x-ai/grok-4.6" vs "grok-4.6").
func modelUsageFor(stats map[string]billing.ModelUsageStat, upstream string) (billing.ModelUsageStat, bool) {
	if u, ok := stats[upstream]; ok {
		return u, true
	}
	bare := strings.ToLower(upstream)
	for k, u := range stats {
		parts := strings.Split(k, "/")
		last := parts[len(parts)-1]
		if strings.EqualFold(last, upstream) || strings.ToLower(parts[0]) == bare {
			return u, true
		}
	}
	return billing.ModelUsageStat{}, false
}

// mod1ComputeScore maps usage stats + role emphasis to 0..1.
func mod1ComputeScore(u billing.ModelUsageStat, role string) float64 {
	w, ok := mod1RoleWeights[role]
	if !ok {
		w = mod1RoleWeights["build"]
	}
	// Latency component: 1s -> 1.0, 40s -> 0.0 (clamped).
	latScore := 1.0 - (u.AvgLatencyMS-1000)/39000.0
	if latScore < 0 {
		latScore = 0
	}
	if latScore > 1 {
		latScore = 1
	}
	// Success component with hard floor: below 50% the channel is unsuitable.
	succ := u.SuccessRate
	if succ < 0.5 {
		return 0.05 // heavily penalized but not zero (avoids total exclusion
		// of a channel that could still serve as last resort)
	}
	score := w.fast*latScore + w.reasoning*succ + w.cheap*0.9 /* free pool */
	if score > 1 {
		score = 1
	}
	// Latency hard penalty: slow channels must not dominate even with
	// high success rates.  >10s -> near-floor (0.1), >5s -> 40% cut.
	// Fixes mimo-v2.5 (80% succ but 11.5s avg) crowding out faster channels.
	if u.AvgLatencyMS > 10000 {
		return 0.1
	}
	if u.AvgLatencyMS > 5000 {
		return score * 0.4
	}
	return score
}

// UpdateChannels registers/refreshes the mod1 channel set (id + upstream).
func (m *mod1Scorer) UpdateChannels(p *providers.Registry) {
	if p == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Discover mod1 channels by probing the known 8-slot naming scheme plus
	// any channel that serves the mod1 virtual model.
	for _, chID := range mod1KnownChannels() {
		upstream, ok := p.UpstreamModelForChannel(chID, mod1VirtualModel)
		if !ok {
			// Registry not populated yet — fall back to static overrides.
			upstream, ok = mod1UpstreamOverrides[chID]
		}
		if ok {
			cs, exists := m.scores[chID]
			if !exists {
				cs = &mod1ChannelScore{channelID: chID, score: 0.15}
				m.scores[chID] = cs
			}
			cs.upstream = upstream
			cs.role = mod1RoleOf(chID)
		}
	}
}

func mod1KnownChannels() []string {
	roles := []string{"arch", "build", "craft", "scout"}
	out := make([]string, 0, len(roles)*2)
	for _, r := range roles {
		out = append(out, "mod1-"+r, "mod1-"+r+"-x5", "mod1-"+r+"-bai")
	}
	return out
}

// Pick selects the best mod1 channel by score (weighted power-of-choice:
// sample 3, take the best — balances exploitation with spread).
// Returns ("", false) when no channels are known — caller falls back to
// the existing round-robin path.
func (m *mod1Scorer) Pick() (string, bool) {
	m.mu.RLock()
	if len(m.scores) == 0 {
		m.mu.RUnlock()
		return "", false
	}
	n := len(m.scores)
	sample := 3
	if sample > n {
		sample = n
	}
	ids := make([]string, 0, n)
	for id := range m.scores {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	// Sample `sample` distinct channels, pick highest score.
	bestID, bestScore := "", -1.0
	picked := map[string]bool{}
	for len(picked) < sample {
		m.mu.RLock()
		pool := ids
		m.mu.RUnlock()
		if len(pool) == 0 {
			break
		}
		cand := pool[m.rand.Intn(len(pool))]
		if picked[cand] {
			continue
		}
		picked[cand] = true
		m.mu.RLock()
		sc := 0.0
		if cs, ok := m.scores[cand]; ok {
			sc = cs.score
		}
		m.mu.RUnlock()
		if sc > bestScore {
			bestID, bestScore = cand, sc
		}
	}
	// Reject low-confidence picks: a channel whose score is at the penalty
	// floor (0.05, success < 50%) must never be selected — equality was a bug
	// (deepseek-v4-flash at exactly 0.05 was still being picked during the
	// 2026-09-06 sensenova rate-limit incident). Threshold 0.2 also filters
	// mediocre channels, deferring to round-robin + failover.
	if bestID == "" || bestScore < 0.2 {
		return "", false
	}
	return bestID, true
}

// PickBest returns the highest-scoring channel regardless of the 0.2
// threshold used by Pick. It is the fallback when Pick returns false,
// replacing the old Redis round-robin which blindly selected channels
// (including slow/unstable ones like mimo-v2.5 at 0.1). Only channels
// at the frozen floor (<=0.05, success <50%) are excluded.
func (m *mod1Scorer) PickBest() (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.scores) == 0 {
		return "", false
	}
	bestID, bestScore := "", -1.0
	for id, cs := range m.scores {
		if cs.score <= 0.05 {
			continue // frozen, skip
		}
		if cs.score > bestScore {
			bestID, bestScore = id, cs.score
		}
	}
	if bestID == "" {
		return "", false
	}
	return bestID, true
}

// Snapshot returns current scores for observability (admin/debug endpoint).
func (m *mod1Scorer) Snapshot() []mod1ChannelScore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]mod1ChannelScore, 0, len(m.scores))
	for _, cs := range m.scores {
		out = append(out, *cs)
	}
	return out
}

// mod1ScoredChannel asks the scorer for the best channel; it also lazily
// starts the scorer on first use (server wiring is not always available in
// tests).
func (s *Server) mod1ScoredChannel() (string, bool) {
	if s.mod1Scorer == nil {
		return "", false
	}
	return s.mod1Scorer.Pick()
}

// WithMod1Scorer attaches the mod1 scored selector and starts its
// background refresh (5-minute interval). billingStore supplies usage stats.
func (s *Server) WithMod1Scorer(store *billing.Store) *Server {
	if store == nil {
		return s
	}
	scorer := newMod1Scorer(store)
	scorer.registry = s.providers
	scorer.UpdateChannels(s.providers)
	scorer.Start(5 * time.Minute)
	s.mod1Scorer = scorer
	slog.Info("mod1 scorer started", "channels", len(scorer.Snapshot()))
	return s
}
