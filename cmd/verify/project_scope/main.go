package main

import (
	"context"
	"fmt"
	"os"

	"llm-gateway/gateway/internal/controlplane"
)

func main() {
	verifyProjectScopeValidation()
	verifyEffectiveScopePrecedence()
	verifyResolveConfigPrecedence()

	fmt.Println("verify result: PASS project_scope")
}

func verifyProjectScopeValidation() {
	scope := controlplane.ConfigScope{
		TenantID:    "t1",
		Environment: controlplane.EnvironmentProd,
		Scope:       controlplane.ScopeProject,
	}
	if err := scope.Validate(); err == nil {
		fail("project scope validation", fmt.Errorf("expected validation error for missing project_id"))
	}
	fmt.Println("project scope validation: PASS missing project_id rejected")
}

func verifyEffectiveScopePrecedence() {
	tenant := &controlplane.ConfigScope{TenantID: "t1", Scope: controlplane.ScopeTenant, Environment: controlplane.EnvironmentProd}
	project := &controlplane.ConfigScope{TenantID: "t1", Scope: controlplane.ScopeProject, ProjectID: "p1", Environment: controlplane.EnvironmentProd}
	resolved := controlplane.ResolveEffectiveScope(project, tenant)
	if resolved != project {
		fail("effective scope precedence", fmt.Errorf("expected project scope to win over tenant scope"))
	}
	fmt.Println("effective precedence: PASS project override > tenant default")
}

func verifyResolveConfigPrecedence() {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released",
		Config: map[string]string{
			"model": "prod-model",
		},
	})
	if err != nil {
		fail("seed released config", err)
	}

	resolved, err := svc.ResolveConfig(
		context.Background(),
		"router",
		"tenant-a",
		"prod",
		"tenant",
		"",
		map[string]string{"timeout": "1s", "model": "project-model"},
		map[string]string{"timeout": "2s", "region": "tenant-region"},
		map[string]string{"timeout": "3s", "region": "template-region", "tier": "template-tier"},
		map[string]string{"timeout": "4s", "region": "default-region", "tier": "default-tier"},
	)
	if err != nil {
		fail("resolve config", err)
	}
	if got := resolved["model"]; got != "project-model" {
		fail("resolve config model precedence", fmt.Errorf("expected project override model, got %q", got))
	}
	if got := resolved["timeout"]; got != "1s" {
		fail("resolve config timeout precedence", fmt.Errorf("expected project override timeout, got %q", got))
	}
	if got := resolved["region"]; got != "tenant-region" {
		fail("resolve config tenant region precedence", fmt.Errorf("expected tenant override region, got %q", got))
	}
	if got := resolved["tier"]; got != "template-tier" {
		fail("resolve config tenant template precedence", fmt.Errorf("expected tenant template tier, got %q", got))
	}
	fmt.Println("project scope config precedence: PASS project > tenant > template > default")
}

func fail(step string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "verify failed at %s: %v\n", step, err)
	os.Exit(1)
}
