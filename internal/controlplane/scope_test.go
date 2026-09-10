package controlplane

import "testing"

func TestScopeConstantsShape(t *testing.T) {
	if ScopeTenant != "tenant" || ScopeProject != "project" {
		t.Fatalf("unexpected scope constants")
	}
	if TenantTierFree != "free" || TenantTierPro != "pro" || TenantTierEnterprise != "enterprise" {
		t.Fatalf("unexpected tenant tier constants")
	}
	if EnvironmentDev != "dev" || EnvironmentStaging != "staging" || EnvironmentProd != "prod" {
		t.Fatalf("unexpected environment constants")
	}
}

func TestProjectScopeRequiresProjectID(t *testing.T) {
	s := ConfigScope{TenantID: "t1", TenantTier: TenantTierPro, Environment: EnvironmentDev, Scope: ScopeProject}
	if err := s.Validate(); err == nil {
		t.Fatal("expected validation error for missing project_id")
	}
}

func TestConfigScopeValidation(t *testing.T) {
	s := ConfigScope{TenantID: "t1", TenantTier: TenantTierEnterprise, Environment: EnvironmentProd, Scope: ScopeTenant}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected valid scope, got %v", err)
	}
}

func TestProjectOverridePrecedence(t *testing.T) {
	tenant := &ConfigScope{TenantID: "t1", Scope: ScopeTenant, Environment: EnvironmentProd}
	project := &ConfigScope{TenantID: "t1", Scope: ScopeProject, ProjectID: "p1", Environment: EnvironmentProd}
	resolved := ResolveEffectiveScope(project, tenant)
	if resolved != project {
		t.Fatal("expected project scope to win")
	}
}

func TestMissingProjectOverrideFallsBackToTenantDefault(t *testing.T) {
	tenant := &ConfigScope{TenantID: "t1", Scope: ScopeTenant, Environment: EnvironmentProd}
	resolved := ResolveEffectiveScope(nil, tenant)
	if resolved != tenant {
		t.Fatal("expected tenant default fallback")
	}
}
