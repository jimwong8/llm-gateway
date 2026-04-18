package cache

import (
	"context"
	"testing"
	"time"

	"llm-gateway/gateway/internal/providers"
)

func TestL1Cache_Memory(t *testing.T) {
	c := NewMemoryL1Cache()
	ctx := context.Background()

	req1_key_build := providers.ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		TenantID: "tenant-demo",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "Hello world \n  test  "},
		},
	}
	
	key, err := c.BuildKey(req1_key_build)
	if err != nil {
		t.Fatalf("unexpected error building key: %v", err)
	}
	if key == "" {
		t.Fatal("key should not be empty")
	}

	// 1. Get before Set
	cached, ok, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss, got hit")
	}
	if cached != nil {
		t.Fatal("expected nil cached response")
	}

	// 2. Set
	resp := &providers.ChatCompletionResponse{
		ID: "test-id",
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: "assistant", Content: "Hi!"},
			},
		},
	}
	if err := c.Set(ctx, key, resp); err != nil {
		t.Fatalf("unexpected error on set: %v", err)
	}

	// 3. Get after Set
	cached, ok, err = c.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if cached == nil {
		t.Fatal("expected non-nil cached response")
	}
	if cached.ID != "test-id" {
		t.Errorf("expected cached ID test-id, got %s", cached.ID)
	}

	// 4. Test Normalization (same logic, different whitespace spacing)
	req2 := providers.ChatCompletionRequest{
		Model:    " GPT-4o-MINI ",
		TenantID: "tenant-demo",
		Messages: []providers.ChatMessage{
			{Role: "USER", Content: "Hello world test"},
		},
	}
	
	key2, _ := c.BuildKey(req2)
	if key != key2 {
		t.Errorf("expected normalized key to match. original: %s, normalized: %s", key, key2)
	}
}

func TestL1Cache_Expiration(t *testing.T) {
	c := NewMemoryL1Cache()
	ctx := context.Background()
	key := "test-key"
	
	c.store[key] = cacheItem{
		resp: providers.ChatCompletionResponse{ID: "expired"},
		expiresAt: time.Now().Add(-1 * time.Second), // Already expired
	}
	
	cached, ok, _ := c.Get(ctx, key)
	if ok {
		t.Error("expected miss due to expiration")
	}
	if cached != nil {
		t.Error("expected nil cached response on expiration")
	}
	
	// Ensure cleanup occurred
	if _, exists := c.store[key]; exists {
		t.Error("expected expired key to be deleted from store")
	}
}
