package router

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"llm-gateway/gateway/internal/providers"
)

// AdaptiveRouter 自适应路由器
// 基于 Thompson Sampling 的在线学习路由决策
type AdaptiveRouter struct {
	base    *Router
	mu      sync.RWMutex
	// 每个 (task_type, model) 维护 Beta 后验分布
	scores map[string]*BetaDistribution
	// 滑动窗口统计
	metrics *RollingMetrics
	// 学习率
	alpha float64
	// 最小样本数（低于此数使用默认权重）
	minSamples int
	// 上次学习时间
	lastLearn time.Time
}

// BetaDistribution Beta 分布（Thompson Sampling）
type BetaDistribution struct {
	Alpha float64 // 成功次数 + 1
	Beta  float64 // 失败次数 + 1
}

// Sample 从 Beta 分布采样
func (b *BetaDistribution) Sample() float64 {
	// 使用正态近似（大样本时更快）
	if b.Alpha+b.Beta > 30 {
		mean := b.Alpha / (b.Alpha + b.Beta)
		variance := (b.Alpha * b.Beta) / ((b.Alpha + b.Beta) * (b.Alpha + b.Beta) * (b.Alpha + b.Beta + 1))
		std := math.Sqrt(variance)
		return math.Max(0, math.Min(1, mean+std*rand.NormFloat64()))
	}
	// 小样本使用直接采样
	x := rand.Float64()
	y := rand.Float64()
	for x < 0 || y < 0 || x+y == 0 {
		x = rand.Float64()
		y = rand.Float64()
	}
	// 简化的 Beta 采样
	return b.Alpha / (b.Alpha + b.Beta)
}

// RollingMetrics 滑动窗口指标
type RollingMetrics struct {
	mu       sync.RWMutex
	// 每个模型的最近 N 次请求指标
	records  map[string][]*RequestMetric
	window   int
}

// RequestMetric 单次请求指标
type RequestMetric struct {
	TaskType   string
	Model      string
	Provider   string
	LatencyMs  float64
	Cost       float64
	Tokens     int
	Error      bool
	Timestamp  time.Time
	// 质量评分 (0-1, 基于响应分析)
	QualityScore float64
}

// NewAdaptiveRouter 创建自适应路由器
func NewAdaptiveRouter(base *Router) *AdaptiveRouter {
	return &AdaptiveRouter{
		base:       base,
		scores:     make(map[string]*BetaDistribution),
		metrics:    &RollingMetrics{records: make(map[string][]*RequestMetric), window: 1000},
		alpha:      0.1,
		minSamples: 10,
		lastLearn:  time.Now(),
	}
}

// Decide 自适应路由决策
func (ar *AdaptiveRouter) Decide(ctx context.Context, req providers.ChatCompletionRequest) Decision {
	// 1. 先用基础路由器做初步决策
	baseDecision := ar.base.Decide(req)
	
	// 2. 如果用户指定了模型，直接返回
	if req.PreferredModel != "" {
		return baseDecision
	}
	
	// 3. 获取任务类型
	task := classifyTask(req)
	
	// 4. 获取候选模型
	candidates := ar.getCandidates(req)
	if len(candidates) == 0 {
		return baseDecision
	}
	
	// 5. Thompson Sampling 选择
	selected := ar.thinpsonSample(task, candidates)
	
	// 6. 构建决策
	decision := Decision{
		Model:     selected.Model,
		Provider:  selected.Provider,
		Task:      task,
		RouteMode: "adaptive",
		Reason:    fmt.Sprintf("adaptive routing selected %s/%s (score: %.3f)", selected.Provider, selected.Model, selected.TotalScore),
		Scores:    ar.scoreAll(task, candidates),
	}
	
	if len(decision.Scores) > 1 {
		decision.FallbackModel = decision.Scores[1].Model
	}
	
	return decision
}

// thinpsonSample Thompson Sampling 选择模型
func (ar *AdaptiveRouter) thinpsonSample(task string, candidates []ModelProfile) CandidateScore {
	ar.mu.RLock()
	defer ar.mu.RUnlock()
	
	bestIdx := 0
	bestSample := -1.0
	
	for i, p := range candidates {
		key := task + ":" + p.ID
		dist := ar.scores[key]
		if dist == nil {
			// 未见过此组合，给予探索机会
			dist = &BetaDistribution{Alpha: 1, Beta: 1}
		}
		
		sample := dist.Sample()
		if sample > bestSample {
			bestSample = sample
			bestIdx = i
		}
	}
	
	c := candidates[bestIdx]
	return CandidateScore{
		Model:      c.ID,
		Provider:   c.Provider,
		Task:       c.Task,
		TotalScore: bestSample,
		Reason:     fmt.Sprintf("thompson sampling (task=%s)", task),
	}
}

// UpdateFeedback 更新路由反馈
func (ar *AdaptiveRouter) UpdateFeedback(metric *RequestMetric) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	
	key := metric.TaskType + ":" + metric.Model
	dist := ar.scores[key]
	if dist == nil {
		dist = &BetaDistribution{Alpha: 1, Beta: 1}
		ar.scores[key] = dist
	}
	
	// 计算奖励信号 (0-1)
	reward := ar.computeReward(metric)
	
	// Beta 后验更新
	dist.Alpha += reward * ar.alpha
	dist.Beta += (1 - reward) * ar.alpha
	
	// 记录指标
	ar.metrics.Add(metric)
}

// computeReward 计算奖励信号
func (ar *AdaptiveRouter) computeReward(m *RequestMetric) float64 {
	var reward float64
	
	// 错误惩罚
	if m.Error {
		return 0.0
	}
	
	// 延迟奖励 (归一化到 0-1, 假设 5000ms 为最差)
	latencyReward := 1.0 - math.Min(1.0, m.LatencyMs/5000.0)
	
	// 成本奖励 (归一化)
	costReward := 1.0 - math.Min(1.0, m.Cost/0.1) // 假设 $0.1 为高成本
	
	// 质量奖励
	qualityReward := m.QualityScore
	
	// 加权组合
	reward = latencyReward*0.3 + costReward*0.3 + qualityReward*0.4
	
	return math.Max(0, math.Min(1, reward))
}

// scoreAll 对所有候选评分
func (ar *AdaptiveRouter) scoreAll(task string, candidates []ModelProfile) []CandidateScore {
	scores := make([]CandidateScore, 0, len(candidates))
	for _, p := range candidates {
		key := task + ":" + p.ID
		ar.mu.RLock()
		dist := ar.scores[key]
		ar.mu.RUnlock()
		
		meanScore := 0.5 // 默认
		if dist != nil {
			meanScore = dist.Alpha / (dist.Alpha + dist.Beta)
		}
		
		scores = append(scores, CandidateScore{
			Model:      p.ID,
			Provider:   p.Provider,
			Task:       p.Task,
			TotalScore: meanScore,
			Reason:     fmt.Sprintf("adaptive mean score: %.3f", meanScore),
		})
	}
	
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalScore > scores[j].TotalScore
	})
	return scores
}

// getCandidates 获取候选模型
func (ar *AdaptiveRouter) getCandidates(req providers.ChatCompletionRequest) []ModelProfile {
	pool := filterCandidates(ar.base.registry, req.CandidateModels)
	if len(pool) == 0 {
		pool = ar.base.registry
	}
	return pool
}

// GetStats 获取路由统计
func (ar *AdaptiveRouter) GetStats() map[string]interface{} {
	ar.mu.RLock()
	defer ar.mu.RUnlock()
	
	stats := make(map[string]interface{})
	for key, dist := range ar.scores {
		mean := dist.Alpha / (dist.Alpha + dist.Beta)
		samples := dist.Alpha + dist.Beta - 2 // 减去先验
		stats[key] = map[string]interface{}{
			"mean":    mean,
			"samples": samples,
			"alpha":   dist.Alpha,
			"beta":    dist.Beta,
		}
	}
	return stats
}

// RollingMetrics 方法
func (rm *RollingMetrics) Add(m *RequestMetric) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	key := m.TaskType + ":" + m.Model
	rm.records[key] = append(rm.records[key], m)
	
	// 保持窗口大小
	if len(rm.records[key]) > rm.window {
		rm.records[key] = rm.records[key][len(rm.records[key])-rm.window:]
	}
}

// GetAverage 获取平均指标
func (rm *RollingMetrics) GetAverage(task, model string) *RequestMetric {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	key := task + ":" + model
	records := rm.records[key]
	if len(records) == 0 {
		return nil
	}
	
	var totalLatency, totalCost, totalQuality float64
	var totalTokens int
	var errors int
	
	for _, r := range records {
		totalLatency += r.LatencyMs
		totalCost += r.Cost
		totalTokens += r.Tokens
		totalQuality += r.QualityScore
		if r.Error {
			errors++
		}
	}
	
	n := float64(len(records))
	return &RequestMetric{
		LatencyMs:    totalLatency / n,
		Cost:         totalCost / n,
		Tokens:       int(float64(totalTokens) / n),
		QualityScore: totalQuality / n,
	}
}


