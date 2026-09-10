// AUTO pool scored refresh (2026-09-07).
//
// Adds periodic batch-refresh of router model scores from usage_events
// (7-day aggregate), complementing the per-request EMA in RecordFeedback.
// Pattern mirrors mod1_scorer.go: background goroutine + HTTP admin endpoint.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/router"
)

type autoScorer struct {
	mu       sync.RWMutex
	store    usageStatsStore
	router   *router.Router
	scores   map[string]AutoScore
	stop     chan struct{}
	stopped  sync.WaitGroup
	interval time.Duration
}

// AutoScore is one model's computed score snapshot.
type AutoScore struct {
	Model        string    `json:"model"`
	Health       float64   `json:"health"`
	Latency      float64   `json:"latency"`
	SuccessRate  float64   `json:"success_rate"`
	AvgLatencyMS float64   `json:"avg_latency_ms"`
	Requests     int64     `json:"requests"`
	UpdatedAt    time.Time `json:"updated_at"`
	Source       string    `json:"source"`
}

func newAutoScorer(store usageStatsStore, rt *router.Router, interval time.Duration) *autoScorer {
	return &autoScorer{
		store:    store,
		router:   rt,
		scores:   make(map[string]AutoScore),
		stop:     make(chan struct{}),
		interval: interval,
	}
}

func (a *autoScorer) Start() {
	a.stopped.Add(1)
	go func() {
		defer a.stopped.Done()
		a.refresh()
		t := time.NewTicker(a.interval)
		defer t.Stop()
		for {
			select {
			case <-a.stop:
				return
			case <-t.C:
				a.refresh()
			}
		}
	}()
}

func (a *autoScorer) Stop() {
	close(a.stop)
	a.stopped.Wait()
}

func (a *autoScorer) refresh() {
	if a.store == nil || a.router == nil {
		return
	}
	stats, err := a.store.ModelUsageStats(context.Background(), 7)
	if err != nil {
		return
	}
	a.mu.Lock()
	scoreMap := make(map[string]struct{ Health, Latency float64 })
	for model, u := range stats {
		health := u.SuccessRate
		if health < 0.05 {
			health = 0.05
		}
		lat := 1.0 - (u.AvgLatencyMS-1000)/39000.0
		if lat < 0.05 {
			lat = 0.05
		}
		if lat > 1.0 {
			lat = 1.0
		}
		key := strings.ToLower(strings.TrimSpace(model))
		a.scores[key] = AutoScore{
			Model:        model,
			Health:       health,
			Latency:      lat,
			SuccessRate:  u.SuccessRate,
			AvgLatencyMS: u.AvgLatencyMS,
			Requests:     u.Requests,
			UpdatedAt:    time.Now(),
			Source:       "usage_events",
		}
		scoreMap[key] = struct{ Health, Latency float64 }{Health: health, Latency: lat}
	}
	a.mu.Unlock()
	if len(scoreMap) > 0 {
		a.router.SetModelScores(scoreMap)
	}
}

func (a *autoScorer) Snapshot() []AutoScore {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]AutoScore, 0, len(a.scores))
	for _, s := range a.scores {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Health != out[j].Health {
			return out[i].Health > out[j].Health
		}
		return out[i].Requests > out[j].Requests
	})
	return out
}

func (s *Server) WithAutoScorer(store *billing.Store) *Server {
	if store == nil || s.router == nil {
		return s
	}
	a := newAutoScorer(store, s.router, 5*time.Minute)
	a.Start()
	s.autoScorer = a
	slog.Info("auto scorer started", "interval", "5m")
	return s
}

func (s *Server) adminScores(w http.ResponseWriter, r *http.Request) {
	result := map[string]any{}

	if s.autoScorer != nil {
		scores := s.autoScorer.Snapshot()
		result["auto_scores"] = scores
		result["auto_score_count"] = len(scores)
	} else {
		result["auto_scores"] = []any{}
		result["auto_score_count"] = 0
	}

	if s.mod1Scorer != nil {
		mod1 := s.mod1Scorer.Snapshot()
		type mod1Out struct {
			ChannelID string  `json:"channel_id"`
			Upstream  string  `json:"upstream"`
			Role      string  `json:"role"`
			Score     float64 `json:"score"`
			UpdatedAt string  `json:"updated_at"`
		}
		out := make([]mod1Out, 0, len(mod1))
		for _, cs := range mod1 {
			out = append(out, mod1Out{
				ChannelID: cs.channelID,
				Upstream:  cs.upstream,
				Role:      cs.role,
				Score:     cs.score,
				UpdatedAt: cs.updatedAt.Format(time.RFC3339),
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
		result["mod1_scores"] = out
	} else {
		result["mod1_scores"] = []any{}
	}

	profiles := s.router.Models()
	type profileOut struct {
		Model          string  `json:"model"`
		Provider       string  `json:"provider"`
		StaticHealth   float64 `json:"static_health"`
		StaticLatency  float64 `json:"static_latency"`
		Capability     float64 `json:"capability"`
		CostScore      float64 `json:"cost_score"`
		LearnedHealth  float64 `json:"learned_health"`
		LearnedLatency float64 `json:"learned_latency"`
	}
	profOut := make([]profileOut, 0, len(profiles))
	for _, m := range profiles {
		po := profileOut{
			Model:         m.ID,
			Provider:      m.Provider,
			StaticHealth:  m.HealthScore,
			StaticLatency: m.LatencyScore,
			Capability:    m.Capability,
			CostScore:     m.CostScore,
		}
		if s.autoScorer != nil {
			for _, a := range s.autoScorer.Snapshot() {
				if strings.EqualFold(a.Model, m.ID) {
					po.LearnedHealth = a.Health
					po.LearnedLatency = a.Latency
					break
				}
			}
		}
		profOut = append(profOut, po)
	}
	result["model_profiles"] = profOut

	writeJSON(w, http.StatusOK, result)
}
