package providers

import (
	"testing"

	"llm-gateway/gateway/internal/config"
)

func TestPreferredChannelForModelRoundRobin(t *testing.T) {
	r := NewRegistry(config.Config{}, &stubProvider{name: "fallback"})
	for i := 1; i <= 3; i++ {
		p := &stubProvider{name: "key-provider"}
		r.RegisterChannelModel("nvidia-nim-key-0"+string(rune('0'+i)), "shared-model", p)
	}
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		id, ok := r.PreferredChannelForModel("shared-model")
		if !ok {
			t.Fatal("expected channel")
		}
		seen[id] = true
	}
	if len(seen) != 3 {
		t.Fatalf("round robin used %d distinct channels, want 3: %v", len(seen), seen)
	}
}
