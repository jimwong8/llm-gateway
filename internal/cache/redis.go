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

const (
	conversationMetaKeyPattern   = "conv:%s:meta"
	conversationRecentKeyPattern = "conv:%s:recent"
	conversationCacheTTL         = 24 * time.Hour
)

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

type ConversationMeta struct {
	LastSeq   int64     `json:"last_seq"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RecentMessage struct {
	Seq     int64  `json:"seq"`
	Role    string `json:"role"`
	Content string `json:"content"`
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

func (c *RedisCache) CacheConversationMeta(ctx context.Context, conversationID string, meta ConversationMeta) error {
	convID := strings.TrimSpace(conversationID)
	if convID == "" {
		return fmt.Errorf("conversationID is required")
	}
	pipe := c.client.TxPipeline()
	metaKey := c.conversationMetaKey(convID)

	pipe.HSet(ctx, metaKey,
		"last_seq", meta.LastSeq,
		"updated_at", meta.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	pipe.Expire(ctx, metaKey, conversationCacheTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) GetConversationMeta(ctx context.Context, conversationID string) (*ConversationMeta, bool, error) {
	convID := strings.TrimSpace(conversationID)
	if convID == "" {
		return nil, false, fmt.Errorf("conversationID is required")
	}
	metaKey := c.conversationMetaKey(convID)
	values, err := c.client.HGetAll(ctx, metaKey).Result()
	if err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, false, nil
	}

	lastSeq, err := c.client.HGet(ctx, metaKey, "last_seq").Int64()
	if err != nil {
		return nil, false, err
	}
	updatedAtRaw, ok := values["updated_at"]
	if !ok {
		return nil, false, fmt.Errorf("missing updated_at in meta")
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtRaw)
	if err != nil {
		return nil, false, fmt.Errorf("parse updated_at: %w", err)
	}

	return &ConversationMeta{
		LastSeq:   lastSeq,
		UpdatedAt: updatedAt,
	}, true, nil
}

func (c *RedisCache) CacheRecentMessages(ctx context.Context, conversationID string, messages []RecentMessage, maxItems int64) error {
	convID := strings.TrimSpace(conversationID)
	if convID == "" {
		return fmt.Errorf("conversationID is required")
	}
	if len(messages) == 0 {
		return nil
	}
	if maxItems <= 0 {
		maxItems = 50
	}
	recentKey := c.conversationRecentKey(convID)
	pipe := c.client.TxPipeline()

	for _, msg := range messages {
		raw, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal recent message: %w", err)
		}
		pipe.RPush(ctx, recentKey, raw)
	}
	pipe.LTrim(ctx, recentKey, -maxItems, -1)
	pipe.Expire(ctx, recentKey, conversationCacheTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) GetRecentMessages(ctx context.Context, conversationID string, limit int64) ([]RecentMessage, error) {
	convID := strings.TrimSpace(conversationID)
	if convID == "" {
		return nil, fmt.Errorf("conversationID is required")
	}
	if limit <= 0 {
		return []RecentMessage{}, nil
	}
	recentKey := c.conversationRecentKey(convID)
	rawItems, err := c.client.LRange(ctx, recentKey, -limit, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return []RecentMessage{}, nil
		}
		return nil, err
	}

	items := make([]RecentMessage, 0, len(rawItems))
	for _, raw := range rawItems {
		var item RecentMessage
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("unmarshal recent message: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *RedisCache) InvalidateConversationCache(ctx context.Context, conversationID string) error {
	convID := strings.TrimSpace(conversationID)
	if convID == "" {
		return fmt.Errorf("conversationID is required")
	}
	return c.client.Del(ctx, c.conversationMetaKey(convID), c.conversationRecentKey(convID)).Err()
}

func (c *RedisCache) conversationMetaKey(conversationID string) string {
	return fmt.Sprintf(conversationMetaKeyPattern, conversationID)
}

func (c *RedisCache) conversationRecentKey(conversationID string) string {
	return fmt.Sprintf(conversationRecentKeyPattern, conversationID)
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
