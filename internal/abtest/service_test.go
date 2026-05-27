package abtest

import (
	"fmt"
	"testing"
)

func TestCreateExperiment(t *testing.T) {
	svc := NewService()

	// 正常创建
	err := svc.CreateExperiment("exp1", []Variant{
		{Model: "gpt-4", Weight: 0.5},
		{Model: "gpt-3.5", Weight: 0.5},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 重复名称
	err = svc.CreateExperiment("exp1", []Variant{
		{Model: "gpt-4", Weight: 1.0},
	})
	if err == nil {
		t.Fatal("expected error for duplicate experiment name")
	}

	// 空名称
	err = svc.CreateExperiment("", []Variant{
		{Model: "gpt-4", Weight: 1.0},
	})
	if err == nil {
		t.Fatal("expected error for empty experiment name")
	}

	// 空变体
	err = svc.CreateExperiment("exp2", []Variant{})
	if err == nil {
		t.Fatal("expected error for empty variants")
	}

	// 负权重
	err = svc.CreateExperiment("exp2", []Variant{
		{Model: "gpt-4", Weight: -1.0},
	})
	if err == nil {
		t.Fatal("expected error for negative weight")
	}

	// 零总权重
	err = svc.CreateExperiment("exp2", []Variant{
		{Model: "gpt-4", Weight: 0},
		{Model: "gpt-3.5", Weight: 0},
	})
	if err == nil {
		t.Fatal("expected error for zero total weight")
	}

	// 空模型名
	err = svc.CreateExperiment("exp2", []Variant{
		{Model: "", Weight: 1.0},
	})
	if err == nil {
		t.Fatal("expected error for empty model name")
	}
}

func TestListExperiments(t *testing.T) {
	svc := NewService()

	if len(svc.ListExperiments()) != 0 {
		t.Fatal("expected empty list")
	}

	svc.CreateExperiment("exp1", []Variant{
		{Model: "gpt-4", Weight: 1.0},
	})

	list := svc.ListExperiments()
	if len(list) != 1 {
		t.Fatalf("expected 1 experiment, got %d", len(list))
	}
	if list[0].Name != "exp1" {
		t.Fatalf("expected name exp1, got %s", list[0].Name)
	}
}

func TestGetExperiment(t *testing.T) {
	svc := NewService()
	svc.CreateExperiment("exp1", []Variant{
		{Model: "gpt-4", Weight: 0.5},
		{Model: "gpt-3.5", Weight: 0.5},
	})

	exp, err := svc.GetExperiment("exp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exp.Name != "exp1" {
		t.Fatalf("expected name exp1, got %s", exp.Name)
	}
	if len(exp.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(exp.Variants))
	}

	// 不存在的实验
	_, err = svc.GetExperiment("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent experiment")
	}
}

func TestStopExperiment(t *testing.T) {
	svc := NewService()
	svc.CreateExperiment("exp1", []Variant{
		{Model: "gpt-4", Weight: 1.0},
	})

	err := svc.StopExperiment("exp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exp, _ := svc.GetExperiment("exp1")
	if exp.Status != StatusStopped {
		t.Fatalf("expected stopped status, got %s", exp.Status)
	}

	// 停止不存在的实验
	err = svc.StopExperiment("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent experiment")
	}
}

func TestAssignUser(t *testing.T) {
	svc := NewService()
	svc.CreateExperiment("exp1", []Variant{
		{Model: "gpt-4", Weight: 0.5},
		{Model: "gpt-3.5", Weight: 0.5},
	})

	// 正常分配
	variant, err := svc.AssignUser("user1", "exp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if variant != "gpt-4" && variant != "gpt-3.5" {
		t.Fatalf("unexpected variant: %s", variant)
	}

	// 一致性：同一用户多次分配应返回相同结果
	variant2, err := svc.AssignUser("user1", "exp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if variant != variant2 {
		t.Fatalf("inconsistent assignment: %s vs %s", variant, variant2)
	}

	// 不同用户可能分配到不同变体（概率性，但权重各0.5时大概率不同）
	variant3, err := svc.AssignUser("user2", "exp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if variant3 != "gpt-4" && variant3 != "gpt-3.5" {
		t.Fatalf("unexpected variant: %s", variant3)
	}

	// 空 userID
	_, err = svc.AssignUser("", "exp1")
	if err == nil {
		t.Fatal("expected error for empty userID")
	}

	// 不存在的实验
	_, err = svc.AssignUser("user1", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent experiment")
	}

	// 已停止的实验
	svc.StopExperiment("exp1")
	_, err = svc.AssignUser("user3", "exp1")
	if err == nil {
		t.Fatal("expected error for stopped experiment")
	}
}

func TestAssignUserDistribution(t *testing.T) {
	svc := NewService()
	svc.CreateExperiment("dist-exp", []Variant{
		{Model: "model-a", Weight: 0.7},
		{Model: "model-b", Weight: 0.3},
	})

	counts := map[string]int{"model-a": 0, "model-b": 0}
	for i := 0; i < 1000; i++ {
		v, err := svc.AssignUser(fmt.Sprintf("user-%d", i), "dist-exp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[v]++
	}

	// 允许一定误差范围（70% ± 10%）
	if counts["model-a"] < 600 || counts["model-a"] > 800 {
		t.Fatalf("model-a distribution out of range: %d/1000", counts["model-a"])
	}
	if counts["model-b"] < 200 || counts["model-b"] > 400 {
		t.Fatalf("model-b distribution out of range: %d/1000", counts["model-b"])
	}
}

func TestRecordOutcome(t *testing.T) {
	svc := NewService()
	svc.CreateExperiment("exp1", []Variant{
		{Model: "gpt-4", Weight: 0.5},
		{Model: "gpt-3.5", Weight: 0.5},
	})

	err := svc.RecordOutcome("exp1", "gpt-4", 0.95)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.RecordOutcome("exp1", "gpt-3.5", 0.85)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 不存在的实验
	err = svc.RecordOutcome("nonexistent", "gpt-4", 0.5)
	if err == nil {
		t.Fatal("expected error for nonexistent experiment")
	}
}

func TestGetResults(t *testing.T) {
	svc := NewService()
	svc.CreateExperiment("exp1", []Variant{
		{Model: "gpt-4", Weight: 0.5},
		{Model: "gpt-3.5", Weight: 0.5},
	})

	// 分配用户
	svc.AssignUser("user1", "exp1")
	svc.AssignUser("user2", "exp1")
	svc.AssignUser("user3", "exp1")

	// 记录指标
	svc.RecordOutcome("exp1", "gpt-4", 0.9)
	svc.RecordOutcome("exp1", "gpt-4", 0.8)
	svc.RecordOutcome("exp1", "gpt-3.5", 0.7)

	results, err := svc.GetResults("exp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results.Name != "exp1" {
		t.Fatalf("expected name exp1, got %s", results.Name)
	}
	if results.Status != "running" {
		t.Fatalf("expected running status, got %s", results.Status)
	}
	if len(results.Variants) != 2 {
		t.Fatalf("expected 2 variant results, got %d", len(results.Variants))
	}

	// 验证各变体的指标
	const epsilon = 1e-9
	for _, vr := range results.Variants {
		if vr.Model == "gpt-4" {
			if diff := vr.TotalMetric - 1.7; diff < -epsilon || diff > epsilon {
				t.Fatalf("expected total metric 1.7 for gpt-4, got %f", vr.TotalMetric)
			}
		}
		if vr.Model == "gpt-3.5" {
			if diff := vr.TotalMetric - 0.7; diff < -epsilon || diff > epsilon {
				t.Fatalf("expected total metric 0.7 for gpt-3.5, got %f", vr.TotalMetric)
			}
		}
	}

	// 不存在的实验
	_, err = svc.GetResults("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent experiment")
	}
}

func TestGetResultsEmptyExperiment(t *testing.T) {
	svc := NewService()
	svc.CreateExperiment("empty-exp", []Variant{
		{Model: "model-a", Weight: 1.0},
	})

	results, err := svc.GetResults("empty-exp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results.Variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(results.Variants))
	}
	if results.Variants[0].Assignments != 0 {
		t.Fatalf("expected 0 assignments, got %d", results.Variants[0].Assignments)
	}
	if results.Variants[0].AvgMetric != 0 {
		t.Fatalf("expected 0 avg metric, got %f", results.Variants[0].AvgMetric)
	}
}
