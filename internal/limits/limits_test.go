package limits

import (
	"net/http"
	"testing"
	"time"
)

func TestParseHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("X-RateLimit-Limit-Requests", "60")
	h.Set("X-RateLimit-Limit-Tokens", "1000")
	h.Set("Retry-After", "2")
	p := ParseHeaders(h)
	if p.RPM != 60 || p.TPM != 1000 || p.RetryAfter != 2*time.Second || p.Confidence != "observed" {
		t.Fatalf("%+v", p)
	}
}
func TestLimiter(t *testing.T) {
	l := NewLimiter()
	l.Set(Profile{Provider: "openai", Model: "m", RPM: 60})
	if d := l.Allow("openai", "m"); d != 0 {
		t.Fatal(d)
	}
	if d := l.Allow("openai", "m"); d <= 0 {
		t.Fatal("second request was not throttled")
	}
	l.Backoff("openai", "m", time.Second)
	if d := l.Allow("openai", "m"); d <= 0 {
		t.Fatal("backoff was not applied")
	}
}
