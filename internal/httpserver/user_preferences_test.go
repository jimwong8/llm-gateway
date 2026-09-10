package httpserver

import (
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestExtractExplicitUserPreferences(t *testing.T) {
	req := providers.ChatCompletionRequest{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "以后都用中文回答"},
			{Role: "user", Content: "回答简洁一点"},
			{Role: "user", Content: "不要频繁确认"},
		},
	}

	prefs := extractExplicitUserPreferences(req)
	if len(prefs) != 3 {
		t.Fatalf("expected 3 preferences, got %d", len(prefs))
	}

	if prefs[0].Key != "language" || prefs[0].Value != "zh-CN" {
		t.Fatalf("expected language=zh-CN, got %s=%s", prefs[0].Key, prefs[0].Value)
	}
	if prefs[1].Key != "verbosity" || prefs[1].Value != "low" {
		t.Fatalf("expected verbosity=low, got %s=%s", prefs[1].Key, prefs[1].Value)
	}
	if prefs[2].Key != "confirmation" || prefs[2].Value != "minimal" {
		t.Fatalf("expected confirmation=minimal, got %s=%s", prefs[2].Key, prefs[2].Value)
	}
}

func TestExtractExplicitUserPreferencesIgnoresNonExplicitSignals(t *testing.T) {
	req := providers.ChatCompletionRequest{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "这次先用中文吧"},
			{Role: "user", Content: "可以稍微短一点吗"},
			{Role: "assistant", Content: "不要频繁确认"},
		},
	}

	prefs := extractExplicitUserPreferences(req)
	if len(prefs) != 0 {
		t.Fatalf("expected no explicit long-term preferences, got %#v", prefs)
	}
}
