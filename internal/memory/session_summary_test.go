package memory

import (
	"strings"
	"testing"
)

func TestApplySessionSummaryRulesTracksGoalAndStateTransitions(t *testing.T) {
	summary := &SessionSummary{}
	messages := []Message{
		{Role: "user", Content: "当前目标：实现 Phase E 验证与交付"},
		{Role: "assistant", Content: "TODO: 增加 session_summary 相关测试"},
		{Role: "assistant", Content: "Decision: 报告必须如实记录通过与失败"},
		{Role: "assistant", Content: "Blocker: smoke 环境未启动"},
		{Role: "assistant", Content: "completed: 增加 session_summary 相关测试"},
		{Role: "assistant", Content: "resolved blocker: smoke 环境未启动"},
	}

	applySessionSummaryRules(summary, messages)

	if summary.CurrentGoal != "实现 Phase E 验证与交付" {
		t.Fatalf("expected current goal updated, got %q", summary.CurrentGoal)
	}
	if !containsExact(summary.CompletedItems, "增加 session_summary 相关测试") {
		t.Fatalf("expected completed item recorded, got %#v", summary.CompletedItems)
	}
	if containsExact(summary.OpenItems, "增加 session_summary 相关测试") {
		t.Fatalf("expected completed item removed from open items, got %#v", summary.OpenItems)
	}
	if !containsExact(summary.KeyDecisions, "报告必须如实记录通过与失败") {
		t.Fatalf("expected key decision recorded, got %#v", summary.KeyDecisions)
	}
	if containsExact(summary.Blockers, "smoke 环境未启动") {
		t.Fatalf("expected resolved blocker removed, got %#v", summary.Blockers)
	}
}

func TestFormatSessionSummaryContainsExpectedSections(t *testing.T) {
	summary := &SessionSummary{
		CurrentGoal:      "完成 Phase E",
		CompletedItems:   []string{"补充单测"},
		OpenItems:        []string{"执行 smoke"},
		KeyDecisions:     []string{"先验证后出报告"},
		Blockers:         []string{"无"},
		SourceMessageSeq: 42,
	}

	formatted := FormatSessionSummary(summary)
	expectedSnippets := []string{
		"[Session Summary]",
		"Current Goal:\n完成 Phase E",
		"Completed Items:\n- 补充单测",
		"Open Items:\n- 执行 smoke",
		"Key Decisions:\n- 先验证后出报告",
		"Blockers:\n- 无",
		"Source Message Seq:\n- 42",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(formatted, snippet) {
			t.Fatalf("expected formatted summary to contain %q, got:\n%s", snippet, formatted)
		}
	}
}

func TestFormatUserPreferencesContainsExpectedSections(t *testing.T) {
	prefs := []UserPreference{
		{Key: "language", Value: "zh-CN", SourceText: "以后都用中文"},
		{Key: "verbosity", Value: "low", SourceText: "回答简洁"},
	}

	formatted := FormatUserPreferences(prefs)
	expectedSnippets := []string{
		"[User Preferences]",
		"Long-term explicit user preferences",
		"- language: zh-CN",
		"- verbosity: low",
		"source: \"以后都用中文\"",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(formatted, snippet) {
			t.Fatalf("expected formatted preferences to contain %q, got:\n%s", snippet, formatted)
		}
	}
}

func TestFormatProjectFactsContainsExpectedSections(t *testing.T) {
	facts := []ProjectFact{
		{Key: "pg_truth", Value: "PG is Truth", SourceText: "已确认：PG is Truth"},
		{Key: "redis_role", Value: "Redis 只做热层", SourceText: "最终决定：Redis 只做热层"},
	}

	formatted := FormatProjectFacts(facts)
	expectedSnippets := []string{
		"[Project Facts]",
		"Stable confirmed architecture/workflow facts",
		"- pg_truth: PG is Truth",
		"- redis_role: Redis 只做热层",
		"source: \"已确认：PG is Truth\"",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(formatted, snippet) {
			t.Fatalf("expected formatted project facts to contain %q, got:\n%s", snippet, formatted)
		}
	}
}

func TestPruneSummaryItemsReferencingFactValues(t *testing.T) {
	items := []string{
		"已决定：PG is Truth 作为唯一事实来源",
		"保留：继续执行 smoke",
		"阻塞：Redis 只做热层导致未命中",
	}
	factValues := []string{"pg is truth", "redis 只做热层"}

	got := pruneSummaryItemsReferencingFactValues(items, factValues)
	if len(got) != 1 {
		t.Fatalf("expected 1 item after prune, got %d (%#v)", len(got), got)
	}
	if strings.TrimSpace(got[0]) != "保留：继续执行 smoke" {
		t.Fatalf("expected remaining item to be unrelated one, got %q", got[0])
	}
}

func containsExact(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}
