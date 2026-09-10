package limits

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Profile struct {
	Provider    string        `json:"provider"`
	Channel     string        `json:"channel"`
	Model       string        `json:"model"`
	RPM         int64         `json:"rpm"`
	TPM         int64         `json:"tpm"`
	Concurrency int64         `json:"concurrency"`
	Confidence  string        `json:"confidence"`
	LastProbe   time.Time     `json:"last_probe"`
	LastStatus  int           `json:"last_status"`
	RetryAfter  time.Duration `json:"-"`
}

func ParseHeaders(h http.Header) Profile {
	p := Profile{Confidence: "unknown", LastProbe: time.Now().UTC()}
	p.RPM = headerInt(h, "x-ratelimit-limit-requests")
	p.TPM = headerInt(h, "x-ratelimit-limit-tokens")
	if p.RPM > 0 || p.TPM > 0 {
		p.Confidence = "observed"
	}
	if v := strings.TrimSpace(h.Get("retry-after")); v != "" {
		if n, e := strconv.ParseFloat(v, 64); e == nil {
			p.RetryAfter = time.Duration(n * float64(time.Second))
		}
	}
	return p
}
func headerInt(h http.Header, n string) int64 {
	v := h.Get(n)
	if i := strings.IndexByte(v, ';'); i >= 0 {
		v = v[:i]
	}
	x, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return x
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	_, e := db.Exec(`CREATE TABLE IF NOT EXISTS model_limits(provider TEXT NOT NULL,channel TEXT NOT NULL DEFAULT '',model TEXT NOT NULL,rpm BIGINT NOT NULL DEFAULT 0,tpm BIGINT NOT NULL DEFAULT 0,concurrency BIGINT NOT NULL DEFAULT 0,confidence TEXT NOT NULL DEFAULT 'unknown',last_probe TIMESTAMPTZ,last_status INT NOT NULL DEFAULT 0,PRIMARY KEY(provider,channel,model))`)
	if e != nil {
		return nil, e
	}
	_, e = db.Exec(`
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS context_window BIGINT NOT NULL DEFAULT 0;
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS safe_context_window BIGINT NOT NULL DEFAULT 0;
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS context_confidence TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS context_last_probe TIMESTAMPTZ;
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS context_last_status INT NOT NULL DEFAULT 0;
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS context_error_type TEXT NOT NULL DEFAULT '';
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS context_probe_tokens BIGINT NOT NULL DEFAULT 0;
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS health_score DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS latency_score DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE model_limits ADD COLUMN IF NOT EXISTS score_updated_at TIMESTAMPTZ;
`)
	return s, e
}
func (s *Store) Upsert(ctx context.Context, p Profile) error {
	_, e := s.db.ExecContext(ctx, `INSERT INTO model_limits(provider,channel,model,rpm,tpm,concurrency,confidence,last_probe,last_status)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)ON CONFLICT(provider,channel,model)DO UPDATE SET rpm=EXCLUDED.rpm,tpm=EXCLUDED.tpm,concurrency=EXCLUDED.concurrency,confidence=EXCLUDED.confidence,last_probe=EXCLUDED.last_probe,last_status=EXCLUDED.last_status`, p.Provider, p.Channel, p.Model, p.RPM, p.TPM, p.Concurrency, p.Confidence, p.LastProbe, p.LastStatus)
	return e
}
func (s *Store) List(ctx context.Context) ([]Profile, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT provider,channel,model,rpm,tpm,concurrency,confidence,last_probe,last_status FROM model_limits`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		var p Profile
		if e = rows.Scan(&p.Provider, &p.Channel, &p.Model, &p.RPM, &p.TPM, &p.Concurrency, &p.Confidence, &p.LastProbe, &p.LastStatus); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type ContextProfile struct {
	Provider      string
	Channel       string
	Model         string
	ContextWindow int
	SafeWindow    int
	Confidence    string
	LastProbe     time.Time
	LastStatus    int
	ErrorType     string
}

func (s *Store) ListContextProfiles(ctx context.Context) ([]ContextProfile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider,channel,model,context_window,safe_context_window,context_confidence,context_last_probe,context_last_status,context_error_type FROM model_limits WHERE safe_context_window > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ContextProfile{}
	for rows.Next() {
		var p ContextProfile
		if err := rows.Scan(&p.Provider, &p.Channel, &p.Model, &p.ContextWindow, &p.SafeWindow, &p.Confidence, &p.LastProbe, &p.LastStatus, &p.ErrorType); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type Score struct {
	Provider string
	Channel  string
	Model    string
	Health   float64
	Latency  float64
	Updated  time.Time
}

// SaveScore persists one model's runtime health/latency scores so they
// survive gateway restarts.
func (s *Store) SaveScore(ctx context.Context, provider, channel, model string, health, latency float64) error {
	_, e := s.db.ExecContext(ctx, `
INSERT INTO model_limits(provider,channel,model,health_score,latency_score,score_updated_at)
VALUES($1,$2,$3,$4,$5,NOW())
ON CONFLICT(provider,channel,model)
DO UPDATE SET health_score=EXCLUDED.health_score, latency_score=EXCLUDED.latency_score, score_updated_at=EXCLUDED.score_updated_at`,
		provider, channel, model, health, latency)
	return e
}

// ListScores returns persisted scores for models that have observations.
func (s *Store) ListScores(ctx context.Context) ([]Score, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider,channel,model,health_score,latency_score,score_updated_at FROM model_limits WHERE health_score > 0 OR latency_score > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Score{}
	for rows.Next() {
		var sc Score
		var upd sql.NullTime
		if err := rows.Scan(&sc.Provider, &sc.Channel, &sc.Model, &sc.Health, &sc.Latency, &upd); err != nil {
			return nil, err
		}
		if upd.Valid {
			sc.Updated = upd.Time
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

type ProbeTarget struct{ Provider, Channel, BaseURL, APIKey, Model string }
type Probe struct {
	Client *http.Client
	Store  *Store
	mu     sync.Mutex
	last   map[string]time.Time
}

func NewProbe(s *Store) *Probe {
	return &Probe{Client: &http.Client{Timeout: 20 * time.Second}, Store: s, last: map[string]time.Time{}}
}

// Run performs one low-volume probe. It never intentionally exceeds one request.
func (p *Probe) Run(ctx context.Context, t ProbeTarget) (Profile, error) {
	now := time.Now().UTC()
	out := Profile{Provider: t.Provider, Channel: t.Channel, Model: t.Model, LastProbe: now, Confidence: "unknown"}
	if strings.TrimSpace(t.BaseURL) == "" || strings.TrimSpace(t.APIKey) == "" || strings.TrimSpace(t.Model) == "" {
		return out, fmt.Errorf("probe target incomplete")
	}
	body, _ := json.Marshal(map[string]any{"model": t.Model, "messages": []map[string]string{{"role": "user", "content": "ping"}}, "max_tokens": 1, "temperature": 0})
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(t.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if e != nil {
		return out, e
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, e := p.Client.Do(req)
	if e != nil {
		out.Confidence = "probe_error"
		if p.Store != nil {
			if storeErr := p.Store.Upsert(ctx, out); storeErr != nil {
				return out, storeErr
			}
		}
		return out, e
	}
	defer resp.Body.Close()
	out.LastStatus = resp.StatusCode
	out.ProfileFrom(resp.Header)
	if resp.StatusCode == 429 {
		out.Confidence = "rate_limited"
	} else if resp.StatusCode >= 400 {
		out.Confidence = "probe_error"
	}
	if p.Store != nil {
		if e = p.Store.Upsert(ctx, out); e != nil {
			return out, e
		}
	}
	return out, nil
}
func (p *Profile) ProfileFrom(h http.Header) {
	q := ParseHeaders(h)
	p.RPM = q.RPM
	p.TPM = q.TPM
	p.RetryAfter = q.RetryAfter
	if q.Confidence != "unknown" {
		p.Confidence = q.Confidence
	}
}

// Limiter applies the most recently observed safe RPM; unknown limits are not guessed.
type Limiter struct {
	mu       sync.Mutex
	next     map[string]time.Time
	profiles map[string]Profile
}

func NewLimiter() *Limiter {
	return &Limiter{next: map[string]time.Time{}, profiles: map[string]Profile{}}
}
func key(a, b string) string { return strings.ToLower(a) + ":" + strings.ToLower(b) }
func (l *Limiter) Set(p Profile) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.profiles[key(p.Provider, p.Model)] = p
}
func (l *Limiter) Allow(provider, model string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.profiles[key(provider, model)]
	if !ok || p.RPM <= 0 {
		return 0
	}
	k := key(provider, model)
	n := time.Now()
	if n.Before(l.next[k]) {
		return l.next[k].Sub(n)
	}
	l.next[k] = n.Add(time.Minute / time.Duration(p.RPM))
	return 0
}
func (l *Limiter) Backoff(provider, model string, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := key(provider, model)
	t := time.Now().Add(d)
	if t.After(l.next[k]) {
		l.next[k] = t
	}
}

type Scheduler struct {
	Probe     *Probe
	Interval  time.Duration
	Targets   func(context.Context) ([]ProbeTarget, error)
	OnProfile func(Profile)
}

func (s *Scheduler) Run(ctx context.Context) {
	if s == nil || s.Probe == nil || s.Targets == nil {
		return
	}
	if s.Interval <= 0 {
		s.Interval = 24 * time.Hour
	}
	run := func() {
		ts, e := s.Targets(ctx)
		if e != nil {
			return
		}
		for _, t := range ts {
			p, e := s.Probe.Run(ctx, t)
			if e == nil && s.OnProfile != nil {
				s.OnProfile(p)
			}
			time.Sleep(time.Second)
		}
	}
	run()
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
