package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"llm-gateway/gateway/internal/memory"
)

// presetCacheTTL 是 preset/mask 数据的缓存时间
const presetCacheTTL = 60 * time.Second

// cachedPresetStore 是 presetStore 的缓存装饰器
type cachedPresetStore struct {
	base presetStore

	mu    sync.RWMutex
	items []cacheItem
}

type cacheItem struct {
	key       string
	data      []byte
	expiresAt time.Time
}

func newCachedPresetStore(base presetStore) *cachedPresetStore {
	return &cachedPresetStore{base: base}
}

func (c *cachedPresetStore) get(key string, dest any) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, item := range c.items {
		if item.key == key && time.Now().Before(item.expiresAt) {
			return json.Unmarshal(item.data, dest) == nil
		}
	}
	return false
}

func (c *cachedPresetStore) set(key string, src any) {
	data, err := json.Marshal(src)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.upsertLocked(key, data)
}

// invalidateByPrefix 删除所有匹配前缀的缓存 key
func (c *cachedPresetStore) invalidateByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	filtered := c.items[:0]
	for _, item := range c.items {
		if len(item.key) < len(prefix) || item.key[:len(prefix)] != prefix {
			filtered = append(filtered, item)
		}
	}
	c.items = filtered
}

func (c *cachedPresetStore) upsertLocked(key string, data []byte) {
	for i := range c.items {
		if c.items[i].key == key {
			c.items[i].data = data
			c.items[i].expiresAt = time.Now().Add(presetCacheTTL)
			return
		}
	}
	c.items = append(c.items, cacheItem{
		key:       key,
		data:      data,
		expiresAt: time.Now().Add(presetCacheTTL),
	})
}

func (c *cachedPresetStore) CreatePreset(ctx context.Context, userID int64, tenantID, name, description, template string, variables []string, tags []string, isPublic bool) (*memory.PromptPreset, error) {
	p, err := c.base.CreatePreset(ctx, userID, tenantID, name, description, template, variables, tags, isPublic)
	if err != nil {
		return nil, err
	}
	c.invalidateByPrefix(fmt.Sprintf("preset:%d:", userID))
	return p, nil
}

func (c *cachedPresetStore) ListPresets(ctx context.Context, userID int64, tenantID string, includePublic bool) ([]memory.PromptPreset, error) {
	key := fmt.Sprintf("preset:%d:list", userID)
	var cached []memory.PromptPreset
	if c.get(key, &cached) {
		return cached, nil
	}
	presets, err := c.base.ListPresets(ctx, userID, tenantID, includePublic)
	if err != nil {
		return nil, err
	}
	c.set(key, presets)
	return presets, nil
}

func (c *cachedPresetStore) GetPreset(ctx context.Context, presetID int64, tenantID string) (*memory.PromptPreset, error) {
	key := fmt.Sprintf("preset:%d", presetID)
	var cached memory.PromptPreset
	if c.get(key, &cached) {
		return &cached, nil
	}
	p, err := c.base.GetPreset(ctx, presetID, tenantID)
	if err != nil {
		return nil, err
	}
	c.set(key, *p)
	return p, nil
}

func (c *cachedPresetStore) UpdatePreset(ctx context.Context, presetID int64, tenantID, name, description, template string, variables []string, tags []string) (*memory.PromptPreset, error) {
	p, err := c.base.UpdatePreset(ctx, presetID, tenantID, name, description, template, variables, tags)
	if err != nil {
		return nil, err
	}
	// 更新单条缓存
	c.set(fmt.Sprintf("preset:%d", presetID), *p)
	// 失效该用户的所有列表缓存
	c.invalidateByPrefix(fmt.Sprintf("preset:%d:", p.UserID))
	return p, nil
}

func (c *cachedPresetStore) DeletePreset(ctx context.Context, presetID, userID int64, tenantID string) error {
	if err := c.base.DeletePreset(ctx, presetID, userID, tenantID); err != nil {
		return err
	}
	c.invalidateByPrefix(fmt.Sprintf("preset:%d:", userID))
	c.invalidateByPrefix(fmt.Sprintf("preset:%d", presetID))
	return nil
}

func (c *cachedPresetStore) CreateMaskRule(ctx context.Context, userID int64, tenantID, name, pattern, replace string) (*memory.MaskRule, error) {
	rule, err := c.base.CreateMaskRule(ctx, userID, tenantID, name, pattern, replace)
	if err != nil {
		return nil, err
	}
	c.invalidateByPrefix(fmt.Sprintf("mask:%d:", userID))
	return rule, nil
}

func (c *cachedPresetStore) ListMaskRules(ctx context.Context, userID int64, tenantID string) ([]memory.MaskRule, error) {
	key := fmt.Sprintf("mask:%d:list", userID)
	var cached []memory.MaskRule
	if c.get(key, &cached) {
		return cached, nil
	}
	rules, err := c.base.ListMaskRules(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}
	c.set(key, rules)
	return rules, nil
}

func (c *cachedPresetStore) DeleteMaskRule(ctx context.Context, ruleID, userID int64, tenantID string) error {
	if err := c.base.DeleteMaskRule(ctx, ruleID, userID, tenantID); err != nil {
		return err
	}
	c.invalidateByPrefix(fmt.Sprintf("mask:%d:", userID))
	c.invalidateByPrefix(fmt.Sprintf("mask:%d", ruleID))
	return nil
}

func (c *cachedPresetStore) UpdateMaskRule(ctx context.Context, ruleID, userID int64, tenantID, name, pattern, replace string, enabled bool) error {
	if err := c.base.UpdateMaskRule(ctx, ruleID, userID, tenantID, name, pattern, replace, enabled); err != nil {
		return err
	}
	c.invalidateByPrefix(fmt.Sprintf("mask:%d:", userID))
	c.invalidateByPrefix(fmt.Sprintf("mask:%d", ruleID))
	return nil
}
