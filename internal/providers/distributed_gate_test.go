package providers

import (
	"context"
	"sync"
	"testing"
	"time"

	"llm-gateway/gateway/internal/config"
)

func TestDistributedChannelGateAcrossInstances(t *testing.T) {
	cfg := config.Config{
		RedisAddr:                     "127.0.0.1:6379",
		ProviderChannelMaxConcurrency: 1,
		ProviderChannelMinIntervalMS:  0,
	}
	fallback := &stubProvider{name: "fallback"}
	r1 := NewRegistry(cfg, fallback)
	r2 := NewRegistry(cfg, fallback)
	sharedKey := "nvidia-nim-key-shared-test"
	r1.RegisterChannelModel(sharedKey, "model-x", &stubProvider{name: "key-provider"})
	r2.RegisterChannelModel(sharedKey, "model-x", &stubProvider{name: "key-provider"})

	// Instance 1 acquires; instance 2 must see the gate held.
	ctx := context.Background()
	if err := r1.acquireChannel(ctx, sharedKey); err != nil {
		t.Fatalf("instance 1 acquire: %v", err)
	}

	acquired := make(chan error, 1)
	go func() {
		// Try with short timeout to avoid hanging forever if gate fails.
		ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		acquired <- r2.acquireChannel(ctx2, sharedKey)
	}()

	select {
	case err := <-acquired:
		if err == nil {
			t.Fatal("instance 2 acquired while instance 1 holds the slot")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("instance 2 acquire did not return within timeout")
	}

	// Release from instance 1; instance 2 should now acquire.
	r1.releaseChannel(sharedKey)

	ctx3, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := r2.acquireChannel(ctx3, sharedKey); err != nil {
		t.Fatalf("instance 2 should acquire after release: %v", err)
	}
	r2.releaseChannel(sharedKey)
}

func TestDistributedRoundRobinAcrossInstances(t *testing.T) {
	cfg := config.Config{RedisAddr: "127.0.0.1:6379"}
	r1 := NewRegistry(cfg, &stubProvider{name: "fallback"})
	r2 := NewRegistry(cfg, &stubProvider{name: "fallback"})
	for _, id := range []string{"key-a", "key-b", "key-c"} {
		p := &stubProvider{name: id}
		r1.RegisterChannelModel(id, "model-x", p)
		r2.RegisterChannelModel(id, "model-x", p)
	}

	seen := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := r1
			if idx%2 == 1 {
				r = r2
			}
			id, ok := r.PreferredChannelForModel("model-x")
			if !ok {
				t.Error("missing channel")
				return
			}
			mu.Lock()
			seen[id] = true
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if len(seen) < 2 {
		t.Fatalf("round robin used %d distinct keys across instances, want >=2: %v", len(seen), seen)
	}
}
