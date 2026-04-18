package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"llm-gateway/gateway/internal/providers"
)

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedis(addr string, ttl time.Duration) *RedisCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &RedisCache{
		client: redis.NewClient(&redis.Options{Addr: addr}),
		ttl:    ttl,
	}
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisCache) BuildKey(req providers.ChatCompletionRequest) (string, error) {
	normalized := normalizeRequest(req)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal cache key payload: %w", err)
	}
	sum := sha256.Sum256(raw)
	return "chat:l1:" + hex.EncodeToString(sum[:]), nil
}

func (c *RedisCache) Get(ctx context.Context, key string) (*providers.ChatCompletionResponse, bool, error) {
	raw, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var resp providers.ChatCompletionResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, false, err
	}
	return &resp, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, resp *providers.ChatCompletionResponse) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, raw, c.ttl).Err()
}

type normalizedRequest struct {
	Model     string                  `json:"model"`
	TenantID  string                  `json:"tenant_id,omitempty"`
	UserID    string                  `json:"user_id,omitempty"`
	SessionID string                  `json:"session_id,omitempty"`
	TaskHint  string                  `json:"task_hint,omitempty"`
	Messages  []normalizedChatMessage `json:"messages"`
}

type normalizedChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func normalizeRequest(req providers.ChatCompletionRequest) normalizedRequest {
	out := normalizedRequest{
		Model:     strings.TrimSpace(strings.ToLower(req.Model)),
		TenantID:  strings.TrimSpace(strings.ToLower(req.TenantID)),
		UserID:    strings.TrimSpace(strings.ToLower(req.UserID)),
		SessionID: strings.TrimSpace(strings.ToLower(req.SessionID)),
		TaskHint:  strings.TrimSpace(strings.ToLower(req.TaskHint)),
	}
	for _, msg := range req.Messages {
		out.Messages = append(out.Messages, normalizedChatMessage{
			Role:    strings.TrimSpace(strings.ToLower(msg.Role)),
			Content: strings.Join(strings.Fields(msg.Content), " "),
		})
	}
	return out
}
