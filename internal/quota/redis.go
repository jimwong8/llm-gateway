package quota

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	client *redis.Client
	rpm    atomic.Int64
}

type Summary struct {
	TenantID   string  `json:"tenant_id"`
	Used       int64   `json:"used"`
	Limit      int     `json:"limit"`
	Remaining  int64   `json:"remaining"`
	Rejected   int64   `json:"rejected"`
	RejectRate float64 `json:"reject_rate"`
}

type TrendPoint struct {
	Minute            string `json:"minute"`
	Used              int64  `json:"used"`
	Rejected          int64  `json:"rejected"`
	RemainingEstimate int64  `json:"remaining_estimate"`
}

func New(addr string, rpm int) *Limiter {
	limiter := &Limiter{client: redis.NewClient(&redis.Options{Addr: addr})}
	limiter.SetRPM(rpm)
	return limiter
}

func (l *Limiter) SetRPM(rpm int) {
	if l == nil {
		return
	}
	if rpm < 0 {
		rpm = 0
	}
	l.rpm.Store(int64(rpm))
}

func (l *Limiter) RPM() int {
	if l == nil {
		return 0
	}
	return int(l.rpm.Load())
}

func (l *Limiter) Allow(ctx context.Context, tenantID string) (bool, int64, error) {
	currentRPM := l.RPM()
	if l == nil || currentRPM <= 0 || tenantID == "" {
		return true, 0, nil
	}
	now := time.Now().UTC()
	minuteKey := now.Format("200601021504")
	usedKey := fmt.Sprintf("quota:rpm:used:%s:%s", tenantID, minuteKey)
	rejectedKey := fmt.Sprintf("quota:rpm:rejected:%s:%s", tenantID, minuteKey)
	legacyKey := fmt.Sprintf("quota:rpm:%s:%s", tenantID, minuteKey)

	n, err := l.client.Incr(ctx, usedKey).Result()
	if err != nil {
		return false, 0, err
	}
	if n == 1 {
		_ = l.client.Expire(ctx, usedKey, time.Minute+5*time.Second).Err()
	}
	// keep legacy key for backward compatibility
	_ = l.client.Set(ctx, legacyKey, n, time.Minute+5*time.Second).Err()

	allowed := n <= int64(currentRPM)
	if !allowed {
		r, err := l.client.Incr(ctx, rejectedKey).Result()
		if err == nil && r == 1 {
			_ = l.client.Expire(ctx, rejectedKey, time.Minute+5*time.Second).Err()
		}
	}
	return allowed, n, nil
}

func (l *Limiter) Summary(ctx context.Context, tenantID string) (Summary, error) {
	currentRPM := l.RPM()
	if l == nil || currentRPM <= 0 || tenantID == "" {
		return Summary{TenantID: tenantID}, nil
	}
	now := time.Now().UTC().Format("200601021504")
	usedKey := fmt.Sprintf("quota:rpm:used:%s:%s", tenantID, now)
	rejectedKey := fmt.Sprintf("quota:rpm:rejected:%s:%s", tenantID, now)
	used, err := l.client.Get(ctx, usedKey).Int64()
	if err == redis.Nil {
		used = 0
	} else if err != nil {
		return Summary{}, err
	}
	rejected, err := l.client.Get(ctx, rejectedKey).Int64()
	if err == redis.Nil {
		rejected = 0
	} else if err != nil {
		return Summary{}, err
	}
	remaining := int64(currentRPM) - used
	if remaining < 0 {
		remaining = 0
	}
	rate := 0.0
	if used > 0 {
		rate = float64(rejected) / float64(used)
	}
	return Summary{TenantID: tenantID, Used: used, Limit: currentRPM, Remaining: remaining, Rejected: rejected, RejectRate: rate}, nil
}

func (l *Limiter) Trends(ctx context.Context, tenantID string, windowMinutes int) ([]TrendPoint, error) {
	currentRPM := l.RPM()
	if l == nil || currentRPM <= 0 || tenantID == "" {
		return nil, nil
	}
	if windowMinutes <= 0 {
		windowMinutes = 5
	}
	points := make([]TrendPoint, 0, windowMinutes)
	now := time.Now().UTC()
	for i := windowMinutes - 1; i >= 0; i-- {
		ts := now.Add(-time.Duration(i) * time.Minute)
		minuteKey := ts.Format("200601021504")
		usedKey := fmt.Sprintf("quota:rpm:used:%s:%s", tenantID, minuteKey)
		rejectedKey := fmt.Sprintf("quota:rpm:rejected:%s:%s", tenantID, minuteKey)
		used, err := l.client.Get(ctx, usedKey).Int64()
		if err == redis.Nil {
			used = 0
		} else if err != nil {
			return nil, err
		}
		rejected, err := l.client.Get(ctx, rejectedKey).Int64()
		if err == redis.Nil {
			rejected = 0
		} else if err != nil {
			return nil, err
		}
		remaining := int64(currentRPM) - used
		if remaining < 0 {
			remaining = 0
		}
		points = append(points, TrendPoint{
			Minute:            ts.Format(time.RFC3339),
			Used:              used,
			Rejected:          rejected,
			RemainingEstimate: remaining,
		})
	}
	return points, nil
}
