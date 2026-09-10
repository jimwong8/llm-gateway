package longcontext

import (
	"context"
	"sort"
	"sync"
	"time"
)

type ChannelHealth struct {
	ID            string
	Failures      int
	CooldownUntil time.Time
	LastUsed      time.Time
}
type ChannelSelector struct {
	mu       sync.Mutex
	items    map[string]*ChannelHealth
	cooldown time.Duration
}

func NewChannelSelector(ids []string, cooldown time.Duration) *ChannelSelector {
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	s := &ChannelSelector{items: map[string]*ChannelHealth{}, cooldown: cooldown}
	for _, id := range ids {
		s.items[id] = &ChannelHealth{ID: id}
	}
	return s
}
func (s *ChannelSelector) Pick(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := []*ChannelHealth{}
	for _, x := range s.items {
		if now.Before(x.CooldownUntil) {
			continue
		}
		a = append(a, x)
	}
	sort.Slice(a, func(i, j int) bool {
		if a[i].Failures != a[j].Failures {
			return a[i].Failures < a[j].Failures
		}
		if !a[i].LastUsed.Equal(a[j].LastUsed) {
			if !a[i].LastUsed.Equal(a[j].LastUsed) {
				return a[i].LastUsed.Before(a[j].LastUsed)
			}
			return a[i].ID < a[j].ID
		}
		return a[i].ID < a[j].ID
	})
	if len(a) == 0 {
		return ""
	}
	a[0].LastUsed = now
	return a[0].ID
}
func (s *ChannelSelector) Success(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x := s.items[id]; x != nil {
		x.Failures = 0
		x.CooldownUntil = time.Time{}
	}
}
func (s *ChannelSelector) Failure(id string, status int, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x := s.items[id]; x == nil {
		return
	} else if status == 403 {
		x.Failures = 99
		x.CooldownUntil = now.Add(365 * 24 * time.Hour)
	} else if status == 429 {
		x.CooldownUntil = now.Add(s.cooldown)
	} else {
		x.Failures++
		x.CooldownUntil = now.Add(time.Duration(x.Failures) * time.Second)
	}
}

// PickWithTimeout blocks until at least one channel is out of cooldown or the
// context is canceled. It returns the selected channel ID or an empty string
// on timeout/cancellation.
func (s *ChannelSelector) PickWithTimeout(ctx context.Context) string {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if id := s.Pick(time.Now()); id != "" {
			return id
		}
		select {
		case <-ctx.Done():
			return ""
		case <-ticker.C:
		}
	}
}
