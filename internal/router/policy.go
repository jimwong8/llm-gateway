package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"llm-gateway/gateway/internal/providers"
)

// Policy 定义了策略执行的公共接口
type Policy interface {
	Execute(req providers.ChatCompletionRequest, r *Router) (*CandidateScore, error)
}

// PolicyConfig 定义统一配置模型
type PolicyConfig struct {
	Type     string             `json:"type"`               // "direct", "load_balance", "fallback"
	Model    string             `json:"model,omitempty"`    // for direct
	Provider string             `json:"provider,omitempty"` // for direct
	Weights  map[string]float64 `json:"weights,omitempty"`  // for load_balance
	Targets  []json.RawMessage  `json:"targets,omitempty"`  // for fallback
}

// =======================
// 1. Direct Policy
// =======================
type DirectPolicy struct {
	Model    string
	Provider string
}

func (p *DirectPolicy) Execute(req providers.ChatCompletionRequest, r *Router) (*CandidateScore, error) {
	// 先看看这个模型在 registry 里有没有
	modelProfile := r.lookup(p.Model)
	if p.Provider != "" {
		modelProfile.Provider = p.Provider
	}
	return &CandidateScore{
		Model:      modelProfile.ID,
		Provider:   modelProfile.Provider,
		Task:       modelProfile.Task,
		TotalScore: 1.0,
		Reason:     "direct policy matched",
	}, nil
}

// =======================
// 2. LoadBalance Policy
// =======================
type lbTarget struct {
	key    string
	weight float64
}

type LoadBalancePolicy struct {
	Targets []lbTarget
}

func (p *LoadBalancePolicy) Execute(req providers.ChatCompletionRequest, r *Router) (*CandidateScore, error) {
	if len(p.Targets) == 0 {
		return nil, errors.New("lb policy has no targets")
	}
	
	totalWeight := 0.0
	for _, t := range p.Targets {
		totalWeight += t.weight
	}
	
	if totalWeight <= 0 {
		return nil, errors.New("lb policy has total weight 0")
	}

	val := rand.Float64() * totalWeight
	var chosenKey string
	var accum float64
	for _, t := range p.Targets {
		accum += t.weight
		if val <= accum {
			chosenKey = t.key
			break
		}
	}
	
	// 若没选中(精度问题兜底)，取最后一个
	if chosenKey == "" {
		chosenKey = p.Targets[len(p.Targets)-1].key
	}
	
	// 解析 key (可以是 "provider" 或 "provider:model" 这种格式，简化处理我们这里直接假设是 model，并且去查 profile)
	// 在生产中可以支持更复杂的 key，这里使用简化的 model
	modelProfile := r.lookup(chosenKey)
	return &CandidateScore{
		Model:      modelProfile.ID,
		Provider:   modelProfile.Provider,
		Task:       modelProfile.Task,
		TotalScore: 1.0,
		Reason:     fmt.Sprintf("lb policy matched weight for %s", chosenKey),
	}, nil
}

// =======================
// 3. Fallback Policy
// =======================
type FallbackPolicy struct {
	Targets []Policy
}

func (p *FallbackPolicy) Execute(req providers.ChatCompletionRequest, r *Router) (*CandidateScore, error) {
	if len(p.Targets) == 0 {
		return nil, errors.New("fallback policy has no targets")
	}

	var lastErr error
	for _, target := range p.Targets {
		if target == nil {
			continue
		}
		if score, err := target.Execute(req, r); err == nil && score != nil {
			score.Reason = "fallback selected: " + score.Reason
			return score, nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("all fallback targets were nil or empty")
	}
	return nil, fmt.Errorf("all fallback targets failed, last error: %w", lastErr)
}

// =======================
// 工厂函数: 解析 JSON 配置
// =======================
func ParsePolicyConfig(raw json.RawMessage) (Policy, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var cfg PolicyConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal policy config: %w", err)
	}

	switch strings.ToLower(cfg.Type) {
	case "direct":
		if cfg.Model == "" {
			return nil, errors.New("direct policy requires model")
		}
		return &DirectPolicy{
			Model:    cfg.Model,
			Provider: cfg.Provider,
		}, nil

	case "load_balance", "lb":
		if len(cfg.Weights) == 0 {
			return nil, errors.New("load_balance policy requires weights")
		}
		var targets []lbTarget
		for k, w := range cfg.Weights {
			if w > 0 {
				targets = append(targets, lbTarget{key: k, weight: w})
			}
		}
		if len(targets) == 0 {
			return nil, errors.New("load_balance policy requires at least one positive weight")
		}
		return &LoadBalancePolicy{Targets: targets}, nil

	case "fallback":
		if len(cfg.Targets) == 0 {
			return nil, errors.New("fallback policy requires targets")
		}
		var subPolicies []Policy
		for _, rawTarget := range cfg.Targets {
			sub, err := ParsePolicyConfig(rawTarget)
			if err != nil {
				return nil, fmt.Errorf("parse fallback target: %w", err)
			}
			if sub != nil {
				subPolicies = append(subPolicies, sub)
			}
		}
		if len(subPolicies) == 0 {
			return nil, errors.New("fallback policy requires valid sub targets")
		}
		return &FallbackPolicy{Targets: subPolicies}, nil

	default:
		return nil, fmt.Errorf("unknown policy type: %s", cfg.Type)
	}
}
