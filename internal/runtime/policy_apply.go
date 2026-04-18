package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"llm-gateway/gateway/internal/controlplane"
	policy "llm-gateway/gateway/internal/policy"
)

type policyAllowedModelsOverlayWriter interface {
	SetAllowedModelsOverlay(tenantID string, models []string)
	SetRoleBindingsOverlay(tenantID string, bindings []policy.TenantRoleBinding)
	SetProviderPoliciesOverlay(tenantID string, policies []policy.TenantProviderPolicy)
	SetSensitiveRulesOverlay(tenantID string, rules []policy.SensitiveRule)
}

func BuildPolicyPayloadDrivenApply(store policyAllowedModelsOverlayWriter, publisher *Publisher, resolver releasedVersionResolver) func(ConfigChangeEvent) error {
	return BuildPolicyPayloadDrivenApplyWithResolver(store, publisher, resolver)
}

// BuildPolicyPayloadDrivenApplyWithResolver 构建 policy runtime apply：
// 1) 优先从 released payload_ref 对应 controlplane 版本构建 payload 并应用；
// 2) resolver 不可用或失败时回退到 publisher 缓存 payload；
// 3) 仅支持最小动态字段 models/allowed_models、provider_policies 与 sensitive rules，缺失时 no-op（保持兼容）。
func BuildPolicyPayloadDrivenApplyWithResolver(store policyAllowedModelsOverlayWriter, publisher *Publisher, resolver releasedVersionResolver) func(ConfigChangeEvent) error {
	return func(event ConfigChangeEvent) error {
		if store == nil {
			return nil
		}

		payloadRef := strings.TrimSpace(event.PayloadRef)
		if payloadRef == "" {
			return nil
		}

		if resolver != nil {
			if parsed, ok := parseReleasedPayloadRef(payloadRef); ok {
				if version, err := resolver.GetVersion(context.Background(), parsed.Module, parsed.TenantID, parsed.Environment, parsed.Scope, parsed.ProjectID, parsed.VersionID); err == nil {
					if applyPayload, ok := buildRuntimeApplyPayloadFromReleasedVersion(version); ok {
						applied, err := applyPolicyPayload(store, applyPayload)
						if err != nil {
							return err
						}
						if applied {
							return nil
						}
					}
				}
			}
		}

		if publisher != nil {
			if applyPayload, ok := publisher.FindApplyPayloadByRef(payloadRef); ok {
				applied, err := applyPolicyPayload(store, applyPayload)
				if err != nil {
					return err
				}
				if applied {
					return nil
				}
			}
		}

		return nil
	}
}

func applyPolicyPayload(store policyAllowedModelsOverlayWriter, applyPayload RuntimeApplyPayload) (bool, error) {
	rawPolicyPayload, hasPolicyPayload := applyPayload.ModulePayloads["policy"]
	if !hasPolicyPayload {
		return false, nil
	}
	policyPayload, ok := rawPolicyPayload.(map[string]any)
	if !ok {
		return false, fmt.Errorf("policy runtime apply payload type invalid: %T", rawPolicyPayload)
	}

	models, hasModels, err := extractPolicyAllowedModels(policyPayload)
	if err != nil {
		return false, err
	}
	roleBindings, hasRoleBindings, err := extractPolicyRoleBindings(policyPayload)
	if err != nil {
		return false, err
	}
	providerPolicies, hasProviderPolicies, err := extractPolicyProviderPolicies(policyPayload)
	if err != nil {
		return false, err
	}
	sensitiveRules, hasSensitiveRules, err := extractPolicySensitiveRules(policyPayload)
	if err != nil {
		return false, err
	}
	if !hasModels && !hasRoleBindings && !hasProviderPolicies && !hasSensitiveRules {
		return false, nil
	}

	tenantID := strings.TrimSpace(applyPayload.TenantID)
	if tenantID == "" {
		return false, fmt.Errorf("policy runtime apply tenant_id is required")
	}

	if hasModels {
		store.SetAllowedModelsOverlay(tenantID, models)
	}
	if hasRoleBindings {
		overlayBindings := make([]policy.TenantRoleBinding, len(roleBindings))
		for idx, item := range roleBindings {
			overlayBindings[idx] = policy.TenantRoleBinding{
				TenantID: tenantID,
				Subject:  item.Subject,
				Role:     item.Role,
			}
		}
		store.SetRoleBindingsOverlay(tenantID, overlayBindings)
	}
	if hasProviderPolicies {
		overlayPolicies := make([]policy.TenantProviderPolicy, len(providerPolicies))
		for idx, item := range providerPolicies {
			overlayPolicies[idx] = policy.TenantProviderPolicy{
				TenantID: tenantID,
				Provider: item.Provider,
				Mode:     item.Mode,
				Enabled:  item.Enabled,
			}
		}
		store.SetProviderPoliciesOverlay(tenantID, overlayPolicies)
	}
	if hasSensitiveRules {
		overlayRules := make([]policy.SensitiveRule, len(sensitiveRules))
		for idx, item := range sensitiveRules {
			overlayRules[idx] = policy.SensitiveRule{
				TenantID: tenantID,
				Pattern:  item.Pattern,
				Action:   item.Action,
				Enabled:  item.Enabled,
			}
		}
		store.SetSensitiveRulesOverlay(tenantID, overlayRules)
	}
	return true, nil
}

func extractPolicyAllowedModels(policyPayload map[string]any) ([]string, bool, error) {
	if rawModels, ok := policyPayload["models"]; ok {
		modelsMap, ok := rawModels.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("policy runtime apply models type invalid: %T", rawModels)
		}
		if rawAllowedModels, ok := modelsMap["allowed_models"]; ok {
			models, err := parseAllowedModelsList(rawAllowedModels)
			if err != nil {
				return nil, false, err
			}
			return models, true, nil
		}
	}
	if rawAllowedModels, ok := policyPayload["allowed_models"]; ok {
		models, err := parseAllowedModelsList(rawAllowedModels)
		if err != nil {
			return nil, false, err
		}
		return models, true, nil
	}
	return nil, false, nil
}

func extractPolicyRoleBindings(policyPayload map[string]any) ([]policy.TenantRoleBinding, bool, error) {
	if rawRoles, ok := policyPayload["roles"]; ok {
		rolesMap, ok := rawRoles.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("policy runtime apply roles type invalid: %T", rawRoles)
		}
		if rawBindings, ok := rolesMap["bindings"]; ok {
			bindings, err := parsePolicyRoleBindingsList(rawBindings)
			if err != nil {
				return nil, false, err
			}
			return bindings, true, nil
		}
	}
	if rawBindings, ok := policyPayload["role_bindings"]; ok {
		bindings, err := parsePolicyRoleBindingsList(rawBindings)
		if err != nil {
			return nil, false, err
		}
		return bindings, true, nil
	}
	return nil, false, nil
}

func extractPolicyProviderPolicies(policyPayload map[string]any) ([]policy.TenantProviderPolicy, bool, error) {
	if rawProviders, ok := policyPayload["providers"]; ok {
		providersMap, ok := rawProviders.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("policy runtime apply providers type invalid: %T", rawProviders)
		}
		if rawPolicies, ok := providersMap["policies"]; ok {
			policies, err := parsePolicyProviderPoliciesList(rawPolicies)
			if err != nil {
				return nil, false, err
			}
			return policies, true, nil
		}
	}
	if rawPolicies, ok := policyPayload["provider_policies"]; ok {
		policies, err := parsePolicyProviderPoliciesList(rawPolicies)
		if err != nil {
			return nil, false, err
		}
		return policies, true, nil
	}
	return nil, false, nil
}

func extractPolicySensitiveRules(policyPayload map[string]any) ([]policy.SensitiveRule, bool, error) {
	if rawSensitive, ok := policyPayload["sensitive"]; ok {
		sensitiveMap, ok := rawSensitive.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("policy runtime apply sensitive type invalid: %T", rawSensitive)
		}
		if rawRules, ok := sensitiveMap["rules"]; ok {
			rules, err := parsePolicySensitiveRulesList(rawRules)
			if err != nil {
				return nil, false, err
			}
			return rules, true, nil
		}
	}
	if rawRules, ok := policyPayload["sensitive_rules"]; ok {
		rules, err := parsePolicySensitiveRulesList(rawRules)
		if err != nil {
			return nil, false, err
		}
		return rules, true, nil
	}
	return nil, false, nil
}

func parseAllowedModelsList(raw any) ([]string, error) {
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			model, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("policy runtime apply allowed_models contains non-string item: %T", item)
			}
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			out = append(out, model)
		}
		return out, nil
	case []string:
		out := make([]string, 0, len(v))
		for _, model := range v {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			out = append(out, model)
		}
		return out, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return []string{}, nil
		}
		var parsed []string
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return nil, fmt.Errorf("policy runtime apply allowed_models string json invalid: %w", err)
		}
		out := make([]string, 0, len(parsed))
		for _, model := range parsed {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			out = append(out, model)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("policy runtime apply allowed_models type invalid: %T", raw)
	}
}

func parsePolicyRoleBindingsList(raw any) ([]policy.TenantRoleBinding, error) {
	switch v := raw.(type) {
	case []policy.TenantRoleBinding:
		out := make([]policy.TenantRoleBinding, 0, len(v))
		for _, item := range v {
			subject := strings.TrimSpace(item.Subject)
			role := strings.TrimSpace(strings.ToLower(item.Role))
			if subject == "" {
				return nil, fmt.Errorf("policy runtime apply role binding subject is required")
			}
			if role != "admin" && role != "operator" && role != "readonly" {
				return nil, fmt.Errorf("policy runtime apply role binding role invalid: %q", item.Role)
			}
			out = append(out, policy.TenantRoleBinding{Subject: subject, Role: role})
		}
		return dedupeRoleBindings(out), nil
	case []any:
		out := make([]policy.TenantRoleBinding, 0, len(v))
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("policy runtime apply role_bindings contains non-object item: %T", item)
			}
			subject, _ := entry["subject"].(string)
			role, _ := entry["role"].(string)
			subject = strings.TrimSpace(subject)
			role = strings.TrimSpace(strings.ToLower(role))
			if subject == "" {
				return nil, fmt.Errorf("policy runtime apply role binding subject is required")
			}
			if role != "admin" && role != "operator" && role != "readonly" {
				return nil, fmt.Errorf("policy runtime apply role binding role invalid: %q", role)
			}
			out = append(out, policy.TenantRoleBinding{Subject: subject, Role: role})
		}
		return dedupeRoleBindings(out), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return []policy.TenantRoleBinding{}, nil
		}
		var parsed []any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return nil, fmt.Errorf("policy runtime apply role_bindings string json invalid: %w", err)
		}
		return parsePolicyRoleBindingsList(parsed)
	default:
		return nil, fmt.Errorf("policy runtime apply role_bindings type invalid: %T", raw)
	}
}

func dedupeRoleBindings(in []policy.TenantRoleBinding) []policy.TenantRoleBinding {
	out := make([]policy.TenantRoleBinding, 0, len(in))
	indexBySubject := map[string]int{}
	for _, item := range in {
		subject := strings.TrimSpace(item.Subject)
		if subject == "" {
			continue
		}
		normalized := policy.TenantRoleBinding{Subject: subject, Role: strings.TrimSpace(strings.ToLower(item.Role))}
		if idx, ok := indexBySubject[subject]; ok {
			out[idx] = normalized
			continue
		}
		indexBySubject[subject] = len(out)
		out = append(out, normalized)
	}
	return out
}

func parsePolicyProviderPoliciesList(raw any) ([]policy.TenantProviderPolicy, error) {
	switch v := raw.(type) {
	case []policy.TenantProviderPolicy:
		out := make([]policy.TenantProviderPolicy, 0, len(v))
		for _, item := range v {
			provider := strings.TrimSpace(strings.ToLower(item.Provider))
			mode := strings.TrimSpace(strings.ToLower(item.Mode))
			if provider == "" {
				return nil, fmt.Errorf("policy runtime apply provider policy provider is required")
			}
			if mode != "allow" && mode != "deny" {
				return nil, fmt.Errorf("policy runtime apply provider policy mode invalid: %q", item.Mode)
			}
			out = append(out, policy.TenantProviderPolicy{Provider: provider, Mode: mode, Enabled: item.Enabled})
		}
		return dedupeProviderPolicies(out), nil
	case []any:
		out := make([]policy.TenantProviderPolicy, 0, len(v))
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("policy runtime apply provider_policies contains non-object item: %T", item)
			}
			provider, _ := entry["provider"].(string)
			mode, _ := entry["mode"].(string)
			provider = strings.TrimSpace(strings.ToLower(provider))
			mode = strings.TrimSpace(strings.ToLower(mode))
			if provider == "" {
				return nil, fmt.Errorf("policy runtime apply provider policy provider is required")
			}
			if mode != "allow" && mode != "deny" {
				return nil, fmt.Errorf("policy runtime apply provider policy mode invalid: %q", mode)
			}
			enabled := true
			if rawEnabled, hasEnabled := entry["enabled"]; hasEnabled {
				enabledBool, ok := rawEnabled.(bool)
				if !ok {
					return nil, fmt.Errorf("policy runtime apply provider policy enabled type invalid: %T", rawEnabled)
				}
				enabled = enabledBool
			}
			out = append(out, policy.TenantProviderPolicy{Provider: provider, Mode: mode, Enabled: enabled})
		}
		return dedupeProviderPolicies(out), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return []policy.TenantProviderPolicy{}, nil
		}
		var parsed []any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return nil, fmt.Errorf("policy runtime apply provider_policies string json invalid: %w", err)
		}
		return parsePolicyProviderPoliciesList(parsed)
	default:
		return nil, fmt.Errorf("policy runtime apply provider_policies type invalid: %T", raw)
	}
}

func dedupeProviderPolicies(in []policy.TenantProviderPolicy) []policy.TenantProviderPolicy {
	out := make([]policy.TenantProviderPolicy, 0, len(in))
	indexByProvider := map[string]int{}
	for _, item := range in {
		provider := strings.TrimSpace(strings.ToLower(item.Provider))
		if provider == "" {
			continue
		}
		normalized := policy.TenantProviderPolicy{Provider: provider, Mode: strings.TrimSpace(strings.ToLower(item.Mode)), Enabled: item.Enabled}
		if idx, ok := indexByProvider[provider]; ok {
			out[idx] = normalized
			continue
		}
		indexByProvider[provider] = len(out)
		out = append(out, normalized)
	}
	return out
}

func parsePolicySensitiveRulesList(raw any) ([]policy.SensitiveRule, error) {
	switch v := raw.(type) {
	case []policy.SensitiveRule:
		out := make([]policy.SensitiveRule, 0, len(v))
		for _, item := range v {
			pattern := strings.TrimSpace(item.Pattern)
			action := strings.TrimSpace(strings.ToLower(item.Action))
			if pattern == "" {
				return nil, fmt.Errorf("policy runtime apply sensitive rule pattern is required")
			}
			if action != "block" {
				return nil, fmt.Errorf("policy runtime apply sensitive rule action invalid: %q", item.Action)
			}
			out = append(out, policy.SensitiveRule{Pattern: pattern, Action: action, Enabled: item.Enabled})
		}
		return dedupeSensitiveRules(out), nil
	case []any:
		out := make([]policy.SensitiveRule, 0, len(v))
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("policy runtime apply sensitive_rules contains non-object item: %T", item)
			}
			pattern, _ := entry["pattern"].(string)
			action, _ := entry["action"].(string)
			pattern = strings.TrimSpace(pattern)
			action = strings.TrimSpace(strings.ToLower(action))
			if pattern == "" {
				return nil, fmt.Errorf("policy runtime apply sensitive rule pattern is required")
			}
			if action != "block" {
				return nil, fmt.Errorf("policy runtime apply sensitive rule action invalid: %q", action)
			}
			enabled := true
			if rawEnabled, hasEnabled := entry["enabled"]; hasEnabled {
				enabledBool, ok := rawEnabled.(bool)
				if !ok {
					return nil, fmt.Errorf("policy runtime apply sensitive rule enabled type invalid: %T", rawEnabled)
				}
				enabled = enabledBool
			}
			out = append(out, policy.SensitiveRule{Pattern: pattern, Action: action, Enabled: enabled})
		}
		return dedupeSensitiveRules(out), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return []policy.SensitiveRule{}, nil
		}
		var parsed []any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return nil, fmt.Errorf("policy runtime apply sensitive_rules string json invalid: %w", err)
		}
		return parsePolicySensitiveRulesList(parsed)
	default:
		return nil, fmt.Errorf("policy runtime apply sensitive_rules type invalid: %T", raw)
	}
}

func dedupeSensitiveRules(in []policy.SensitiveRule) []policy.SensitiveRule {
	out := make([]policy.SensitiveRule, 0, len(in))
	indexByPattern := map[string]int{}
	for _, item := range in {
		pattern := strings.TrimSpace(item.Pattern)
		if pattern == "" {
			continue
		}
		normalized := policy.SensitiveRule{Pattern: pattern, Action: strings.TrimSpace(strings.ToLower(item.Action)), Enabled: item.Enabled}
		if idx, ok := indexByPattern[pattern]; ok {
			out[idx] = normalized
			continue
		}
		indexByPattern[pattern] = len(out)
		out = append(out, normalized)
	}
	return out
}

func buildPolicyModulePayload(version controlplane.ConfigVersion) (map[string]any, bool) {
	if !strings.EqualFold(strings.TrimSpace(version.Module), "policy") {
		return nil, false
	}
	if len(version.Config) == 0 {
		return nil, false
	}

	payload := map[string]any{}
	if parsed, ok := parsePolicyAllowedModelsFromConfig(version.Config["models/allowed_models"]); ok {
		payload["models"] = map[string]any{"allowed_models": parsed}
	} else if parsed, ok := parsePolicyAllowedModelsFromConfig(version.Config["allowed_models"]); ok {
		payload["models"] = map[string]any{"allowed_models": parsed}
	}

	if parsed, ok := parsePolicyRoleBindingsFromConfig(version.Config["roles/bindings"]); ok {
		payload["roles"] = map[string]any{"bindings": parsed}
	} else if parsed, ok := parsePolicyRoleBindingsFromConfig(version.Config["role_bindings"]); ok {
		payload["roles"] = map[string]any{"bindings": parsed}
	}

	if parsed, ok := parsePolicyProviderPoliciesFromConfig(version.Config["providers/policies"]); ok {
		payload["providers"] = map[string]any{"policies": parsed}
	} else if parsed, ok := parsePolicyProviderPoliciesFromConfig(version.Config["provider_policies"]); ok {
		payload["providers"] = map[string]any{"policies": parsed}
	}

	if parsed, ok := parsePolicySensitiveRulesFromConfig(version.Config["sensitive/rules"]); ok {
		payload["sensitive"] = map[string]any{"rules": parsed}
	} else if parsed, ok := parsePolicySensitiveRulesFromConfig(version.Config["sensitive_rules"]); ok {
		payload["sensitive"] = map[string]any{"rules": parsed}
	}

	if len(payload) == 0 {
		return nil, false
	}
	return payload, true
}

func parsePolicyAllowedModelsFromConfig(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	out := make([]string, 0, len(parsed))
	for _, model := range parsed {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		out = append(out, model)
	}
	return out, true
}

func parsePolicyRoleBindingsFromConfig(raw string) ([]policy.TenantRoleBinding, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	var parsed []any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	bindings, err := parsePolicyRoleBindingsList(parsed)
	if err != nil {
		return nil, false
	}
	return bindings, true
}

func parsePolicyProviderPoliciesFromConfig(raw string) ([]policy.TenantProviderPolicy, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	var parsed []any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	policies, err := parsePolicyProviderPoliciesList(parsed)
	if err != nil {
		return nil, false
	}
	return policies, true
}

func parsePolicySensitiveRulesFromConfig(raw string) ([]policy.SensitiveRule, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	var parsed []any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	rules, err := parsePolicySensitiveRulesList(parsed)
	if err != nil {
		return nil, false
	}
	return rules, true
}
