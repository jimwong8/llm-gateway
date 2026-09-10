package runtime

import (
	"errors"
	"testing"
	"time"
)

func TestReloadStatusShape(t *testing.T) {
	m := NewManager()
	m.SetStatus("policy", "ok", "")
	status := m.GetStatus("policy")
	if status.Name != "policy" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("unexpected reload status: %+v", status)
	}
}

func TestReloadStatusTracksLastSeenEvent(t *testing.T) {
	m := NewManager()
	now := time.Now().UTC()
	m.MarkEventSeen("quota", "v1", now)
	status := m.GetStatus("quota")
	if status.LastSeenEventVersion != "v1" {
		t.Fatalf("unexpected event version: %+v", status)
	}
	if status.LastSeenEventAt.IsZero() {
		t.Fatalf("expected last seen event time: %+v", status)
	}
}

func TestHandleConfigChangeUpdatesReloadStatus(t *testing.T) {
	m := NewManager()
	event := ConfigChangeEvent{Module: "route", Version: "v2", ChangedAt: time.Now().UTC()}
	m.HandleConfigChange(event, func() error { return nil })
	status := m.GetStatus("route")
	if status.LastSeenEventVersion != "v2" {
		t.Fatalf("unexpected status after handle: %+v", status)
	}
	if status.LastReloadStatus != "ok" {
		t.Fatalf("expected ok reload status: %+v", status)
	}
}

func TestHandleConfigChangeHandlesReloadError(t *testing.T) {
	m := NewManager()
	event := ConfigChangeEvent{Module: "observability", Version: "v3", ChangedAt: time.Now().UTC()}
	m.HandleConfigChange(event, func() error { return errors.New("reload failed") })
	status := m.GetStatus("observability")
	if status.LastReloadStatus != "error" {
		t.Fatalf("expected error reload status: %+v", status)
	}
	if status.LastReloadError == "" {
		t.Fatalf("expected reload error message: %+v", status)
	}
}
