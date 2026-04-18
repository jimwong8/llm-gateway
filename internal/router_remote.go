package router

import (
	"sort"
	"strings"

	"llm-gateway/gateway/internal/providers"
)

type ModelProfile struct {
	ID           string  `json:"id"`
	Provider     string  `json:"provider"`
	Task         string  `json:"task"`
	Description  string  `json:"description"`
	Capability   float64 `json:"capability"`
	CostScore    float64 `json:"cost_score"`
	LatencyScore float64 `json:"latency_score"`
	HealthScore  float64 `json:"health_score"`
}

type CandidateScore struct {
	Model      string  `json:"model"`
	Provider   string  `json:"provider"`
	Task       string  `json:"task"`
	TotalScore float64 `json:"total_score"`
	Reason     string  `json:"reason"`
}

type Decision struct {
	Model         string           `json:"model"`
	Provider      string           `json:"provider"`
	Task          string           `json:"task"`
	RouteMode     string           `json:"route_mode"`
	Reason        string           `json:"reason"`
	Scores        []CandidateScore `json:"scores,omitempty"`
	FallbackModel string           `json:"fallback_model,omitempty"`
}

type Router struct {
	defaultModel string
	defaultProv  string
	registry     []ModelProfile
}

func New(defaultProvider, defaultModel string) *Router {
	return &Router{
		defaultModel: defaultModel,
		defaultProv:  defaultProvider,
		registry: []ModelProfile{
			{ID: defaultModel, Provider: defaultProvider, Task: "general", Description: "default general model", Capability: 0.82, CostScore: 0.90, LatencyScore: 0.92, HealthScore: 0.95},
			{ID: "gpt-4o-mini", Provider: "openai", Task: "general", Description: "fast general chat model", Capability: 0.82, CostScore: 0.90, LatencyScore: 0.92, HealthScore: 0.95},
			{ID: "deepseek-coder", Provider: "mock-code", Task: "code", Description: "coding-focused route target", Capability: 0.97, CostScore: 0.88, LatencyScore: 0.82, HealthScore: 0.90},
			{ID: "claude-sonnet", Provider: "mock-analysis", Task: "analysis", Description: "analysis-focused route target", Capability: 0.95, CostScore: 0.70, LatencyScore: 0.78, HealthScore: 0.94},
			{ID: "fail-code", Provider: "mock-fail", Task: "code", Description: "failure injection model for fallback testing", Capability: 0.99, CostScore: 0.95, LatencyScore: 0.95, HealthScore: 0.10},
		},
	}
}

func (r *Router) Models() []ModelProfile { return r.registry }

func (r *Router) Decide(req providers.ChatCompletionRequest) Decision {
	task := classifyTask(req)
	mode := normalizedRouteMode(req.RouteMode)

	if mode == "manual" && strings.TrimSpace(req.PreferredModel) != "" {
		chosen := r.lookup(req.PreferredModel)
		return Decision{Model: chosen.ID, Provider: chosen.Provider, Task: task, RouteMode: mode, Reason: "preferred_model specified by caller", Scores: []CandidateScore{{Model: chosen.ID, Provider: chosen.Provider, Task: chosen.Task, TotalScore: 1.0, Reason: "manual override"}}}
	}

	pool := filterCandidates(r.registry, req.CandidateModels)
	if len(pool) == 0 {
		pool = r.registry
	}

	if strings.TrimSpace(req.PreferredModel) != "" {
		chosen := r.lookup(req.PreferredModel)
		return Decision{Model: chosen.ID, Provider: chosen.Provider, Task: task, RouteMode: "hybrid", Reason: "preferred_model overrides automatic selection", Scores: []CandidateScore{{Model: chosen.ID, Provider: chosen.Provider, Task: chosen.Task, TotalScore: 1.0, Reason: "preferred_model override"}}}
	}

	scores := scoreCandidates(pool, task)
	if len(scores) == 0 {
		fallback := r.lookup(r.defaultModel)
		return Decision{Model: fallback.ID, Provider: fallback.Provider, Task: task, RouteMode: mode, Reason: "fallback to default model"}
	}

	best := scores[0]
	decision := Decision{Model: best.Model, Provider: best.Provider, Task: task, RouteMode: mode, Reason: best.Reason, Scores: scores}
	if len(scores) > 1 {
		decision.FallbackModel = scores[1].Model
	}
	return decision
}

func classifyTask(req providers.ChatCompletionRequest) string {
	if hint := strings.TrimSpace(strings.ToLower(req.TaskHint)); hint != "" {
		return hint
	}
	combined := make([]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		combined = append(combined, strings.ToLower(msg.Content))
	}
	text := strings.Join(combined, " ")
	switch {
	case strings.Contains(text, "code"), strings.Contains(text, "golang"), strings.Contains(text, "python"), strings.Contains(text, "hello world"), strings.Contains(text, "软件"), strings.Contains(text, "开发"), strings.Contains(text, "代码"):
		return "code"
	case strings.Contains(text, "analyze"), strings.Contains(text, "分析"), strings.Contains(text, "report"), strings.Contains(text, "总结"):
		return "analysis"
	default:
		return "general"
	}
}

func (r *Router) lookup(model string) ModelProfile {
	for _, item := range r.registry {
		if strings.EqualFold(item.ID, model) {
			return item
		}
	}
	return ModelProfile{ID: model, Provider: r.defaultProv, Task: "general", Description: "externally supplied model", Capability: 0.8, CostScore: 0.8, LatencyScore: 0.8, HealthScore: 0.8}
}

func filterCandidates(registry []ModelProfile, candidates []string) []ModelProfile {
	if len(candidates) == 0 {
		return nil
	}
	var out []ModelProfile
	for _, candidate := range candidates {
		for _, item := range registry {
			if strings.EqualFold(item.ID, candidate) {
				out = append(out, item)
			}
		}
	}
	return out
}

func normalizedRouteMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "manual" || mode == "hybrid" {
		return mode
	}
	return "auto"
}

func scoreCandidates(models []ModelProfile, targetTask string) []CandidateScore {
	scores := make([]CandidateScore, 0, len(models))
	for _, item := range models {
		taskBoost := 0.72
		reason := "general weighted routing"
		if item.Task == targetTask {
			taskBoost = 1.0
			reason = "task-based routing matched model capability"
		}
		total := item.Capability*0.45*taskBoost + item.CostScore*0.20 + item.LatencyScore*0.15 + item.HealthScore*0.20
		scores = append(scores, CandidateScore{Model: item.ID, Provider: item.Provider, Task: item.Task, TotalScore: total, Reason: reason})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].TotalScore > scores[j].TotalScore })
	return scores
}
