package runtime

import (
	"testing"
	"time"
)

func TestAllStatusesReturnsStableNameSortedOrder(t *testing.T) {
	m := NewManager()
	now := time.Now().UTC()
	m.MarkEventSeen("quota", "v1", now)
	m.MarkEventSeen("router", "v2", now.Add(time.Minute))
	m.MarkEventSeen("audit", "v3", now.Add(2*time.Minute))

	statuses := m.AllStatuses()
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	if statuses[0].Name != "audit" || statuses[1].Name != "quota" || statuses[2].Name != "router" {
		t.Fatalf("expected name-sorted order [audit quota router], got [%s %s %s]", statuses[0].Name, statuses[1].Name, statuses[2].Name)
	}
}
