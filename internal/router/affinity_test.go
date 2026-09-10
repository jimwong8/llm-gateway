package router

import (
	"testing"

	"llm-gateway/gateway/internal/providers"
)

// fakeRegistry builds a Router with two AUTO candidates so affinity can be observed.
func newAffinityRouter(t *testing.T) *Router {
	r := New("gpt-fallback", "openai")
	r.RegisterProductionModel("model-a", "provider-a", "general", "A")
	r.RegisterProductionModel("model-b", "provider-b", "general", "B")
	// force both providers "open" so neither is skipped
	r.SetProviderHealthChecker(func(p string) bool { return false })
	r.SetModelContextWindows(map[string]int{"model-a": 1048576, "model-b": 1048576})
	return r
}

func TestPrefixFamilyAffinity_PinsSameProvider(t *testing.T) {
	r := newAffinityRouter(t)
	r.SetPrefixFamilyAffinity(true)

	reqA := providers.ChatCompletionRequest{Model: "AUTO", RouteMode: "auto", PrefixFamily: "pf1_sticky_x"}
	d1 := r.Decide(reqA)
	d2 := r.Decide(reqA)
	if d1.Provider != d2.Provider {
		t.Fatalf("same prefix_family routed to different providers: %s vs %s", d1.Provider, d2.Provider)
	}
	if d1.Provider == "" {
		t.Fatal("empty provider decision")
	}

	// a different prefix family should be allowed to pick independently
	reqB := providers.ChatCompletionRequest{Model: "AUTO", RouteMode: "auto", PrefixFamily: "pf1_other_y"}
	_ = r.Decide(reqB)
}

func TestPrefixFamilyAffinity_DisabledIgnoresFamily(t *testing.T) {
	r := newAffinityRouter(t)
	r.SetPrefixFamilyAffinity(false)

	// with affinity disabled, Decide must not consult PrefixFamily (no panic, deterministic)
	req := providers.ChatCompletionRequest{Model: "AUTO", RouteMode: "auto", PrefixFamily: "pf1_sticky_x"}
	_ = r.Decide(req)
}

func TestPrefixFamilyAffinity_ExplicitModelUntouched(t *testing.T) {
	r := newAffinityRouter(t)
	r.SetPrefixFamilyAffinity(true)
	// explicit model must still go to its resolved provider, affinity must not override
	req := providers.ChatCompletionRequest{Model: "model-a", PrefixFamily: "pf1_sticky_x"}
	d := r.Decide(req)
	if d.Provider != "provider-a" {
		t.Fatalf("explicit model affinity override: got %s want provider-a", d.Provider)
	}
}

func TestPrefixFamilyAffinity_PinsConfiguredChannel(t *testing.T) {
	r := newAffinityRouter(t)
	// First, find which model AUTO actually picks (varies by registry order), then
	// register a channel for that model so the channel-filling code fires.
	probe := r.Decide(providers.ChatCompletionRequest{Model: "AUTO", RouteMode: "auto"})
	if probe.Channel != "" {
		t.Fatal("probe should not pick a channel yet (no channels configured)")
	}
	winnerModel := probe.Model
	winnerProvider := probe.Provider
	r.SetChannels([]Channel{
		{ID: "channel-" + winnerModel, Provider: winnerProvider, Model: winnerModel, Task: "general", Enabled: true, Priority: 1, Weight: 1},
	})
	r.RegisterProductionChannelModel("channel-"+winnerModel, winnerModel)
	r.SetPrefixFamilyAffinity(true)

	req := providers.ChatCompletionRequest{Model: "AUTO", RouteMode: "auto", PrefixFamily: "pf1_channel_sticky"}
	first := r.Decide(req)
	second := r.Decide(req)
	if first.Channel == "" {
		t.Fatalf("first decision for model %s did not select its configured channel", winnerModel)
	}
	if first.Channel != second.Channel {
		t.Fatalf("same prefix family routed to different channels: %s vs %s", first.Channel, second.Channel)
	}
	if first.RouteMode != "prefix_affinity" && second.RouteMode != "prefix_affinity" {
		t.Fatalf("expected one decision to report prefix_affinity, got %q/%q", first.RouteMode, second.RouteMode)
	}
}
