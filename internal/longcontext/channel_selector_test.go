package longcontext

import (
	"testing"
	"time"
)

func TestChannelSelectorRotatesAndCoolsDown(t *testing.T) {
	now := time.Unix(100, 0)
	s := NewChannelSelector([]string{"a", "b"}, time.Minute)
	if got := s.Pick(now); got != "a" {
		t.Fatal(got)
	}
	s.Failure("a", 429, now)
	if got := s.Pick(now); got != "b" {
		t.Fatal(got)
	}
	if got := s.Pick(now); got != "b" {
		t.Fatal(got)
	}
	if got := s.Pick(now.Add(time.Minute)); got != "a" {
		t.Fatal(got)
	}
}
func TestChannelSelectorDisables403(t *testing.T) {
	now := time.Unix(100, 0)
	s := NewChannelSelector([]string{"a", "b"}, time.Minute)
	s.Failure("a", 403, now)
	if got := s.Pick(now); got != "b" {
		t.Fatal(got)
	}
	if got := s.Pick(now.Add(365 * 24 * time.Hour)); got != "b" {
		t.Fatal(got)
	}
}
