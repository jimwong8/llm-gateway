package audit

import (
	"testing"
	"time"
)

func TestRecordInheritanceDraft(t *testing.T) {
	recorder := NewRecorder()
	recorder.now = func() time.Time {
		return time.Date(2026, 3, 24, 19, 10, 0, 0, time.UTC)
	}

	event := recorder.RecordInheritanceDraft(
		"router",
		"tenant-a",
		"prod",
		"staging",
		"cfg_rel_staging",
		"cfg_draft_prod",
		"architect",
		"seed prod candidate from staging",
	)

	if event.Type != ControlPlaneEventTypeInheritanceDraft {
		t.Fatalf("expected event type %q, got %q", ControlPlaneEventTypeInheritanceDraft, event.Type)
	}
	if event.SourceEnvironment != "staging" {
		t.Fatalf("expected source environment staging, got %q", event.SourceEnvironment)
	}
	if event.SourceVersionID != "cfg_rel_staging" {
		t.Fatalf("expected source version cfg_rel_staging, got %q", event.SourceVersionID)
	}
	if got := len(recorder.Events()); got != 1 {
		t.Fatalf("expected 1 event, got %d", got)
	}
}

func TestRecordRelease(t *testing.T) {
	recorder := NewRecorder()
	recorder.now = func() time.Time {
		return time.Date(2026, 3, 24, 19, 11, 0, 0, time.UTC)
	}

	event := recorder.RecordRelease(
		"router",
		"tenant-a",
		"prod",
		"cfg_rel_prod",
		"release-bot",
		"promote staging to prod",
	)

	if event.Type != ControlPlaneEventTypeRelease {
		t.Fatalf("expected event type %q, got %q", ControlPlaneEventTypeRelease, event.Type)
	}
	if event.SourceEnvironment != "" {
		t.Fatalf("expected empty source environment for release, got %q", event.SourceEnvironment)
	}
	if event.VersionID != "cfg_rel_prod" {
		t.Fatalf("expected version cfg_rel_prod, got %q", event.VersionID)
	}
	if got := len(recorder.Events()); got != 1 {
		t.Fatalf("expected 1 event, got %d", got)
	}
}
