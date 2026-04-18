# 2026-03-24 路由策略引擎 (Routing Engine) 设计

## 1. 背景

当前在 `/internal/router/router.go` 中，我们有一个基础的 `Router` 实现。现有的 `Decide` 方法主要是基于模型硬编码的一个分数打分（`Capability`、`CostScore` 等）。

为了更好地满足生产环境中动态调度与高可用的需求，我们需要引入一层“路由策略引擎”。这允许我们以配置驱动的方式，实现诸如：
- **负载均衡 (Load Balancing)**：按照给定的权重比例将请求分发给不同模型或 provider。
- **主备降级 (Fallback)**：优先尝试一个链路，失败或不可用时无缝切换到备用链路。
- **条件路由 (Conditional Routing)**：基于租户或请求的特征（如 `preferred_model`）做更精确匹配。

## 2. 目标

- 引入可序列化/反序列化（例如从 JSON 配置）的 `Policy` 接口。
- 实现至少 3 种基础策略（Direct、LoadBalance、Fallback）。
- 保证扩展性：未来可以非常方便地加入基于 Tag 等其他属性的新策略。
- 重构现有的 `Router.Decide` 方法：如果配置了对应的策略则按策略执行，否则降级回现有的分数模型。

## 3. 设计方案

### 3.1 接口与配置模型

定义 `Policy` 接口和解析用结构体：

```go
package router

import (
	"encoding/json"
	"errors"
	"math/rand"
	"llm-gateway/gateway/internal/providers"
)

type Policy interface {
	// Execute 运行策略。返回匹配的 CandidateScore
	Execute(req providers.ChatCompletionRequest, r *Router) (*CandidateScore, error)
}

// 通用的配置承载体
type PolicyConfig struct {
	Type     string                  `json:"type"`               // "direct", "load_balance", "fallback"
	Model    string                  `json:"model,omitempty"`    // for direct
	Provider string                  `json:"provider,omitempty"` // for direct
	Weights  map[string]float64      `json:"weights,omitempty"`  // for load_balance (model/provider -> weight)
	Targets  []json.RawMessage       `json:"targets,omitempty"`  // for fallback
}
```

### 3.2 基础策略实现

#### 1. DirectPolicy (直连策略)
直接指定路由到哪个模型（及可选的 Provider）。

#### 2. LoadBalancePolicy (权重负载均衡策略)
持有多个子目标的权重分布。每次 `Execute` 内部基于加权随机选取其中一个目标。

#### 3. FallbackPolicy (降级/主备策略)
持有多个子策略数组。顺序执行，返回第一个成功的；如果失败则自动执行下一个，以此作为降级容灾的基础保障。

### 3.3 Router 整合

在 `Router` 中：
```go
type Router struct {
	defaultModel string
	defaultProv  string
	registry     []ModelProfile
    // 新增：如果配置了 Policy 则直接用 Policy
	globalPolicy Policy 
}
```

改造 `Decide` 方法：
```go
func (r *Router) Decide(req providers.ChatCompletionRequest) Decision {
    // 1. 如果有全局配置的策略引擎，走策略解析
    if r.globalPolicy != nil {
        if score, err := r.globalPolicy.Execute(req, r); err == nil && score != nil {
            return Decision{
                Model:     score.Model,
                Provider:  score.Provider,
                Task:      classifyTask(req),
                RouteMode: "policy",
                Reason:    score.Reason,
            }
        }
    }
    
    // 2. 原有基于打分的降级逻辑
    // ...
}
```

## 4. 落地与实施计划

1. 在 `internal/router/policy.go` 中完成上述结构及基础反序列化（基于 Factory 模式解析 `PolicyConfig`）。
2. 在 `internal/router/router.go` 补充相关整合字段。
3. 编写一份基于本地 `main` 或测试单元的简易验收脚本。

本规划保证了后续通过配置中心注入 JSON 文件即可随时热重载网关的全局路由规则。