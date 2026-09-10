package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PromptPreset struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Template    string    `json:"template"`
	Variables   string    `json:"variables"`
	Tags        []string  `json:"tags"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MaskRule struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Pattern   string    `json:"pattern"`
	Replace   string    `json:"replace"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type PresetStore interface {
	CreatePreset(ctx context.Context, userID int64, tenantID, name, description, template string, variables []string, tags []string, isPublic bool) (*PromptPreset, error)
	ListPresets(ctx context.Context, userID int64, tenantID string, includePublic bool) ([]PromptPreset, error)
	GetPreset(ctx context.Context, presetID int64, tenantID string) (*PromptPreset, error)
	UpdatePreset(ctx context.Context, presetID int64, tenantID, name, description, template string, variables []string, tags []string) (*PromptPreset, error)
	DeletePreset(ctx context.Context, presetID, userID int64, tenantID string) error
	CreateMaskRule(ctx context.Context, userID int64, tenantID, name, pattern, replace string) (*MaskRule, error)
	ListMaskRules(ctx context.Context, userID int64, tenantID string) ([]MaskRule, error)
	DeleteMaskRule(ctx context.Context, ruleID, userID int64, tenantID string) error
	UpdateMaskRule(ctx context.Context, ruleID, userID int64, tenantID, name, pattern, replace string, enabled bool) error
	ApplyMasks(ctx context.Context, userID int64, tenantID, text string) (string, error)
}

type sqlPresetStore struct {
	db *sql.DB
}

func NewPresetStore(db *sql.DB) PresetStore {
	return &sqlPresetStore{db: db}
}

func (s *sqlPresetStore) CreatePreset(ctx context.Context, userID int64, tenantID, name, description, template string, variables []string, tags []string, isPublic bool) (*PromptPreset, error) {
	varsJSON, _ := json.Marshal(variables)
	var p PromptPreset
	err := s.db.QueryRowContext(ctx, `
INSERT INTO prompt_presets (user_id, tenant_id, name, description, template, variables, tags, is_public)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
RETURNING id, user_id, name, description, template, variables::text, tags, is_public, created_at, updated_at`,
		userID, tenantID, name, description, template, varsJSON, pqArray(tags), isPublic,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Template, &p.Variables, pqArrayScanner(&p.Tags), &p.IsPublic, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create preset: %w", err)
	}
	return &p, nil
}

func (s *sqlPresetStore) ListPresets(ctx context.Context, userID int64, tenantID string, includePublic bool) ([]PromptPreset, error) {
	var query string
	var args []interface{}
	if includePublic {
		query = `SELECT id, user_id, name, description, template, variables::text, tags, is_public, created_at, updated_at
FROM prompt_presets WHERE user_id = $1 OR is_public = TRUE ORDER BY created_at DESC`
		args = []interface{}{userID}
	} else {
		query = `SELECT id, user_id, name, description, template, variables::text, tags, is_public, created_at, updated_at
FROM prompt_presets WHERE user_id = $1 ORDER BY created_at DESC`
		args = []interface{}{userID}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPresets(rows)
}

func (s *sqlPresetStore) GetPreset(ctx context.Context, presetID int64, tenantID string) (*PromptPreset, error) {
	var p PromptPreset
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, name, description, template, variables::text, tags, is_public, created_at, updated_at
FROM prompt_presets WHERE id = $1`, presetID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Template, &p.Variables, pqArrayScanner(&p.Tags), &p.IsPublic, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *sqlPresetStore) UpdatePreset(ctx context.Context, presetID int64, tenantID, name, description, template string, variables []string, tags []string) (*PromptPreset, error) {
	varsJSON, _ := json.Marshal(variables)
	var p PromptPreset
	err := s.db.QueryRowContext(ctx, `
UPDATE prompt_presets SET name=$2, description=$3, template=$4, variables=$5::jsonb, tags=$6, updated_at=NOW()
WHERE id=$1
RETURNING id, user_id, name, description, template, variables::text, tags, is_public, created_at, updated_at`,
		presetID, name, description, template, varsJSON, pqArray(tags),
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Template, &p.Variables, pqArrayScanner(&p.Tags), &p.IsPublic, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update preset: %w", err)
	}
	return &p, nil
}

func (s *sqlPresetStore) DeletePreset(ctx context.Context, presetID, userID int64, tenantID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM prompt_presets WHERE id=$1 AND user_id=$2`, presetID, userID)
	return err
}

func (s *sqlPresetStore) CreateMaskRule(ctx context.Context, userID int64, tenantID, name, pattern, replace string) (*MaskRule, error) {
	var r MaskRule
	err := s.db.QueryRowContext(ctx, `
INSERT INTO mask_rules (user_id, name, pattern, replace_with)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, name, pattern, replace_with, is_active, created_at`,
		userID, name, pattern, replace,
	).Scan(&r.ID, &r.UserID, &r.Name, &r.Pattern, &r.Replace, &r.IsActive, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create mask rule: %w", err)
	}
	return &r, nil
}

func (s *sqlPresetStore) ListMaskRules(ctx context.Context, userID int64, tenantID string) ([]MaskRule, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, name, pattern, replace_with, is_active, created_at
FROM mask_rules WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []MaskRule
	for rows.Next() {
		var r MaskRule
		if err := rows.Scan(&r.ID, &r.UserID, &r.Name, &r.Pattern, &r.Replace, &r.IsActive, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *sqlPresetStore) DeleteMaskRule(ctx context.Context, ruleID, userID int64, tenantID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM mask_rules WHERE id=$1 AND user_id=$2`, ruleID, userID)
	return err
}

func (s *sqlPresetStore) UpdateMaskRule(ctx context.Context, ruleID, userID int64, tenantID, name, pattern, replace string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE mask_rules SET name=$3, pattern=$4, replace_with=$5, is_active=$6, updated_at=NOW()
WHERE id=$1 AND user_id=$2`, ruleID, userID, name, pattern, replace, enabled)
	return err
}

func (s *sqlPresetStore) ApplyMasks(ctx context.Context, userID int64, tenantID, text string) (string, error) {
	rules, err := s.ListMaskRules(ctx, userID, tenantID)
	if err != nil {
		return text, err
	}
	return applyMaskRules(text, rules), nil
}

func applyMaskRules(text string, rules []MaskRule) string {
	result := text
	for _, r := range rules {
		result = strings.ReplaceAll(result, r.Pattern, r.Replace)
	}
	return result
}

func scanPresets(rows *sql.Rows) ([]PromptPreset, error) {
	var presets []PromptPreset
	for rows.Next() {
		var p PromptPreset
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Template, &p.Variables, pqArrayScanner(&p.Tags), &p.IsPublic, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		presets = append(presets, p)
	}
	return presets, rows.Err()
}
