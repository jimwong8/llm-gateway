package configstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// --- 错误定义 ---

var ErrNotFound = errors.New("entity not found")

// --- 实体快照类型 ---

// PresetSnapshot 表示一个 PromptPreset 的快照
type PresetSnapshot struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Template    string    `json:"template"`
	Variables   []string  `json:"variables"`
	Tags        []string  `json:"tags"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MaskSnapshot 表示一个 MaskRule 的快照
type MaskSnapshot struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Pattern   string    `json:"pattern"`
	Replace   string    `json:"replace"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// --- presetStore 接口 ---

// presetStore 定义了底层 preset/mask 存储所需的方法
type presetStore interface {
	CreatePreset(ctx context.Context, userID int64, tenantID, name, description, template string, variables []string, tags []string, isPublic bool) (*PresetSnapshot, error)
	ListPresets(ctx context.Context, userID int64, tenantID string, includePublic bool) ([]PresetSnapshot, error)
	GetPreset(ctx context.Context, presetID int64, tenantID string) (*PresetSnapshot, error)
	UpdatePreset(ctx context.Context, presetID int64, tenantID, name, description, template string, variables []string, tags []string) (*PresetSnapshot, error)
	DeletePreset(ctx context.Context, presetID, userID int64, tenantID string) error
	CreateMaskRule(ctx context.Context, userID int64, tenantID, name, pattern, replace string) (*MaskSnapshot, error)
	ListMaskRules(ctx context.Context, userID int64, tenantID string) ([]MaskSnapshot, error)
	DeleteMaskRule(ctx context.Context, ruleID, userID int64, tenantID string) error
	UpdateMaskRule(ctx context.Context, ruleID, userID int64, tenantID, name, pattern, replace string, enabled bool) error
}

// --- 版本管理类型 ---

// EntityType 表示版本记录关联的实体类型
type EntityType string

const (
	EntityTypePreset EntityType = "preset"
	EntityTypeMask   EntityType = "mask"
)

// Action 表示变更动作类型
type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// VersionRecord 表示一次配置变更的版本记录
type VersionRecord struct {
	ID         int64           `json:"id"`
	EntityType EntityType      `json:"entity_type"`
	EntityID   int64           `json:"entity_id"`
	Action     Action          `json:"action"`
	Snapshot   json.RawMessage `json:"snapshot"`
	ActorID    int64           `json:"actor_id"`
	CreatedAt  time.Time       `json:"created_at"`
}

// VersionedStore 包装 presetStore，记录每次变更的历史版本
type VersionedStore struct {
	base presetStore

	mu      sync.RWMutex
	records []VersionRecord
	nextID  int64
}

// NewVersionedStore 创建一个新的 VersionedStore
func NewVersionedStore(base presetStore) *VersionedStore {
	return &VersionedStore{
		base:   base,
		nextID: 1,
	}
}

// RecordVersion 记录一次配置变更
func (v *VersionedStore) RecordVersion(ctx context.Context, entityType EntityType, entityID int64, action Action, snapshot any, actorID int64) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	record := VersionRecord{
		ID:         v.nextID,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Snapshot:   data,
		ActorID:    actorID,
		CreatedAt:  time.Now().UTC(),
	}
	v.records = append(v.records, record)
	v.nextID++
	return nil
}

// GetHistory 查询指定实体的变更历史（按时间倒序）
func (v *VersionedStore) GetHistory(ctx context.Context, entityType EntityType, entityID int64, limit, offset int) []VersionRecord {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var matched []VersionRecord
	for i := len(v.records) - 1; i >= 0; i-- {
		r := v.records[i]
		if r.EntityType == entityType && r.EntityID == entityID {
			matched = append(matched, r)
		}
	}

	if offset > len(matched) {
		return nil
	}
	start := offset
	end := start + limit
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end]
}

// Rollback 恢复到指定版本的快照
func (v *VersionedStore) Rollback(ctx context.Context, versionID int64, actorID int64) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	var target *VersionRecord
	for i := range v.records {
		if v.records[i].ID == versionID {
			target = &v.records[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("version %d not found", versionID)
	}

	switch target.EntityType {
	case EntityTypePreset:
		return v.rollbackPresetLocked(ctx, target, actorID)
	case EntityTypeMask:
		return v.rollbackMaskLocked(ctx, target, actorID)
	default:
		return fmt.Errorf("unknown entity type: %s", target.EntityType)
	}
}

func (v *VersionedStore) rollbackPresetLocked(ctx context.Context, target *VersionRecord, actorID int64) error {
	var snapshot struct {
		ID          int64    `json:"id"`
		UserID      int64    `json:"user_id"`
		TenantID    string   `json:"tenant_id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Template    string   `json:"template"`
		Variables   []string `json:"variables"`
		Tags        []string `json:"tags"`
		IsPublic    bool     `json:"is_public"`
	}
	if err := json.Unmarshal(target.Snapshot, &snapshot); err != nil {
		return fmt.Errorf("unmarshal preset snapshot: %w", err)
	}

	_, err := v.base.UpdatePreset(ctx, snapshot.ID, snapshot.TenantID, snapshot.Name, snapshot.Description, snapshot.Template, snapshot.Variables, snapshot.Tags)
	if err != nil {
		return fmt.Errorf("rollback preset %d: %w", snapshot.ID, err)
	}

	// 记录回滚操作本身
	record := VersionRecord{
		ID:         v.nextID,
		EntityType: EntityTypePreset,
		EntityID:   snapshot.ID,
		Action:     ActionUpdate,
		Snapshot:   target.Snapshot,
		ActorID:    actorID,
		CreatedAt:  time.Now().UTC(),
	}
	v.records = append(v.records, record)
	v.nextID++
	return nil
}

func (v *VersionedStore) rollbackMaskLocked(ctx context.Context, target *VersionRecord, actorID int64) error {
	var snapshot struct {
		ID       int64  `json:"id"`
		UserID   int64  `json:"user_id"`
		TenantID string `json:"tenant_id"`
		Name     string `json:"name"`
		Pattern  string `json:"pattern"`
		Replace  string `json:"replace"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.Unmarshal(target.Snapshot, &snapshot); err != nil {
		return fmt.Errorf("unmarshal mask snapshot: %w", err)
	}

	err := v.base.UpdateMaskRule(ctx, snapshot.ID, snapshot.UserID, snapshot.TenantID, snapshot.Name, snapshot.Pattern, snapshot.Replace, snapshot.IsActive)
	if err != nil {
		return fmt.Errorf("rollback mask %d: %w", snapshot.ID, err)
	}

	record := VersionRecord{
		ID:         v.nextID,
		EntityType: EntityTypeMask,
		EntityID:   snapshot.ID,
		Action:     ActionUpdate,
		Snapshot:   target.Snapshot,
		ActorID:    actorID,
		CreatedAt:  time.Now().UTC(),
	}
	v.records = append(v.records, record)
	v.nextID++
	return nil
}

// AllRecords 返回所有版本记录（用于测试）
func (v *VersionedStore) AllRecords() []VersionRecord {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make([]VersionRecord, len(v.records))
	copy(result, v.records)
	return result
}

// --- 委托方法：将 presetStore 接口的方法委托给 base，并自动记录版本 ---

func (v *VersionedStore) CreatePreset(ctx context.Context, userID int64, tenantID, name, description, template string, variables []string, tags []string, isPublic bool) (*PresetSnapshot, error) {
	p, err := v.base.CreatePreset(ctx, userID, tenantID, name, description, template, variables, tags, isPublic)
	if err != nil {
		return nil, err
	}
	snap := &PresetSnapshot{
		ID: p.ID, UserID: p.UserID, TenantID: p.TenantID,
		Name: p.Name, Description: p.Description, Template: p.Template,
		Variables: variables, Tags: tags, IsPublic: p.IsPublic,
	}
	_ = v.RecordVersion(ctx, EntityTypePreset, p.ID, ActionCreate, snap, userID)
	return snap, nil
}

func (v *VersionedStore) ListPresets(ctx context.Context, userID int64, tenantID string, includePublic bool) ([]PresetSnapshot, error) {
	return v.base.ListPresets(ctx, userID, tenantID, includePublic)
}

func (v *VersionedStore) GetPreset(ctx context.Context, presetID int64, tenantID string) (*PresetSnapshot, error) {
	return v.base.GetPreset(ctx, presetID, tenantID)
}

func (v *VersionedStore) UpdatePreset(ctx context.Context, presetID int64, tenantID, name, description, template string, variables []string, tags []string) (*PresetSnapshot, error) {
	p, err := v.base.UpdatePreset(ctx, presetID, tenantID, name, description, template, variables, tags)
	if err != nil {
		return nil, err
	}
	snap := &PresetSnapshot{
		ID: p.ID, UserID: p.UserID, TenantID: p.TenantID,
		Name: p.Name, Description: p.Description, Template: p.Template,
		Variables: variables, Tags: tags, IsPublic: p.IsPublic,
	}
	_ = v.RecordVersion(ctx, EntityTypePreset, p.ID, ActionUpdate, snap, p.UserID)
	return snap, nil
}

func (v *VersionedStore) DeletePreset(ctx context.Context, presetID, userID int64, tenantID string) error {
	// 删除前先记录版本
	p, err := v.base.GetPreset(ctx, presetID, tenantID)
	if err == nil {
		snap := &PresetSnapshot{
			ID: p.ID, UserID: p.UserID, TenantID: p.TenantID,
			Name: p.Name, Description: p.Description, Template: p.Template,
			Tags: p.Tags, IsPublic: p.IsPublic,
		}
		_ = v.RecordVersion(ctx, EntityTypePreset, presetID, ActionDelete, snap, userID)
	}
	return v.base.DeletePreset(ctx, presetID, userID, tenantID)
}

func (v *VersionedStore) CreateMaskRule(ctx context.Context, userID int64, tenantID, name, pattern, replace string) (*MaskSnapshot, error) {
	rule, err := v.base.CreateMaskRule(ctx, userID, tenantID, name, pattern, replace)
	if err != nil {
		return nil, err
	}
	snap := &MaskSnapshot{
		ID: rule.ID, UserID: rule.UserID, TenantID: rule.TenantID,
		Name: rule.Name, Pattern: rule.Pattern, Replace: rule.Replace,
		IsActive: rule.IsActive,
	}
	_ = v.RecordVersion(ctx, EntityTypeMask, rule.ID, ActionCreate, snap, userID)
	return snap, nil
}

func (v *VersionedStore) ListMaskRules(ctx context.Context, userID int64, tenantID string) ([]MaskSnapshot, error) {
	return v.base.ListMaskRules(ctx, userID, tenantID)
}

func (v *VersionedStore) DeleteMaskRule(ctx context.Context, ruleID, userID int64, tenantID string) error {
	// 删除前先记录版本
	rules, err := v.base.ListMaskRules(ctx, userID, tenantID)
	if err == nil {
		for _, r := range rules {
			if r.ID == ruleID {
				snap := &MaskSnapshot{
					ID: r.ID, UserID: r.UserID, TenantID: r.TenantID,
					Name: r.Name, Pattern: r.Pattern, Replace: r.Replace,
					IsActive: r.IsActive,
				}
				_ = v.RecordVersion(ctx, EntityTypeMask, ruleID, ActionDelete, snap, userID)
				break
			}
		}
	}
	return v.base.DeleteMaskRule(ctx, ruleID, userID, tenantID)
}

func (v *VersionedStore) UpdateMaskRule(ctx context.Context, ruleID, userID int64, tenantID, name, pattern, replace string, enabled bool) error {
	// 更新前先记录版本（记录更新前的状态）
	rules, err := v.base.ListMaskRules(ctx, userID, tenantID)
	if err == nil {
		for _, r := range rules {
			if r.ID == ruleID {
				snap := &MaskSnapshot{
					ID: r.ID, UserID: r.UserID, TenantID: r.TenantID,
					Name: r.Name, Pattern: r.Pattern, Replace: r.Replace,
					IsActive: r.IsActive,
				}
				_ = v.RecordVersion(ctx, EntityTypeMask, ruleID, ActionUpdate, snap, userID)
				break
			}
		}
	}
	return v.base.UpdateMaskRule(ctx, ruleID, userID, tenantID, name, pattern, replace, enabled)
}
