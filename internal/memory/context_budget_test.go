package memory

import (
	"strings"
	"testing"
)

func TestContextBudgetAllocateRespectsRatiosAndCaps(t *testing.T) {
	cb := ContextBudget{
		MaxTokens:          100,
		ReserveTokens:      10,
		MemoryRatio:        0.5,
		SystemRatio:        0.2,
		HistoryRatio:       0.5,
		StretchedThreshold: 0.6,
		CriticalThreshold:  0.8,
		StallThreshold:     0.95,
	}

	alloc := cb.Allocate(100, 50)

	if alloc.AvailableTokens != 90 {
		t.Fatalf("available tokens mismatch: got=%d want=%d", alloc.AvailableTokens, 90)
	}
	if alloc.SystemTokens != 18 {
		t.Fatalf("system tokens mismatch: got=%d want=%d", alloc.SystemTokens, 18)
	}
	if alloc.HistoryTokens != 36 {
		t.Fatalf("history tokens mismatch: got=%d want=%d", alloc.HistoryTokens, 36)
	}
	if alloc.MemoryTokens != 18 {
		t.Fatalf("memory tokens mismatch: got=%d want=%d", alloc.MemoryTokens, 18)
	}
	if alloc.ReserveTokens != 10 {
		t.Fatalf("reserve tokens mismatch: got=%d want=%d", alloc.ReserveTokens, 10)
	}
}

func TestContextBudgetAllocateHandlesNoAvailableTokens(t *testing.T) {
	cb := ContextBudget{
		MaxTokens:          10,
		ReserveTokens:      10,
		MemoryRatio:        0.2,
		SystemRatio:        0.1,
		HistoryRatio:       0.5,
		StretchedThreshold: 0.6,
		CriticalThreshold:  0.8,
		StallThreshold:     0.95,
	}

	alloc := cb.Allocate(50, 20)
	if alloc.SystemTokens != 0 || alloc.HistoryTokens != 0 || alloc.MemoryTokens != 0 {
		t.Fatalf("expected all allocated buckets to be zero, got %#v", alloc)
	}
	if alloc.ReserveTokens != 10 {
		t.Fatalf("reserve tokens mismatch: got=%d want=%d", alloc.ReserveTokens, 10)
	}
	if alloc.AvailableTokens != 0 {
		t.Fatalf("expected available tokens 0, got %d", alloc.AvailableTokens)
	}
}

// ---- Context Budget State Machine Tests ----

func TestContextBudgetStateTransitions(t *testing.T) {
	cb := DefaultContextBudget() // thresholds: 0.6 / 0.8 / 0.95

	tests := []struct {
		name            string
		recentTokens    int
		systemTokens    int
		memoryTokens    int
		graphTokens     int
		summaryTokens   int
		wantState       BudgetState
		wantUtilization float64
	}{
		{
			name:         "normal: low utilization",
			recentTokens: 100, systemTokens: 50, memoryTokens: 0, graphTokens: 0, summaryTokens: 0,
			wantState:       BudgetStateNormal,
			wantUtilization: 0.0183,
		},
		{
			name:         "stretched: above 60%",
			recentTokens: 4000, systemTokens: 1000, memoryTokens: 500, graphTokens: 0, summaryTokens: 0,
			wantState:       BudgetStateStretched,
			wantUtilization: 0.6719,
		},
		{
			name:         "critical: above 80%",
			recentTokens: 5000, systemTokens: 1500, memoryTokens: 500, graphTokens: 0, summaryTokens: 0,
			wantState:       BudgetStateCritical,
			wantUtilization: 0.8594,
		},
		{
			name:         "stall: above 95%",
			recentTokens: 6000, systemTokens: 1500, memoryTokens: 500, graphTokens: 0, summaryTokens: 500,
			wantState:       BudgetStateStall,
			wantUtilization: 1.0391,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := cb.ComputeStats(tt.recentTokens, tt.systemTokens, tt.memoryTokens, tt.graphTokens, tt.summaryTokens)
			if stats.BudgetState != tt.wantState {
				t.Fatalf("state mismatch: got=%s want=%s", stats.BudgetState, tt.wantState)
			}
			// Check utilization is approximately correct (within 0.01).
			diff := stats.ContextUtilization - tt.wantUtilization
			if diff < -0.01 || diff > 0.01 {
				t.Fatalf("utilization mismatch: got=%f want=%f", stats.ContextUtilization, tt.wantUtilization)
			}
		})
	}
}

func TestContextBudgetShouldTruncate(t *testing.T) {
	cb := DefaultContextBudget()

	normal := cb.ComputeStats(100, 50, 0, 0, 0)
	if cb.ShouldTruncate(normal) {
		t.Fatal("should not truncate in normal state")
	}

	stretched := cb.ComputeStats(4000, 1000, 500, 0, 0)
	if cb.ShouldTruncate(stretched) {
		t.Fatal("should not truncate in stretched state")
	}

	critical := cb.ComputeStats(5000, 1500, 500, 0, 0)
	if !cb.ShouldTruncate(critical) {
		t.Fatal("should truncate in critical state")
	}

	stall := cb.ComputeStats(6000, 1500, 500, 0, 500)
	if !cb.ShouldTruncate(stall) {
		t.Fatal("should truncate in stall state")
	}
}

func TestContextBudgetShouldSummarise(t *testing.T) {
	cb := DefaultContextBudget()

	normal := cb.ComputeStats(100, 50, 0, 0, 0)
	if cb.ShouldSummarise(normal) {
		t.Fatal("should not summarise in normal state")
	}

	stretched := cb.ComputeStats(4000, 1000, 500, 0, 0)
	if !cb.ShouldSummarise(stretched) {
		t.Fatal("should summarise in stretched state")
	}
}

func TestContextBudgetStatsFields(t *testing.T) {
	cb := DefaultContextBudget()
	stats := cb.ComputeStats(2000, 500, 300, 200, 100)

	if stats.ContextWindowTokens != cb.MaxTokens {
		t.Fatalf("window tokens mismatch: got=%d want=%d", stats.ContextWindowTokens, cb.MaxTokens)
	}
	if stats.SummaryEnabled != true {
		t.Fatal("summary should be enabled by default")
	}
	if stats.SummaryAvailable != true {
		t.Fatal("summary should be available when summaryTokens > 0")
	}
	if stats.ReplyReserveTokens != cb.ReserveTokens {
		t.Fatalf("reserve tokens mismatch: got=%d want=%d", stats.ReplyReserveTokens, cb.ReserveTokens)
	}
	if stats.GraphContextTokens != 200 {
		t.Fatalf("graph tokens mismatch: got=%d want=%d", stats.GraphContextTokens, 200)
	}
	if stats.RetrievedTokens != 300 {
		t.Fatalf("retrieved tokens mismatch: got=%d want=%d", stats.RetrievedTokens, 300)
	}
	if stats.RecentTokens != 2000 {
		t.Fatalf("recent tokens mismatch: got=%d want=%d", stats.RecentTokens, 2000)
	}
}

// ---- Memory Selector Tests (unchanged) ----

func TestMemorySelectorSelectMemoriesByScoreWithinBudget(t *testing.T) {
	ms := NewMemorySelector(DefaultContextBudget())
	memories := []HybridSearchResult{
		{ID: 1, Content: strings.Repeat("a", 20), Score: 0.5}, // 5 tokens
		{ID: 2, Content: strings.Repeat("b", 16), Score: 0.9}, // 4 tokens
		{ID: 3, Content: strings.Repeat("c", 40), Score: 0.1}, // 10 tokens
	}

	selected := ms.SelectMemories(memories, 9)
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected memories, got %d (%#v)", len(selected), selected)
	}
	if selected[0].ID != 2 || selected[1].ID != 1 {
		t.Fatalf("expected ids [2,1] by score and budget fit, got [%d,%d]", selected[0].ID, selected[1].ID)
	}
}

func TestMemorySelectorSelectMemoriesEmptyWhenBudgetOrInputInvalid(t *testing.T) {
	ms := NewMemorySelector(DefaultContextBudget())
	if got := ms.SelectMemories(nil, 10); got != nil {
		t.Fatalf("expected nil for nil memories, got %#v", got)
	}
	if got := ms.SelectMemories([]HybridSearchResult{{ID: 1, Content: "abc", Score: 1}}, 0); got != nil {
		t.Fatalf("expected nil for zero budget, got %#v", got)
	}
}

func TestMemorySelectorBuildContextIncludesSectionsInOrder(t *testing.T) {
	ms := NewMemorySelector(DefaultContextBudget())
	selected := []HybridSearchResult{
		{ID: 1, Content: "记忆 A"},
		{ID: 2, Content: "记忆 B"},
	}

	ctx := ms.BuildContext(selected, "你是助手", "请回答")
	wantSnippets := []string{
		"System: 你是助手",
		"Relevant memories:\n  [1] 记忆 A\n  [2] 记忆 B",
		"User: 请回答",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(ctx, snippet) {
			t.Fatalf("expected context to contain %q, got:\n%s", snippet, ctx)
		}
	}
}
