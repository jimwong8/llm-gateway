package policy

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"

	_ "github.com/lib/pq"
)

type TenantModelPolicy struct {
	TenantID string `json:"tenant_id"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
}

type TenantRoleBinding struct {
	TenantID string `json:"tenant_id"`
	Subject  string `json:"subject"`
	Role     string `json:"role"`
}

type TenantProviderPolicy struct {
	TenantID string `json:"tenant_id"`
	Provider string `json:"provider"`
	Mode     string `json:"mode"`
	Enabled  bool   `json:"enabled"`
}

type SensitiveRule struct {
	TenantID string `json:"tenant_id"`
	Pattern  string `json:"pattern"`
	Action   string `json:"action"`
	Enabled  bool   `json:"enabled"`
}

type Store struct {
	db *sql.DB

	overlayMu               sync.RWMutex
	allowedModelsOverlay    map[string][]string
	roleBindingsOverlay     map[string]map[string]string
	providerPoliciesOverlay map[string][]TenantProviderPolicy
	sensitiveRulesOverlay   map[string][]SensitiveRule
}

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS tenant_model_policies (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    model TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, model)
);
CREATE TABLE IF NOT EXISTS tenant_role_bindings (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, subject)
);
CREATE TABLE IF NOT EXISTS tenant_provider_policies (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    mode TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, provider)
);
CREATE TABLE IF NOT EXISTS tenant_sensitive_rules (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    pattern TEXT NOT NULL,
    action TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, pattern)
);
`)
	return err
}

func (s *Store) AllowedModels(ctx context.Context, tenantID string) ([]string, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, nil
	}

	if models, ok := s.allowedModelsOverlayForTenant(tenantID); ok {
		return models, nil
	}

	rows, err := s.db.QueryContext(ctx, `SELECT model FROM tenant_model_policies WHERE tenant_id = $1 AND enabled = TRUE ORDER BY model`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var model string
		if err := rows.Scan(&model); err != nil {
			return nil, err
		}
		out = append(out, model)
	}
	return out, rows.Err()
}

func (s *Store) SetAllowedModelsOverlay(tenantID string, models []string) {
	if s == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	normalized := normalizeAllowedModels(models)
	s.overlayMu.Lock()
	if s.allowedModelsOverlay == nil {
		s.allowedModelsOverlay = map[string][]string{}
	}
	s.allowedModelsOverlay[tenantID] = normalized
	s.overlayMu.Unlock()
}

func (s *Store) allowedModelsOverlayForTenant(tenantID string) ([]string, bool) {
	if s == nil {
		return nil, false
	}
	s.overlayMu.RLock()
	models, ok := s.allowedModelsOverlay[tenantID]
	s.overlayMu.RUnlock()
	if !ok {
		return nil, false
	}
	out := make([]string, len(models))
	copy(out, models)
	return out, true
}

func normalizeAllowedModels(models []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	return out
}

func (s *Store) SetRoleBindingsOverlay(tenantID string, bindings []TenantRoleBinding) {
	if s == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	normalized := normalizeRoleBindings(bindings)
	s.overlayMu.Lock()
	if s.roleBindingsOverlay == nil {
		s.roleBindingsOverlay = map[string]map[string]string{}
	}
	s.roleBindingsOverlay[tenantID] = normalized
	s.overlayMu.Unlock()
}

func (s *Store) roleBindingOverlayForTenant(tenantID, subject string) (string, bool) {
	if s == nil {
		return "", false
	}
	tenantID = strings.TrimSpace(tenantID)
	subject = strings.TrimSpace(subject)
	if tenantID == "" || subject == "" {
		return "", false
	}

	s.overlayMu.RLock()
	bindings, ok := s.roleBindingsOverlay[tenantID]
	s.overlayMu.RUnlock()
	if !ok {
		return "", false
	}
	role, ok := bindings[subject]
	if !ok {
		return "", false
	}
	return role, true
}

func normalizeRoleBindings(bindings []TenantRoleBinding) map[string]string {
	out := map[string]string{}
	for _, item := range bindings {
		subject := strings.TrimSpace(item.Subject)
		role := strings.TrimSpace(strings.ToLower(item.Role))
		if subject == "" {
			continue
		}
		if role != "admin" && role != "operator" && role != "readonly" {
			continue
		}
		out[subject] = role
	}
	return out
}

func (s *Store) SetProviderPoliciesOverlay(tenantID string, policies []TenantProviderPolicy) {
	if s == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	normalized := normalizeProviderPolicies(tenantID, policies)
	s.overlayMu.Lock()
	if s.providerPoliciesOverlay == nil {
		s.providerPoliciesOverlay = map[string][]TenantProviderPolicy{}
	}
	s.providerPoliciesOverlay[tenantID] = normalized
	s.overlayMu.Unlock()
}

func (s *Store) providerPoliciesOverlayForTenant(tenantID string) ([]TenantProviderPolicy, bool) {
	if s == nil {
		return nil, false
	}
	s.overlayMu.RLock()
	policies, ok := s.providerPoliciesOverlay[tenantID]
	s.overlayMu.RUnlock()
	if !ok {
		return nil, false
	}
	out := make([]TenantProviderPolicy, len(policies))
	copy(out, policies)
	return out, true
}

func normalizeProviderPolicies(tenantID string, policies []TenantProviderPolicy) []TenantProviderPolicy {
	out := make([]TenantProviderPolicy, 0, len(policies))
	indexByProvider := map[string]int{}
	for _, item := range policies {
		provider := strings.TrimSpace(strings.ToLower(item.Provider))
		mode := strings.TrimSpace(strings.ToLower(item.Mode))
		if provider == "" {
			continue
		}
		if mode != "allow" && mode != "deny" {
			continue
		}
		normalized := TenantProviderPolicy{
			TenantID: tenantID,
			Provider: provider,
			Mode:     mode,
			Enabled:  item.Enabled,
		}
		if idx, ok := indexByProvider[provider]; ok {
			out[idx] = normalized
			continue
		}
		indexByProvider[provider] = len(out)
		out = append(out, normalized)
	}
	return out
}

func (s *Store) SetSensitiveRulesOverlay(tenantID string, rules []SensitiveRule) {
	if s == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	normalized := normalizeSensitiveRules(tenantID, rules)
	s.overlayMu.Lock()
	if s.sensitiveRulesOverlay == nil {
		s.sensitiveRulesOverlay = map[string][]SensitiveRule{}
	}
	s.sensitiveRulesOverlay[tenantID] = normalized
	s.overlayMu.Unlock()
}

func (s *Store) sensitiveRulesOverlayForTenant(tenantID string) ([]SensitiveRule, bool) {
	if s == nil {
		return nil, false
	}
	s.overlayMu.RLock()
	rules, ok := s.sensitiveRulesOverlay[tenantID]
	s.overlayMu.RUnlock()
	if !ok {
		return nil, false
	}
	out := make([]SensitiveRule, len(rules))
	copy(out, rules)
	return out, true
}

func normalizeSensitiveRules(tenantID string, rules []SensitiveRule) []SensitiveRule {
	out := make([]SensitiveRule, 0, len(rules))
	indexByPattern := map[string]int{}
	for _, item := range rules {
		pattern := strings.TrimSpace(item.Pattern)
		action := strings.TrimSpace(strings.ToLower(item.Action))
		if pattern == "" {
			continue
		}
		if action != "block" {
			continue
		}
		normalized := SensitiveRule{
			TenantID: tenantID,
			Pattern:  pattern,
			Action:   action,
			Enabled:  item.Enabled,
		}
		if idx, ok := indexByPattern[pattern]; ok {
			out[idx] = normalized
			continue
		}
		indexByPattern[pattern] = len(out)
		out = append(out, normalized)
	}
	return out
}

func (s *Store) Upsert(ctx context.Context, tenantID string, model string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO tenant_model_policies (tenant_id, model, enabled)
VALUES ($1,$2,$3)
ON CONFLICT (tenant_id, model)
DO UPDATE SET enabled = EXCLUDED.enabled
`, tenantID, model, enabled)
	return err
}

func (s *Store) UpsertRole(ctx context.Context, tenantID, subject, role string) error {
	role = strings.TrimSpace(strings.ToLower(role))
	if role != "admin" && role != "operator" && role != "readonly" {
		return errors.New("invalid role")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO tenant_role_bindings (tenant_id, subject, role)
VALUES ($1,$2,$3)
ON CONFLICT (tenant_id, subject)
DO UPDATE SET role = EXCLUDED.role
`, tenantID, subject, role)
	return err
}

func (s *Store) RoleFor(ctx context.Context, tenantID, subject string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	subject = strings.TrimSpace(subject)
	if tenantID == "" || subject == "" {
		return "", nil
	}
	if role, ok := s.roleBindingOverlayForTenant(tenantID, subject); ok {
		return role, nil
	}
	var role string
	err := s.db.QueryRowContext(ctx, `SELECT role FROM tenant_role_bindings WHERE tenant_id = $1 AND subject = $2`, tenantID, subject).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

func (s *Store) UpsertProviderPolicy(ctx context.Context, tenantID, provider, mode string, enabled bool) error {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode != "allow" && mode != "deny" {
		return errors.New("invalid provider policy mode")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO tenant_provider_policies (tenant_id, provider, mode, enabled)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, provider)
DO UPDATE SET mode = EXCLUDED.mode, enabled = EXCLUDED.enabled
`, tenantID, provider, mode, enabled)
	return err
}

func (s *Store) ProviderPolicies(ctx context.Context, tenantID string) ([]TenantProviderPolicy, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, nil
	}

	if policies, ok := s.providerPoliciesOverlayForTenant(tenantID); ok {
		return policies, nil
	}

	rows, err := s.db.QueryContext(ctx, `SELECT tenant_id, provider, mode, enabled FROM tenant_provider_policies WHERE tenant_id = $1 ORDER BY provider`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TenantProviderPolicy
	for rows.Next() {
		var item TenantProviderPolicy
		if err := rows.Scan(&item.TenantID, &item.Provider, &item.Mode, &item.Enabled); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpsertSensitiveRule(ctx context.Context, tenantID, pattern, action string, enabled bool) error {
	action = strings.TrimSpace(strings.ToLower(action))
	if action != "block" {
		return errors.New("invalid sensitive rule action")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO tenant_sensitive_rules (tenant_id, pattern, action, enabled)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, pattern)
DO UPDATE SET action = EXCLUDED.action, enabled = EXCLUDED.enabled
`, tenantID, pattern, action, enabled)
	return err
}

func (s *Store) SensitiveRules(ctx context.Context, tenantID string) ([]SensitiveRule, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, nil
	}

	if rules, ok := s.sensitiveRulesOverlayForTenant(tenantID); ok {
		return rules, nil
	}

	rows, err := s.db.QueryContext(ctx, `SELECT tenant_id, pattern, action, enabled FROM tenant_sensitive_rules WHERE tenant_id = $1 ORDER BY pattern`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SensitiveRule
	for rows.Next() {
		var item SensitiveRule
		if err := rows.Scan(&item.TenantID, &item.Pattern, &item.Action, &item.Enabled); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
