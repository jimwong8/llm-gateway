package providers

import (
    "testing"
    "time"

    "llm-gateway/gateway/internal/config"
)

func TestPreferredChannelForModelReturnsUnavailableWhenAllChannelsCoolingDown(t *testing.T) {
    r := NewRegistry(config.Config{}, &stubProvider{name: "fallback"})
    r.RegisterChannelModel("key-a", "shared-model", &stubProvider{name: "provider-a"})
    r.RegisterChannelModel("key-b", "shared-model", &stubProvider{name: "provider-b"})
    r.mu.Lock()
    r.coolDowns["key-a"] = time.Now().Add(time.Minute)
    r.coolDowns["key-b"] = time.Now().Add(time.Minute)
    r.mu.Unlock()

    if got, ok := r.PreferredChannelForModel("shared-model"); ok || got != "" {
        t.Fatalf("expected no available channel, got %q, ok=%v", got, ok)
    }
}
