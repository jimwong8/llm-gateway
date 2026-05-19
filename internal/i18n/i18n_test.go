package i18n

import "testing"

func TestT_zhCN(t *testing.T) {
	got := T("zh-CN", "resource_not_found")
	if got != "资源未找到" {
		t.Fatalf("expected 资源未找到, got %s", got)
	}
}

func TestT_enUS(t *testing.T) {
	got := T("en-US", "resource_not_found")
	if got != "resource not found" {
		t.Fatalf("expected resource not found, got %s", got)
	}
}

func TestT_fallback(t *testing.T) {
	got := T("zh-CN", "nonexistent_key")
	if got != "nonexistent_key" {
		t.Fatalf("expected key itself, got %s", got)
	}
}

func TestT_enUSFallback(t *testing.T) {
	got := T("fr-FR", "resource_not_found")
	if got != "resource not found" {
		t.Fatalf("expected en-US fallback, got %s", got)
	}
}

func TestAcceptLanguage_zh(t *testing.T) {
	if got := AcceptLanguage("zh-CN,zh;q=0.9"); got != "zh-CN" {
		t.Fatalf("expected zh-CN, got %s", got)
	}
	if got := AcceptLanguage("zh"); got != "zh-CN" {
		t.Fatalf("expected zh-CN, got %s", got)
	}
}

func TestAcceptLanguage_en(t *testing.T) {
	if got := AcceptLanguage("en-US,en;q=0.9"); got != "en-US" {
		t.Fatalf("expected en-US, got %s", got)
	}
	if got := AcceptLanguage("en"); got != "en-US" {
		t.Fatalf("expected en-US, got %s", got)
	}
}

func TestAcceptLanguage_empty(t *testing.T) {
	if got := AcceptLanguage(""); got != "zh-CN" {
		t.Fatalf("expected zh-CN default, got %s", got)
	}
}

func TestAcceptLanguage_other(t *testing.T) {
	if got := AcceptLanguage("fr-FR"); got != "zh-CN" {
		t.Fatalf("expected zh-CN default, got %s", got)
	}
}
