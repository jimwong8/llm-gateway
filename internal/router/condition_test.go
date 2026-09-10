package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConditionEvaluator_Always(t *testing.T) {
	rules := []RoutingRule{
		{Name: "default", Priority: 0, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"openai"}},
	}
	eval := NewConditionEvaluator(rules)
	result := eval.Evaluate(EvalContext{})
	if len(result) != 1 || result[0] != "openai" {
		t.Fatalf("expected [openai], got %v", result)
	}
}

func TestConditionEvaluator_HeaderMatch(t *testing.T) {
	rules := []RoutingRule{
		{Name: "premium", Priority: 10, Condition: RouteCondition{Type: ConditionHeader, Field: "X-Model-Type", Operator: "==", Value: "chat"}, Providers: []string{"anthropic"}},
		{Name: "default", Priority: 0, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"openai"}},
	}
	eval := NewConditionEvaluator(rules)

	result := eval.Evaluate(EvalContext{Headers: map[string]string{"X-Model-Type": "chat"}})
	if len(result) != 1 || result[0] != "anthropic" {
		t.Fatalf("expected [anthropic], got %v", result)
	}

	result = eval.Evaluate(EvalContext{Headers: map[string]string{"X-Model-Type": "other"}})
	if len(result) != 1 || result[0] != "openai" {
		t.Fatalf("expected [openai], got %v", result)
	}
}

func TestConditionEvaluator_BodyField(t *testing.T) {
	rules := []RoutingRule{
		{Name: "gpt4", Priority: 10, Condition: RouteCondition{Type: ConditionBody, Field: "model", Operator: "starts_with", Value: "gpt-4"}, Providers: []string{"openai-primary"}},
		{Name: "default", Priority: 0, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"openai-fallback"}},
	}
	eval := NewConditionEvaluator(rules)

	result := eval.Evaluate(EvalContext{BodyFields: map[string]string{"model": "gpt-4o-mini"}})
	if len(result) != 1 || result[0] != "openai-primary" {
		t.Fatalf("expected [openai-primary], got %v", result)
	}
}

func TestConditionEvaluator_UserRole(t *testing.T) {
	rules := []RoutingRule{
		{Name: "premium", Priority: 10, Condition: RouteCondition{Type: ConditionUserRole, Operator: "==", Value: "premium"}, Providers: []string{"anthropic", "openai"}},
		{Name: "default", Priority: 0, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"openai"}},
	}
	eval := NewConditionEvaluator(rules)

	result := eval.Evaluate(EvalContext{UserRole: "premium"})
	if len(result) != 2 {
		t.Fatalf("expected 2 providers, got %v", result)
	}
}

func TestConditionEvaluator_TimeWindow(t *testing.T) {
	rules := []RoutingRule{
		{Name: "business-hours", Priority: 10, Condition: RouteCondition{Type: ConditionTimeWindow, Operator: "between", Value: "9-18"}, Providers: []string{"premium-provider"}},
		{Name: "default", Priority: 0, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"standard-provider"}},
	}
	eval := NewConditionEvaluator(rules)

	result := eval.Evaluate(EvalContext{Hour: 14})
	if len(result) != 1 || result[0] != "premium-provider" {
		t.Fatalf("expected [premium-provider], got %v", result)
	}

	result = eval.Evaluate(EvalContext{Hour: 22})
	if len(result) != 1 || result[0] != "standard-provider" {
		t.Fatalf("expected [standard-provider], got %v", result)
	}
}

func TestConditionEvaluator_Tags(t *testing.T) {
	rules := []RoutingRule{
		{Name: "prod", Priority: 10, Condition: RouteCondition{Type: ConditionTags, Operator: "==", Value: "production"}, Providers: []string{"prod-provider"}},
		{Name: "default", Priority: 0, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"dev-provider"}},
	}
	eval := NewConditionEvaluator(rules)

	result := eval.Evaluate(EvalContext{Tags: []string{"production", "v2"}})
	if len(result) != 1 || result[0] != "prod-provider" {
		t.Fatalf("expected [prod-provider], got %v", result)
	}
}

func TestConditionEvaluator_CompoundAND(t *testing.T) {
	rules := []RoutingRule{
		{
			Name:     "premium-chat",
			Priority: 10,
			Condition: RouteCondition{
				Logic: "and",
				Conditions: []RouteCondition{
					{Type: ConditionUserRole, Operator: "==", Value: "premium"},
					{Type: ConditionHeader, Field: "X-Model-Type", Operator: "==", Value: "chat"},
				},
			},
			Providers: []string{"anthropic"},
		},
		{Name: "default", Priority: 0, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"openai"}},
	}
	eval := NewConditionEvaluator(rules)

	result := eval.Evaluate(EvalContext{UserRole: "premium", Headers: map[string]string{"X-Model-Type": "chat"}})
	if len(result) != 1 || result[0] != "anthropic" {
		t.Fatalf("expected [anthropic], got %v", result)
	}

	result = eval.Evaluate(EvalContext{UserRole: "basic", Headers: map[string]string{"X-Model-Type": "chat"}})
	if len(result) != 1 || result[0] != "openai" {
		t.Fatalf("expected [openai], got %v", result)
	}
}

func TestConditionEvaluator_CompoundOR(t *testing.T) {
	rules := []RoutingRule{
		{
			Name:     "special",
			Priority: 10,
			Condition: RouteCondition{
				Logic: "or",
				Conditions: []RouteCondition{
					{Type: ConditionUserRole, Operator: "==", Value: "premium"},
					{Type: ConditionTags, Operator: "==", Value: "vip"},
				},
			},
			Providers: []string{"special-provider"},
		},
		{Name: "default", Priority: 0, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"openai"}},
	}
	eval := NewConditionEvaluator(rules)

	result := eval.Evaluate(EvalContext{UserRole: "basic", Tags: []string{"vip"}})
	if len(result) != 1 || result[0] != "special-provider" {
		t.Fatalf("expected [special-provider], got %v", result)
	}
}

func TestConditionEvaluator_Priority(t *testing.T) {
	rules := []RoutingRule{
		{Name: "low", Priority: 1, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"low"}},
		{Name: "high", Priority: 100, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"high"}},
		{Name: "mid", Priority: 50, Condition: RouteCondition{Type: ConditionAlways}, Providers: []string{"mid"}},
	}
	eval := NewConditionEvaluator(rules)
	result := eval.Evaluate(EvalContext{})
	if len(result) != 1 || result[0] != "high" {
		t.Fatalf("expected [high], got %v", result)
	}
}

func TestParseRoutingRulesJSON(t *testing.T) {
	data := `[
		{"name": "rule1", "priority": 10, "condition": {"type": "header", "field": "X-Type", "operator": "==", "value": "chat"}, "providers": ["openai"]},
		{"name": "rule2", "priority": 0, "condition": {"type": "always"}, "providers": ["fallback"]}
	]`
	rules, err := ParseRoutingRulesJSON([]byte(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Name != "rule1" {
		t.Fatalf("expected rule1, got %s", rules[0].Name)
	}
}

func TestHookPipeline(t *testing.T) {
	pipeline := NewHookPipeline()
	hook := &LoggingHook{}
	pipeline.Register(hook)

	req := &HookRequest{Request: httptest.NewRequest(http.MethodGet, "/test", nil), UserID: 1}
	resp := &HookResponse{StatusCode: 200}

	err := pipeline.Before(context.Background(), req)
	if err != nil {
		t.Fatalf("before error: %v", err)
	}
	err = pipeline.After(context.Background(), req, resp)
	if err != nil {
		t.Fatalf("after error: %v", err)
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		actual, operator, expected string
		want                       bool
	}{
		{"a", "==", "a", true},
		{"a", "==", "b", false},
		{"a", "!=", "b", true},
		{"hello", "contains", "ell", true},
		{"gpt-4", "starts_with", "gpt", true},
		{"file.txt", "ends_with", ".txt", true},
		{"a", "in", "a,b,c", true},
		{"d", "in", "a,b,c", false},
	}
	for _, tt := range tests {
		got := compare(tt.actual, tt.operator, tt.expected)
		if got != tt.want {
			t.Errorf("compare(%q, %q, %q) = %v, want %v", tt.actual, tt.operator, tt.expected, got, tt.want)
		}
	}
}
