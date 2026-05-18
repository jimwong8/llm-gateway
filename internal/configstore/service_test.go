package configstore

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// --- mock presetStore ---

type mockPresetStore struct {
	presets  map[int64]*PresetSnapshot
	masks    map[int64]*MaskSnapshot
	presetID int64
	maskID   int64
}

func newMockPresetStore() *mockPresetStore {
	return &mockPresetStore{
		presets: make(map[int64]*PresetSnapshot),
		masks:   make(map[int64]*MaskSnapshot),
	}
}

func (m *mockPresetStore) CreatePreset(ctx context.Context, userID int64, tenantID, name, description, template string, variables []string, tags []string, isPublic bool) (*PresetSnapshot, error) {
	m.presetID++
	p := &PresetSnapshot{
		ID: m.presetID, UserID: userID, TenantID: tenantID,
		Name: name, Description: description, Template: template,
		Variables: variables, Tags: tags, IsPublic: isPublic,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	m.presets[p.ID] = p
	return p, nil
}

func (m *mockPresetStore) ListPresets(ctx context.Context, userID int64, tenantID string, includePublic bool) ([]PresetSnapshot, error) {
	var result []PresetSnapshot
	for _, p := range m.presets {
		if p.UserID == userID || (includePublic && p.IsPublic) {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (m *mockPresetStore) GetPreset(ctx context.Context, presetID int64, tenantID string) (*PresetSnapshot, error) {
	p, ok := m.presets[presetID]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (m *mockPresetStore) UpdatePreset(ctx context.Context, presetID int64, tenantID, name, description, template string, variables []string, tags []string) (*PresetSnapshot, error) {
	p, ok := m.presets[presetID]
	if !ok {
		return nil, ErrNotFound
	}
	p.Name = name
	p.Description = description
	p.Template = template
	p.Variables = variables
	p.Tags = tags
	p.UpdatedAt = time.Now().UTC()
	return p, nil
}

func (m *mockPresetStore) DeletePreset(ctx context.Context, presetID, userID int64, tenantID string) error {
	if _, ok := m.presets[presetID]; !ok {
		return ErrNotFound
	}
	delete(m.presets, presetID)
	return nil
}

func (m *mockPresetStore) CreateMaskRule(ctx context.Context, userID int64, tenantID, name, pattern, replace string) (*MaskSnapshot, error) {
	m.maskID++
	r := &MaskSnapshot{
		ID: m.maskID, UserID: userID, TenantID: tenantID,
		Name: name, Pattern: pattern, Replace: replace,
		IsActive: true, CreatedAt: time.Now().UTC(),
	}
	m.masks[r.ID] = r
	return r, nil
}

func (m *mockPresetStore) ListMaskRules(ctx context.Context, userID int64, tenantID string) ([]MaskSnapshot, error) {
	var result []MaskSnapshot
	for _, r := range m.masks {
		if r.UserID == userID {
			result = append(result, *r)
		}
	}
	return result, nil
}

func (m *mockPresetStore) DeleteMaskRule(ctx context.Context, ruleID, userID int64, tenantID string) error {
	if _, ok := m.masks[ruleID]; !ok {
		return ErrNotFound
	}
	delete(m.masks, ruleID)
	return nil
}

func (m *mockPresetStore) UpdateMaskRule(ctx context.Context, ruleID, userID int64, tenantID, name, pattern, replace string, enabled bool) error {
	r, ok := m.masks[ruleID]
	if !ok {
		return ErrNotFound
	}
	r.Name = name
	r.Pattern = pattern
	r.Replace = replace
	r.IsActive = enabled
	return nil
}

// --- tests ---

func TestRecordVersion(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	snap := &PresetSnapshot{ID: 1, Name: "test", Template: "hello"}
	err := vs.RecordVersion(ctx, EntityTypePreset, 1, ActionCreate, snap, 100)
	if err != nil {
		t.Fatalf("RecordVersion failed: %v", err)
	}

	records := vs.AllRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ID != 1 {
		t.Errorf("expected ID=1, got %d", records[0].ID)
	}
	if records[0].EntityType != EntityTypePreset {
		t.Errorf("expected entity_type=preset, got %s", records[0].EntityType)
	}
	if records[0].Action != ActionCreate {
		t.Errorf("expected action=create, got %s", records[0].Action)
	}
	if records[0].ActorID != 100 {
		t.Errorf("expected actor_id=100, got %d", records[0].ActorID)
	}

	// 验证快照可反序列化
	var decoded PresetSnapshot
	if err := json.Unmarshal(records[0].Snapshot, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot failed: %v", err)
	}
	if decoded.Name != "test" {
		t.Errorf("expected snapshot name=test, got %s", decoded.Name)
	}
}

func TestGetHistory(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	// 为 preset 1 创建 3 条版本记录
	for i := 1; i <= 3; i++ {
		snap := &PresetSnapshot{ID: 1, Name: "test", Template: "v"}
		action := ActionUpdate
		if i == 1 {
			action = ActionCreate
		}
		_ = vs.RecordVersion(ctx, EntityTypePreset, 1, action, snap, 100)
	}

	// 为 preset 2 创建 1 条记录
	_ = vs.RecordVersion(ctx, EntityTypePreset, 2, ActionCreate, &PresetSnapshot{ID: 2, Name: "other"}, 100)

	// 查询 preset 1 的历史
	history := vs.GetHistory(ctx, EntityTypePreset, 1, 10, 0)
	if len(history) != 3 {
		t.Fatalf("expected 3 records for preset 1, got %d", len(history))
	}

	// 验证倒序（最新的在前）
	if history[0].ID != 3 {
		t.Errorf("expected first record ID=3, got %d", history[0].ID)
	}
	if history[2].ID != 1 {
		t.Errorf("expected last record ID=1, got %d", history[2].ID)
	}

	// 测试 limit
	history = vs.GetHistory(ctx, EntityTypePreset, 1, 2, 0)
	if len(history) != 2 {
		t.Fatalf("expected 2 records with limit=2, got %d", len(history))
	}

	// 测试 offset
	history = vs.GetHistory(ctx, EntityTypePreset, 1, 10, 2)
	if len(history) != 1 {
		t.Fatalf("expected 1 record with offset=2, got %d", len(history))
	}

	// 测试不存在的实体
	history = vs.GetHistory(ctx, EntityTypePreset, 999, 10, 0)
	if history != nil {
		t.Errorf("expected nil for non-existent entity, got %v", history)
	}
}

func TestRollbackPreset(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	// 创建 preset
	p, err := vs.CreatePreset(ctx, 100, "tenant1", "original", "desc", "template-v1", []string{"var1"}, []string{"tag1"}, false)
	if err != nil {
		t.Fatalf("CreatePreset failed: %v", err)
	}

	// 更新 preset
	_, err = vs.UpdatePreset(ctx, p.ID, "tenant1", "updated", "desc-v2", "template-v2", []string{"var2"}, []string{"tag2"})
	if err != nil {
		t.Fatalf("UpdatePreset failed: %v", err)
	}

	// 获取历史，找到 create 版本
	history := vs.GetHistory(ctx, EntityTypePreset, p.ID, 10, 0)
	if len(history) != 2 {
		t.Fatalf("expected 2 records, got %d", len(history))
	}

	// 找到 create 版本（ID 最小的）
	var createVersionID int64
	for _, r := range history {
		if r.Action == ActionCreate {
			createVersionID = r.ID
			break
		}
	}
	if createVersionID == 0 {
		t.Fatal("create version not found")
	}

	// 回滚到 create 版本
	err = vs.Rollback(ctx, createVersionID, 200)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// 验证回滚后的数据
	rolled, err := vs.GetPreset(ctx, p.ID, "tenant1")
	if err != nil {
		t.Fatalf("GetPreset after rollback failed: %v", err)
	}
	if rolled.Name != "original" {
		t.Errorf("expected name=original after rollback, got %s", rolled.Name)
	}
	if rolled.Template != "template-v1" {
		t.Errorf("expected template=template-v1 after rollback, got %s", rolled.Template)
	}

	// 验证回滚本身也记录了版本
	allRecords := vs.AllRecords()
	if len(allRecords) != 3 {
		t.Errorf("expected 3 total records (create + update + rollback), got %d", len(allRecords))
	}
}

func TestRollbackMask(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	// 创建 mask
	rule, err := vs.CreateMaskRule(ctx, 100, "tenant1", "mask1", "pattern-old", "[OLD]")
	if err != nil {
		t.Fatalf("CreateMaskRule failed: %v", err)
	}

	// 更新 mask
	err = vs.UpdateMaskRule(ctx, rule.ID, 100, "tenant1", "mask1-updated", "pattern-new", "[NEW]", true)
	if err != nil {
		t.Fatalf("UpdateMaskRule failed: %v", err)
	}

	// 获取历史
	history := vs.GetHistory(ctx, EntityTypeMask, rule.ID, 10, 0)
	if len(history) != 2 {
		t.Fatalf("expected 2 records, got %d", len(history))
	}

	// 找到 create 版本
	var createVersionID int64
	for _, r := range history {
		if r.Action == ActionCreate {
			createVersionID = r.ID
			break
		}
	}

	// 回滚
	err = vs.Rollback(ctx, createVersionID, 200)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// 验证回滚后的数据
	rules, err := vs.ListMaskRules(ctx, 100, "tenant1")
	if err != nil {
		t.Fatalf("ListMaskRules failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 mask rule, got %d", len(rules))
	}
	if rules[0].Pattern != "pattern-old" {
		t.Errorf("expected pattern=pattern-old after rollback, got %s", rules[0].Pattern)
	}
	if rules[0].Replace != "[OLD]" {
		t.Errorf("expected replace=[OLD] after rollback, got %s", rules[0].Replace)
	}
}

func TestRollbackVersionNotFound(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	err := vs.Rollback(ctx, 999, 100)
	if err == nil {
		t.Fatal("expected error for non-existent version, got nil")
	}
}

func TestCreatePresetRecordsVersion(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	_, err := vs.CreatePreset(ctx, 100, "tenant1", "test", "desc", "tpl", nil, nil, false)
	if err != nil {
		t.Fatalf("CreatePreset failed: %v", err)
	}

	records := vs.AllRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 version record, got %d", len(records))
	}
	if records[0].Action != ActionCreate {
		t.Errorf("expected action=create, got %s", records[0].Action)
	}
	if records[0].ActorID != 100 {
		t.Errorf("expected actor_id=100, got %d", records[0].ActorID)
	}
}

func TestUpdatePresetRecordsVersion(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	p, _ := vs.CreatePreset(ctx, 100, "tenant1", "test", "desc", "tpl", nil, nil, false)
	_, _ = vs.UpdatePreset(ctx, p.ID, "tenant1", "updated", "desc2", "tpl2", nil, nil)

	records := vs.AllRecords()
	if len(records) != 2 {
		t.Fatalf("expected 2 version records, got %d", len(records))
	}

	// 最新记录应该是 update
	latest := records[len(records)-1]
	if latest.Action != ActionUpdate {
		t.Errorf("expected latest action=update, got %s", latest.Action)
	}
}

func TestDeletePresetRecordsVersion(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	p, _ := vs.CreatePreset(ctx, 100, "tenant1", "test", "desc", "tpl", nil, nil, false)
	_ = vs.DeletePreset(ctx, p.ID, 100, "tenant1")

	records := vs.AllRecords()
	if len(records) != 2 {
		t.Fatalf("expected 2 version records, got %d", len(records))
	}

	// 最新记录应该是 delete
	latest := records[len(records)-1]
	if latest.Action != ActionDelete {
		t.Errorf("expected latest action=delete, got %s", latest.Action)
	}
}

func TestCreateMaskRecordsVersion(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	_, err := vs.CreateMaskRule(ctx, 100, "tenant1", "mask1", "pat", "rep")
	if err != nil {
		t.Fatalf("CreateMaskRule failed: %v", err)
	}

	records := vs.AllRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 version record, got %d", len(records))
	}
	if records[0].EntityType != EntityTypeMask {
		t.Errorf("expected entity_type=mask, got %s", records[0].EntityType)
	}
}

func TestConcurrentRecordVersion(t *testing.T) {
	base := newMockPresetStore()
	vs := NewVersionedStore(base)
	ctx := context.Background()

	// 并发写入 100 条记录
	done := make(chan struct{}, 100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			snap := &PresetSnapshot{ID: int64(i), Name: "concurrent"}
			_ = vs.RecordVersion(ctx, EntityTypePreset, int64(i), ActionCreate, snap, int64(i))
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	records := vs.AllRecords()
	if len(records) != 100 {
		t.Fatalf("expected 100 records, got %d", len(records))
	}

	// 验证 ID 唯一且连续
	seen := make(map[int64]bool)
	for _, r := range records {
		if seen[r.ID] {
			t.Errorf("duplicate record ID: %d", r.ID)
		}
		seen[r.ID] = true
	}
}
