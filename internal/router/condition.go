package router

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ConditionType string

const (
	ConditionHeader     ConditionType = "header"
	ConditionBody       ConditionType = "body"
	ConditionUserRole   ConditionType = "user_role"
	ConditionTimeWindow ConditionType = "time_window"
	ConditionTags       ConditionType = "tags"
	ConditionAlways     ConditionType = "always"
)

type RouteCondition struct {
	Type       ConditionType     `json:"type"`
	Field      string            `json:"field,omitempty"`
	Operator   string            `json:"operator,omitempty"`
	Value      string            `json:"value,omitempty"`
	Conditions []RouteCondition  `json:"conditions,omitempty"`
	Logic      string            `json:"logic,omitempty"` // "and" | "or"
}

type RoutingRule struct {
	Name       string            `json:"name"`
	Priority   int               `json:"priority"`
	Condition  RouteCondition    `json:"condition"`
	Providers  []string          `json:"providers"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ConditionEvaluator struct {
	rules []RoutingRule
}

func NewConditionEvaluator(rules []RoutingRule) *ConditionEvaluator {
	sorted := make([]RoutingRule, len(rules))
	copy(sorted, rules)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority > sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return &ConditionEvaluator{rules: sorted}
}

type EvalContext struct {
	Headers    map[string]string
	BodyFields map[string]string
	UserRole   string
	Hour       int
	Tags       []string
}

func (e *ConditionEvaluator) Evaluate(ctx EvalContext) []string {
	for _, rule := range e.rules {
		if e.matchCondition(rule.Condition, ctx) {
			return rule.Providers
		}
	}
	return nil
}

func (e *ConditionEvaluator) matchCondition(cond RouteCondition, ctx EvalContext) bool {
	switch cond.Type {
	case ConditionAlways:
		return true
	case ConditionHeader:
		val, ok := ctx.Headers[cond.Field]
		if !ok {
			return false
		}
		return compare(val, cond.Operator, cond.Value)
	case ConditionBody:
		val, ok := ctx.BodyFields[cond.Field]
		if !ok {
			return false
		}
		return compare(val, cond.Operator, cond.Value)
	case ConditionUserRole:
		return compare(ctx.UserRole, cond.Operator, cond.Value)
	case ConditionTimeWindow:
		if cond.Operator == "between" {
			parts := strings.Split(cond.Value, "-")
			if len(parts) == 2 {
				var start, end int
				fmt.Sscanf(parts[0], "%d", &start)
				fmt.Sscanf(parts[1], "%d", &end)
				return ctx.Hour >= start && ctx.Hour <= end
			}
		}
		return false
	case ConditionTags:
		for _, tag := range ctx.Tags {
			if compare(tag, cond.Operator, cond.Value) {
				return true
			}
		}
		return false
	default:
		if len(cond.Conditions) > 0 {
			return e.matchCompound(cond, ctx)
		}
		return false
	}
}

func (e *ConditionEvaluator) matchCompound(cond RouteCondition, ctx EvalContext) bool {
	if len(cond.Conditions) == 0 {
		return true
	}
	logic := strings.ToLower(cond.Logic)
	if logic == "or" {
		for _, c := range cond.Conditions {
			if e.matchCondition(c, ctx) {
				return true
			}
		}
		return false
	}
	for _, c := range cond.Conditions {
		if !e.matchCondition(c, ctx) {
			return false
		}
	}
	return true
}

func compare(actual, operator, expected string) bool {
	switch operator {
	case "==", "eq":
		return actual == expected
	case "!=", "ne":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	case "starts_with":
		return strings.HasPrefix(actual, expected)
	case "ends_with":
		return strings.HasSuffix(actual, expected)
	case "in":
		parts := strings.Split(expected, ",")
		for _, p := range parts {
			if strings.TrimSpace(p) == actual {
				return true
			}
		}
		return false
	default:
		return actual == expected
	}
}

func ParseRoutingRulesJSON(data []byte) ([]RoutingRule, error) {
	var rules []RoutingRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("parse routing rules: %w", err)
	}
	return rules, nil
}
