package router

import (
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestDefaultPoolDecisionBindsConfiguredChannel(t *testing.T) {
	r := New("openai", "gpt-4o-mini")
	r.RegisterProductionModel("nvidia/test-model", "custom", "general", "test")
	r.SetDefaultModelPool([]string{"nvidia/test-model"})
	r.SetModelContextWindows(map[string]int{"nvidia/test-model": 1048576})
	r.SetChannels([]Channel{
		{ID: "openai:nvidia/test-model", Provider: "openai", Model: "nvidia/test-model", Enabled: true, Weight: 1},
		{ID: "nvidia-nim-key-test", Provider: "custom", Model: "nvidia/test-model", Enabled: true, Weight: 100},
	})
	d := r.Decide(providers.ChatCompletionRequest{})
	if d.Model != "nvidia/test-model" || d.Channel != "nvidia-nim-key-test" || d.Provider != "custom" {
		t.Fatalf("default pool decision = %+v, want model/channel/provider bound", d)
	}
}
