package controlplane

import (
	"testing"
	"time"
)

func TestConfigVersionCarriesEnvironmentScope(t *testing.T) {
	v := ConfigVersion{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "tenant", Version: "v1", CreatedAt: time.Now()}
	if v.Environment != "prod" || v.Scope != "tenant" {
		t.Fatalf("unexpected version shape: %+v", v)
	}
}

func TestVersionStoreAppendsVersions(t *testing.T) {
	store := NewVersionStore()
	store.AddVersion(ConfigVersion{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "tenant", Version: "v1"})
	store.AddVersion(ConfigVersion{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "tenant", Version: "v2"})
	if len(store.ListVersions()) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(store.ListVersions()))
	}
}

func TestVersionStoreTracksActivePointer(t *testing.T) {
	store := NewVersionStore()
	store.SetActive(ActivePointer{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "tenant", Version: "v2"})
	pointer, ok := store.GetActive("policy", "t1", "prod", "tenant", "")
	if !ok || pointer.Version != "v2" {
		t.Fatalf("unexpected pointer: %+v ok=%v", pointer, ok)
	}
}

func TestConfigDiffShape(t *testing.T) {
	from := ConfigVersion{Version: "v1", Summary: "a"}
	to := ConfigVersion{Version: "v2", Summary: "b"}
	diff := DiffVersions(from, to)
	if diff.FromVersion != "v1" || diff.ToVersion != "v2" {
		t.Fatalf("unexpected diff: %+v", diff)
	}
}

func TestVersionRecordCarriesChangeOrigin(t *testing.T) {
	v := ConfigVersion{Version: "v1", Actor: "alice", Source: "api", Summary: "changed policy"}
	if v.Actor != "alice" || v.Source != "api" {
		t.Fatalf("unexpected origin metadata: %+v", v)
	}
}

func TestProjectVersionsAreIndependent(t *testing.T) {
	store := NewVersionStore()
	store.AddVersion(ConfigVersion{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "project", ProjectID: "p1", Version: "v1"})
	store.AddVersion(ConfigVersion{Module: "policy", TenantID: "t1", Environment: "prod", Scope: "project", ProjectID: "p2", Version: "v1"})
	if len(store.ListVersions()) != 2 {
		t.Fatalf("expected 2 independent project versions")
	}
}
