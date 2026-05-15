package cache

import (
	"context"
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestBuildKey(t *testing.T) {
	c := &RedisCache{}

	req := providers.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "Hello World"},
		},
	}

	key, err := c.BuildKey(req)
	if err != nil {
		t.Fatalf("BuildKey() error: %v", err)
	}
	if key == "" {
		t.Fatal("BuildKey() returned empty string")
	}
	if len(key) != 72 {
		t.Errorf("BuildKey() length = %d, want 72", len(key))
	}
}

func TestBuildKeyDeterministic(t *testing.T) {
	c := &RedisCache{}

	req := providers.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "test message"},
		},
	}

	key1, _ := c.BuildKey(req)
	key2, _ := c.BuildKey(req)
	if key1 != key2 {
		t.Errorf("BuildKey() not deterministic: %q vs %q", key1, key2)
	}
}

func TestBuildKeyDifferentInputs(t *testing.T) {
	c := &RedisCache{}

	req1 := providers.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}},
	}
	req2 := providers.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []providers.ChatMessage{{Role: "user", Content: "world"}},
	}

	key1, _ := c.BuildKey(req1)
	key2, _ := c.BuildKey(req2)
	if key1 == key2 {
		t.Error("BuildKey() should produce different keys for different messages")
	}
}

func TestBuildKeyNormalization(t *testing.T) {
	c := &RedisCache{}

	req1 := providers.ChatCompletionRequest{
		Model:    "GPT-4O-MINI",
		Messages: []providers.ChatMessage{{Role: "User", Content: "hello world"}},
	}
	req2 := providers.ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello world"}},
	}

	key1, _ := c.BuildKey(req1)
	key2, _ := c.BuildKey(req2)
	if key1 != key2 {
		t.Errorf("BuildKey() normalization failed: %q vs %q", key1, key2)
	}
}

func TestBuildKeyWithOptionalFields(t *testing.T) {
	c := &RedisCache{}

	req := providers.ChatCompletionRequest{
		Model:     "gpt-4o",
		TenantID:  "tenant-1",
		UserID:    "user-1",
		SessionID: "session-1",
		TaskHint:  "code",
		Messages:  []providers.ChatMessage{{Role: "user", Content: "test"}},
	}

	key, err := c.BuildKey(req)
	if err != nil {
		t.Fatalf("BuildKey() error: %v", err)
	}
	if key == "" {
		t.Fatal("BuildKey() returned empty string")
	}
}

func TestBuildKeyEmptyMessages(t *testing.T) {
	c := &RedisCache{}

	req := providers.ChatCompletionRequest{
		Model: "gpt-4o",
	}

	key, err := c.BuildKey(req)
	if err != nil {
		t.Fatalf("BuildKey() error: %v", err)
	}
	if key == "" {
		t.Fatal("BuildKey() returned empty string for empty messages")
	}
}

func TestConversationMetaKey(t *testing.T) {
	c := &RedisCache{}
	key := c.conversationMetaKey("conv-123")
	if key != "conv:conv-123:meta" {
		t.Errorf("conversationMetaKey() = %q, want %q", key, "conv:conv-123:meta")
	}
}

func TestConversationRecentKey(t *testing.T) {
	c := &RedisCache{}
	key := c.conversationRecentKey("conv-123")
	if key != "conv:conv-123:recent" {
		t.Errorf("conversationRecentKey() = %q, want %q", key, "conv:conv-123:recent")
	}
}

func TestInvalidateConversationCacheEmptyID(t *testing.T) {
	c := &RedisCache{}
	err := c.InvalidateConversationCache(context.Background(), "")
	if err == nil {
		t.Error("InvalidateConversationCache('') should return error")
	}

	err = c.InvalidateConversationCache(context.Background(), "   ")
	if err == nil {
		t.Error("InvalidateConversationCache('   ') should return error")
	}
}

func TestCacheConversationMetaEmptyID(t *testing.T) {
	c := &RedisCache{}
	err := c.CacheConversationMeta(context.Background(), "", ConversationMeta{})
	if err == nil {
		t.Error("CacheConversationMeta('') should return error")
	}
}

func TestCacheRecentMessagesEmptyID(t *testing.T) {
	c := &RedisCache{}
	err := c.CacheRecentMessages(context.Background(), "", []RecentMessage{}, 10)
	if err == nil {
		t.Error("CacheRecentMessages('') should return error")
	}
}

func TestCacheRecentMessagesEmptySlice(t *testing.T) {
	c := &RedisCache{}
	err := c.CacheRecentMessages(context.Background(), "conv-1", []RecentMessage{}, 10)
	if err != nil {
		t.Errorf("CacheRecentMessages with empty slice should not error: %v", err)
	}
}

func TestGetRecentMessagesEmptyID(t *testing.T) {
	c := &RedisCache{}
	_, err := c.GetRecentMessages(context.Background(), "", 10)
	if err == nil {
		t.Error("GetRecentMessages('') should return error")
	}
}

func TestGetRecentMessagesZeroLimit(t *testing.T) {
	c := &RedisCache{}
	msgs, err := c.GetRecentMessages(context.Background(), "conv-1", 0)
	if err != nil {
		t.Errorf("GetRecentMessages with limit=0 should not error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("GetRecentMessages with limit=0 should return empty slice, got %d", len(msgs))
	}
}

func TestNewRedisDefaultTTL(t *testing.T) {
	c := NewRedis("localhost:6379", 0)
	if c.ttl <= 0 {
		t.Error("NewRedis with ttl=0 should set default TTL")
	}
	if c.client == nil {
		t.Fatal("NewRedis should initialize client")
	}
}

func TestNewRedisNegativeTTL(t *testing.T) {
	c := NewRedis("localhost:6379", -1)
	if c.ttl <= 0 {
		t.Error("NewRedis with negative ttl should set default TTL")
	}
}
