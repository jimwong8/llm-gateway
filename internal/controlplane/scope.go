package controlplane

import "errors"

type TenantTier string

const (
	TenantTierFree       TenantTier = "free"
	TenantTierPro        TenantTier = "pro"
	TenantTierEnterprise TenantTier = "enterprise"
)

type Environment string

const (
	EnvironmentDev     Environment = "dev"
	EnvironmentStaging Environment = "staging"
	EnvironmentProd    Environment = "prod"
)

type Scope string

const (
	ScopeTenant  Scope = "tenant"
	ScopeProject Scope = "project"
)

type ConfigScope struct {
	TenantID    string      `json:"tenant_id"`
	TenantTier  TenantTier  `json:"tenant_tier"`
	Environment Environment `json:"environment"`
	Scope       Scope       `json:"scope"`
	ProjectID   string      `json:"project_id,omitempty"`
}

func (s ConfigScope) Validate() error {
	if s.TenantID == "" {
		return errors.New("tenant_id is required")
	}
	switch s.Environment {
	case EnvironmentDev, EnvironmentStaging, EnvironmentProd:
	default:
		return errors.New("invalid environment")
	}
	switch s.Scope {
	case ScopeTenant:
		if s.ProjectID != "" {
			return errors.New("project_id must be empty for tenant scope")
		}
	case ScopeProject:
		if s.ProjectID == "" {
			return errors.New("project_id is required for project scope")
		}
	default:
		return errors.New("invalid scope")
	}
	switch s.TenantTier {
	case "", TenantTierFree, TenantTierPro, TenantTierEnterprise:
	default:
		return errors.New("invalid tenant_tier")
	}
	return nil
}

func ResolveEffectiveScope(projectScope, tenantScope *ConfigScope) *ConfigScope {
	if projectScope != nil {
		return projectScope
	}
	return tenantScope
}
