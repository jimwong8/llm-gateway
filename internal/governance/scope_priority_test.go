package governance_test

import (
	"reflect"
	"testing"

	"llm-gateway/gateway/internal/governance"
)

func TestScopePriorityOrder(t *testing.T) {
	want := []string{
		"emergency_override",
		"tenant_agent",
		"tenant",
		"agent",
		"task_type",
		"environment",
		"global",
	}

	got := governance.ScopePriorityOrder()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected scope priority order: got=%v want=%v", got, want)
	}
}
