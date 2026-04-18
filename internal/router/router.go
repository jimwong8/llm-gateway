package router

import (
	"fmt"
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
	Channel    string  `json:"channel,omitempty"`
	Ability    string  `json:"ability,omitempty"`
	Task       string  `json:"task"`
	TotalScore float64 `json:"total_score"`
	Reason     string  `json:"reason"`
}

type Decision struct {
	Model         string           `json:"model"`
	Provider      string           `json:"provider"`
	Channel       string           `json:"channel,omitempty"`
	Ability       string           `json:"ability,omitempty"`
	Task          string           `json:"task"`
	RouteMode     string           `json:"route_mode"`
	Reason        string           `json:"reason"`
	Scores        []CandidateScore `json:"scores,omitempty"`
	FallbackModel string           `json:"fallback_model,omitempty"`
}

type Channel struct {
	ID       string  `json:"id"`
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Task     string  `json:"task"`
	Enabled  bool    `json:"enabled"`
	Priority int     `json:"priority"`
	Weight   float64 `json:"weight"`
}

type Ability struct {
	ID             string   `json:"id"`
	TenantID       string   `json:"tenant_id,omitempty"`
	RequestedModel string   `json:"requested_model,omitempty"`
	Task           string   `json:"task,omitempty"`
	ChannelIDs     []string `json:"channel_ids"`
	Enabled        bool     `json:"enabled"`
	Priority       int      `json:"priority"`
}

type Router struct {
	defaultModel string
	defaultProv  string
	registry     []ModelProfile
	channels     map[string]Channel
	abilities    map[string]Ability
	globalPolicy Policy
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

func (r *Router) initializeControlPlane() {
	if r.channels == nil {
		r.channels = map[string]Channel{}
	}
	if r.abilities == nil {
		r.abilities = map[string]Ability{}
	}
	for index, item := range r.registry {
		channelID := defaultChannelID(item)
		if _, ok := r.channels[channelID]; !ok {
			r.channels[channelID] = Channel{ID: channelID, Provider: item.Provider, Model: item.ID, Task: item.Task, Enabled: true, Priority: index + 1, Weight: 1}
		}
	}
}

func (r *Router) Models() []ModelProfile { return r.registry }

func (r *Router) SetChannels(channels []Channel) {
	r.initializeControlPlane()
	r.channels = map[string]Channel{}
	for _, channel := range channels {
		if strings.TrimSpace(channel.ID) == "" {
			continue
		}
		if channel.Weight == 0 {
			channel.Weight = 1
		}
		r.channels[strings.ToLower(strings.TrimSpace(channel.ID))] = channel
	}
}

func (r *Router) SetAbilities(abilities []Ability) {
	r.initializeControlPlane()
	r.abilities = map[string]Ability{}
	for _, ability := range abilities {
		if strings.TrimSpace(ability.ID) == "" {
			continue
		}
		r.abilities[strings.ToLower(strings.TrimSpace(ability.ID))] = ability
	}
}

// SetGlobalPolicy 允许在运行时动态注入全局路由策略（常用于动态配置重载）
func (r *Router) SetGlobalPolicy(p Policy) {
	r.globalPolicy = p
}

func (r *Router) Decide(req providers.ChatCompletionRequest) Decision {
	r.initializeControlPlane()
	task := classifyTask(req)
	mode := normalizedRouteMode(req.RouteMode)

	if mode == "manual" && strings.TrimSpace(req.PreferredModel) != "" {
		chosen := r.lookup(req.PreferredModel)
		return Decision{Model: chosen.ID, Provider: chosen.Provider, Task: task, RouteMode: mode, Reason: "preferred_model specified by caller", Scores: []CandidateScore{{Model: chosen.ID, Provider: chosen.Provider, Task: chosen.Task, TotalScore: 1.0, Reason: "manual override"}}}
	}

	// 全局策略优先
	if r.globalPolicy != nil {
		if score, err := r.globalPolicy.Execute(req, r); err == nil && score != nil {
			return Decision{
				Model:     score.Model,
				Provider:  score.Provider,
				Task:      task,
				RouteMode: "policy",
				Reason:    score.Reason,
				Scores:    []CandidateScore{*score},
			}
		}
	}

	if decision, ok := r.decideByControlPlane(req, task, mode); ok {
		return decision
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

func (r *Router) decideByControlPlane(req providers.ChatCompletionRequest, task, mode string) (Decision, bool) {
	if strings.TrimSpace(req.RouteChannel) != "" {
		channel, ok := r.lookupChannel(req.RouteChannel)
		if !ok || !channel.Enabled {
			return Decision{}, false
		}
		profile := r.lookup(channel.Model)
		profile.Provider = channel.Provider
		return Decision{
			Model:     profile.ID,
			Provider:  profile.Provider,
			Channel:   channel.ID,
			Task:      task,
			RouteMode: "channel",
			Reason:    "explicit route_channel requested",
			Scores:    []CandidateScore{{Model: profile.ID, Provider: profile.Provider, Channel: channel.ID, Task: profile.Task, TotalScore: 1.0, Reason: "explicit route_channel override"}},
		}, true
	}

	if len(req.RouteAbilities) == 0 {
		return Decision{}, false
	}

	type routeCandidate struct {
		ability Ability
		channel Channel
		profile ModelProfile
		score   float64
		reason  string
	}

	candidates := make([]routeCandidate, 0)
	for _, abilityID := range req.RouteAbilities {
		ability, ok := r.lookupAbility(abilityID)
		if !ok || !ability.Enabled {
			continue
		}
		if ability.Task != "" && !strings.EqualFold(ability.Task, task) {
			continue
		}
		if ability.RequestedModel != "" && strings.TrimSpace(req.Model) != "" && !strings.EqualFold(ability.RequestedModel, req.Model) {
			continue
		}
		for _, channelID := range ability.ChannelIDs {
			channel, ok := r.lookupChannel(channelID)
			if !ok || !channel.Enabled {
				continue
			}
			profile := r.lookup(channel.Model)
			profile.Provider = channel.Provider
			baseScore := scoreCandidates([]ModelProfile{profile}, task)[0]
			priorityBoost := 0.0
			if ability.Priority > 0 {
				priorityBoost += 1.0 / float64(ability.Priority)
			}
			if channel.Priority > 0 {
				priorityBoost += 1.0 / float64(channel.Priority)
			}
			weightedScore := baseScore.TotalScore + priorityBoost + channel.Weight*0.01
			candidates = append(candidates, routeCandidate{
				ability: ability,
				channel: channel,
				profile: profile,
				score:   weightedScore,
				reason:  fmt.Sprintf("ability %s matched channel %s", ability.ID, channel.ID),
			})
		}
	}

	if len(candidates) == 0 {
		return Decision{}, false
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	scores := make([]CandidateScore, 0, len(candidates))
	for _, candidate := range candidates {
		scores = append(scores, CandidateScore{
			Model:      candidate.profile.ID,
			Provider:   candidate.profile.Provider,
			Channel:    candidate.channel.ID,
			Ability:    candidate.ability.ID,
			Task:       candidate.profile.Task,
			TotalScore: candidate.score,
			Reason:     candidate.reason,
		})
	}
	decision := Decision{
		Model:     candidates[0].profile.ID,
		Provider:  candidates[0].profile.Provider,
		Channel:   candidates[0].channel.ID,
		Ability:   candidates[0].ability.ID,
		Task:      task,
		RouteMode: "ability",
		Reason:    candidates[0].reason,
		Scores:    scores,
	}
	if len(candidates) > 1 {
		decision.FallbackModel = candidates[1].profile.ID
	}
	return decision, true
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

func (r *Router) lookupChannel(channelID string) (Channel, bool) {
	r.initializeControlPlane()
	channel, ok := r.channels[strings.ToLower(strings.TrimSpace(channelID))]
	return channel, ok
}

func (r *Router) lookupAbility(abilityID string) (Ability, bool) {
	r.initializeControlPlane()
	ability, ok := r.abilities[strings.ToLower(strings.TrimSpace(abilityID))]
	return ability, ok
}

func defaultChannelID(item ModelProfile) string {
	provider := strings.ToLower(strings.TrimSpace(item.Provider))
	model := strings.ToLower(strings.TrimSpace(item.ID))
	provider = strings.ReplaceAll(provider, " ", "-")
	model = strings.ReplaceAll(model, " ", "-")
	return provider + ":" + model
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
