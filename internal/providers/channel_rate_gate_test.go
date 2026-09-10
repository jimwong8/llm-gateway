package providers

import (
	"context"
	"testing"
	"time"

	"llm-gateway/gateway/internal/config"
)

func TestChannelRateGateLimitsConcurrencyAndKeepsChannelsIndependent(t *testing.T) {
	r := NewRegistry(config.Config{ProviderChannelMaxConcurrency: 1, ProviderChannelMinIntervalMS: 30}, &stubProvider{name: "fallback"})
	p := &stubProvider{name: "provider"}
	r.RegisterChannelModel("key-a", "model-a", p)
	r.RegisterChannelModel("key-b", "model-a", p)

	start := time.Now()
	if err := r.acquireChannel(context.Background(), "key-a"); err != nil {
		t.Fatal(err)
	}
	if err := r.acquireChannel(context.Background(), "key-b"); err != nil {
		t.Fatal(err)
	}
	r.releaseChannel("key-a")
	r.releaseChannel("key-b")
	if time.Since(start) >= 30*time.Millisecond {
		t.Fatal("different channels should not share a global interval gate")
	}

	if err := r.acquireChannel(context.Background(), "key-a"); err != nil {
		t.Fatal(err)
	}
	ready := make(chan error, 1)
	go func() { ready <- r.acquireChannel(context.Background(), "key-a") }()
	select {
	case err := <-ready:
		t.Fatalf("same channel acquired concurrently: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	r.releaseChannel("key-a")
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("same channel did not acquire after release")
	}
	r.releaseChannel("key-a")
}
