package memory

import (
	"testing"
)

func TestApplyMaskRules_NoRulesReturnsOriginalText(t *testing.T) {
	result := applyMaskRules("hello world", nil)
	if result != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", result)
	}
}

func TestApplyMaskRules_AppliesRulesInOrder(t *testing.T) {
	rules := []MaskRule{
		{Pattern: "foo", Replace: "bar"},
		{Pattern: "bar", Replace: "baz"},
	}
	result := applyMaskRules("foo", rules)
	if result != "baz" {
		t.Fatalf("expected %q, got %q", "baz", result)
	}
}

func TestApplyMaskRules_EmptyPatternExpandsByReplaceAllSemantics(t *testing.T) {
	rules := []MaskRule{
		{Pattern: "", Replace: "X"},
	}
	result := applyMaskRules("ab", rules)
	if result != "XaXbX" {
		t.Fatalf("expected %q, got %q", "XaXbX", result)
	}
}
