package abtest

import (
	"fmt"
	"hash/fnv"
	"sync"
)

// ExperimentStatus 表示实验状态
type ExperimentStatus string

const (
	StatusRunning ExperimentStatus = "running"
	StatusStopped ExperimentStatus = "stopped"
)

// Variant 表示实验的一个变体
type Variant struct {
	Model  string  `json:"model"`
	Weight float64 `json:"weight"`
}

// Experiment 表示一个 A/B 测试实验
type Experiment struct {
	Name     string           `json:"name"`
	Variants []Variant        `json:"variants"`
	Status   ExperimentStatus `json:"status"`
}

// VariantResult 存储单个变体的指标汇总
type VariantResult struct {
	Model        string  `json:"model"`
	Assignments  int     `json:"assignments"`
	TotalMetric  float64 `json:"total_metric"`
	AvgMetric    float64 `json:"avg_metric"`
}

// ExperimentResult 存储实验的整体结果
type ExperimentResult struct {
	Name     string          `json:"name"`
	Status   string          `json:"status"`
	Variants []VariantResult `json:"variants"`
}

// assignmentRecord 记录用户分配到哪个变体
type assignmentRecord struct {
	experiment string
	variant    string
}

// outcomeRecord 记录单次指标
type outcomeRecord struct {
	experimentName string
	variant        string
	metric         float64
}

// Service 管理 A/B 测试实验
type Service struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	assignments map[string]assignmentRecord // key: userID|experimentName
	outcomes    []outcomeRecord
}

// NewService 创建新的 A/B 测试服务
func NewService() *Service {
	return &Service{
		experiments: make(map[string]*Experiment),
		assignments: make(map[string]assignmentRecord),
		outcomes:    make([]outcomeRecord, 0),
	}
}

// CreateExperiment 创建新实验
func (s *Service) CreateExperiment(name string, variants []Variant) error {
	if name == "" {
		return fmt.Errorf("experiment name cannot be empty")
	}
	if len(variants) == 0 {
		return fmt.Errorf("variants cannot be empty")
	}

	totalWeight := 0.0
	for _, v := range variants {
		if v.Model == "" {
			return fmt.Errorf("variant model cannot be empty")
		}
		if v.Weight < 0 {
			return fmt.Errorf("variant weight cannot be negative")
		}
		totalWeight += v.Weight
	}
	if totalWeight <= 0 {
		return fmt.Errorf("total weight must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.experiments[name]; exists {
		return fmt.Errorf("experiment %q already exists", name)
	}

	s.experiments[name] = &Experiment{
		Name:     name,
		Variants: variants,
		Status:   StatusRunning,
	}
	return nil
}

// ListExperiments 列出所有实验
func (s *Service) ListExperiments() []Experiment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Experiment, 0, len(s.experiments))
	for _, exp := range s.experiments {
		result = append(result, *exp)
	}
	return result
}

// GetExperiment 获取指定实验
func (s *Service) GetExperiment(name string) (*Experiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exp, exists := s.experiments[name]
	if !exists {
		return nil, fmt.Errorf("experiment %q not found", name)
	}
	copy := *exp
	return &copy, nil
}

// StopExperiment 停止实验
func (s *Service) StopExperiment(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, exists := s.experiments[name]
	if !exists {
		return fmt.Errorf("experiment %q not found", name)
	}
	exp.Status = StatusStopped
	return nil
}

// AssignUser 将用户分配到实验的某个变体（一致性哈希）
func (s *Service) AssignUser(userID, experimentName string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("userID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	exp, exists := s.experiments[experimentName]
	if !exists {
		return "", fmt.Errorf("experiment %q not found", experimentName)
	}
	if exp.Status != StatusRunning {
		return "", fmt.Errorf("experiment %q is not running", experimentName)
	}

	// 检查是否已分配
	assignKey := userID + "|" + experimentName
	if rec, exists := s.assignments[assignKey]; exists {
		return rec.variant, nil
	}

	// 一致性哈希：根据 userID 选择变体
	variant := s.selectVariant(userID, exp)
	s.assignments[assignKey] = assignmentRecord{
		experiment: experimentName,
		variant:    variant,
	}
	return variant, nil
}

// selectVariant 基于一致性哈希选择变体（调用者需持有写锁）
func (s *Service) selectVariant(userID string, exp *Experiment) string {
	totalWeight := 0.0
	for _, v := range exp.Variants {
		totalWeight += v.Weight
	}

	// 使用 FNV 哈希将 userID 映射到 [0, totalWeight)
	h := fnv.New64a()
	h.Write([]byte(userID + "|" + exp.Name))
	hashVal := float64(h.Sum64()%10000) / 10000.0 * totalWeight

	cumulative := 0.0
	for _, v := range exp.Variants {
		cumulative += v.Weight
		if hashVal < cumulative {
			return v.Model
		}
	}
	// 兜底返回最后一个变体
	return exp.Variants[len(exp.Variants)-1].Model
}

// RecordOutcome 记录指标
func (s *Service) RecordOutcome(experimentName, variant string, metric float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.experiments[experimentName]; !exists {
		return fmt.Errorf("experiment %q not found", experimentName)
	}

	s.outcomes = append(s.outcomes, outcomeRecord{
		experimentName: experimentName,
		variant:        variant,
		metric:         metric,
	})
	return nil
}

// GetResults 获取实验结果汇总
func (s *Service) GetResults(experimentName string) (*ExperimentResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exp, exists := s.experiments[experimentName]
	if !exists {
		return nil, fmt.Errorf("experiment %q not found", experimentName)
	}

	// 统计每个变体的指标
	type agg struct {
		assignments int
		totalMetric float64
	}
	aggMap := make(map[string]*agg)

	// 初始化所有变体
	for _, v := range exp.Variants {
		aggMap[v.Model] = &agg{}
	}

	// 统计分配次数
	for _, rec := range s.assignments {
		if rec.experiment == experimentName {
			if a, ok := aggMap[rec.variant]; ok {
				a.assignments++
			}
		}
	}

	// 统计指标
	for _, o := range s.outcomes {
		if o.experimentName == experimentName {
			if a, ok := aggMap[o.variant]; ok {
				a.totalMetric += o.metric
			}
		}
	}

	// 构建结果
	variantResults := make([]VariantResult, 0, len(exp.Variants))
	for _, v := range exp.Variants {
		a := aggMap[v.Model]
		avg := 0.0
		if a.assignments > 0 {
			avg = a.totalMetric / float64(a.assignments)
		}
		variantResults = append(variantResults, VariantResult{
			Model:       v.Model,
			Assignments: a.assignments,
			TotalMetric: a.totalMetric,
			AvgMetric:   avg,
		})
	}

	return &ExperimentResult{
		Name:     exp.Name,
		Status:   string(exp.Status),
		Variants: variantResults,
	}, nil
}
