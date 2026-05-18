package memory

import (
	"strings"
	"testing"
)

func TestContextBudgetAllocateRespectsRatiosAndCaps(t *testing.T) {
	cb := ContextBudget{
		MaxTokens:     100,
		ReserveTokens: 10,
		MemoryRatio:   0.5,
		SystemRatio:   0.2,
		HistoryRatio:  0.5,
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
		MaxTokens:     10,
		ReserveTokens: 10,
		MemoryRatio:   0.2,
		SystemRatio:   0.1,
		HistoryRatio:  0.5,
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
