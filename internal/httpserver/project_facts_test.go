package httpserver

import (
	"strings"
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestExtractConfirmedProjectFacts(t *testing.T) {
	req := providers.ChatCompletionRequest{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "已确认：PG is Truth，后续都以此为准"},
			{Role: "assistant", Content: "最终决定：Redis 只做热层"},
			{Role: "assistant", Content: "结论：Oracle 审查默认拆小并行"},
		},
	}

	facts := extractConfirmedProjectFacts(req)
	if len(facts) != 3 {
		t.Fatalf("expected 3 facts, got %d", len(facts))
	}
	if facts[0].Key != "pg_truth" || facts[0].Value != "PG is Truth" {
		t.Fatalf("expected pg_truth fact, got %s=%s", facts[0].Key, facts[0].Value)
	}
	if facts[1].Key != "redis_role" || facts[1].Value != "Redis 只做热层" {
		t.Fatalf("expected redis_role fact, got %s=%s", facts[1].Key, facts[1].Value)
	}
	if facts[2].Key != "oracle_review_mode" || facts[2].Value != "Oracle 审查默认拆小并行" {
		t.Fatalf("expected oracle_review_mode fact, got %s=%s", facts[2].Key, facts[2].Value)
	}
}

func TestExtractConfirmedProjectFactsIgnoresTentativeStatements(t *testing.T) {
	req := providers.ChatCompletionRequest{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "候选方案：PG is Truth"},
			{Role: "assistant", Content: "我们先讨论下 Redis 只做热层"},
			{Role: "user", Content: "maybe Oracle 审查默认拆小并行"},
		},
	}

	facts := extractConfirmedProjectFacts(req)
	if len(facts) != 0 {
		t.Fatalf("expected no confirmed project facts, got %#v", facts)
	}
}

func TestExtractProjectFactKV(t *testing.T) {
	cases := []struct {
		content string
		key     string
		value   string
		ok      bool
	}{
		{content: "已确认 PG is Truth", key: "pg_truth", value: "PG is Truth", ok: true},
		{content: "最终决定 Redis 只做热层", key: "redis_role", value: "Redis 只做热层", ok: true},
		{content: "结论：Oracle 审查默认拆小并行", key: "oracle_review_mode", value: "Oracle 审查默认拆小并行", ok: true},
		{content: "普通描述，不是事实", key: "", value: "", ok: false},
	}
	for _, tc := range cases {
		key, value, ok := extractProjectFactKV(tc.content)
		if ok != tc.ok || key != tc.key || value != tc.value {
			t.Fatalf("extractProjectFactKV(%q) => (%q,%q,%v), expected (%q,%q,%v)", tc.content, key, value, ok, tc.key, tc.value, tc.ok)
		}
	}
}

func TestProjectFactSignalHelpers(t *testing.T) {
	if !isConfirmedProjectFactSignal("最终决定：PG is Truth") {
		t.Fatal("expected confirmed signal true")
	}
	if isTentativeProjectFactSignal("最终决定：PG is Truth") {
		t.Fatal("expected tentative signal false")
	}
	if !isTentativeProjectFactSignal("候选方案：PG is Truth") {
		t.Fatal("expected tentative signal true")
	}
	if isConfirmedProjectFactSignal("我们讨论一下") {
		t.Fatal("expected confirmed signal false")
	}

	if !hasAnySignal(strings.ToLower("Redis 只做热层"), []string{"redis 只做热层"}) {
		t.Fatal("expected hasAnySignal to match")
	}
}
