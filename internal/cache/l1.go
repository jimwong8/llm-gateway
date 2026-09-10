package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"llm-gateway/gateway/internal/providers"
)

// L1Cache 定义了精确匹配缓存接口
type L1Cache interface {
	Get(ctx context.Context, key string) (*providers.ChatCompletionResponse, bool, error)
	Set(ctx context.Context, key string, resp *providers.ChatCompletionResponse) error
	BuildKey(req providers.ChatCompletionRequest) (string, error)
}

// NormalizedRequest 是用于计算 L1 Cache Key 的规范化结构，剥离了一些非稳定因素。
type NormalizedRequest struct {
	Model       string                  `json:"model"`
	TenantID    string                  `json:"tenant_id,omitempty"` // 可选：如果同一租户才共享 L1 缓存，则加入；如果全局复用则置空
	Messages    []normalizedChatMessage `json:"messages"`
}

// MemoryL1Cache 实现了简单的内存 L1 缓存 (仅用于演示/本地测试/无 Redis 场景兜底)
type MemoryL1Cache struct {
	store map[string]cacheItem
}

type cacheItem struct {
	resp      providers.ChatCompletionResponse
	expiresAt time.Time
}

func NewMemoryL1Cache() *MemoryL1Cache {
	return &MemoryL1Cache{
		store: make(map[string]cacheItem),
	}
}

func (c *MemoryL1Cache) Get(ctx context.Context, key string) (*providers.ChatCompletionResponse, bool, error) {
	item, ok := c.store[key]
	if !ok {
		return nil, false, nil
	}
	if time.Now().After(item.expiresAt) {
		delete(c.store, key)
		return nil, false, nil
	}
	// 返回副本
	respCopy := item.resp
	return &respCopy, true, nil
}

func (c *MemoryL1Cache) Set(ctx context.Context, key string, resp *providers.ChatCompletionResponse) error {
	if resp == nil {
		return nil
	}
	c.store[key] = cacheItem{
		resp:      *resp,
		expiresAt: time.Now().Add(10 * time.Minute), // 硬编码 10 分钟 TTL 示例
	}
	return nil
}

func (c *MemoryL1Cache) BuildKey(req providers.ChatCompletionRequest) (string, error) {
	return buildL1Key(req)
}

func buildL1Key(req providers.ChatCompletionRequest) (string, error) {
	out := NormalizedRequest{
		Model:       strings.TrimSpace(strings.ToLower(req.Model)),
		TenantID:    strings.TrimSpace(strings.ToLower(req.TenantID)),
	}
	
	// Message 内容去重与格式规范化 (去除多余空格/回车)
	for _, msg := range req.Messages {
		out.Messages = append(out.Messages, normalizedChatMessage{
			Role:    strings.TrimSpace(strings.ToLower(msg.Role)),
			Content: strings.Join(strings.Fields(msg.Content), " "), // 折叠多余空白
		})
	}
	
	raw, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshal cache key payload: %w", err)
	}
	sum := sha256.Sum256(raw)
	return "chat:l1:exact:" + hex.EncodeToString(sum[:]), nil
}
