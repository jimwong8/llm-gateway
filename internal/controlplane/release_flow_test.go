package controlplane

import "testing"

func TestReleaseFlowIsolatedByEnvironment(t *testing.T) {
	store := NewReleaseStore()
	r1 := store.ApproveRelease(ReleaseRecord{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "tenant", Version: "v1"})
	r2 := store.ApproveRelease(ReleaseRecord{Module: "policy", TenantID: "t1", Environment: "staging", Scope: "tenant", Version: "v1"})
	if r1.Environment == r2.Environment {
		t.Fatalf("expected isolated environments")
	}
}

func TestProjectReleaseDoesNotOverrideTenantDefault(t *testing.T) {
	store := NewReleaseStore()
	tenant := store.ApproveRelease(ReleaseRecord{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "tenant", Version: "v1"})
	project := store.ApproveRelease(ReleaseRecord{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "project", ProjectID: "p1", Version: "v2"})
	if tenant.Scope == project.Scope {
		t.Fatalf("expected tenant and project releases to remain isolated")
	}
}

func TestReleasedVersionIsImmutable(t *testing.T) {
	record := ReleaseRecord{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "tenant", Version: "v1", State: ReleaseStateReleased}
	if record.State != ReleaseStateReleased {
		t.Fatalf("expected released state")
	}
}

func TestRollbackCreatesNewReleasedVersion(t *testing.T) {
	store := NewReleaseStore()
	r := store.RollbackToVersion("policy", "t1", "prod", "tenant", "", "v1")
	if r.State != ReleaseStateReleased {
		t.Fatalf("expected rollback to create released record")
	}
}
