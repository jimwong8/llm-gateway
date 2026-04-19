package governance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrActivePolicyNotFound  = errors.New("active policy not found")
	ErrNoMatchingPolicyScope = errors.New("no matching policy scope")
	ErrNoResolvedModel       = errors.New("no resolved model")
)

// ResolveInput 表示一次 runtime 解析请求。
type ResolveInput struct {
	RequestID           string
	TenantID            string
	Environment         string
	AgentID             string
	TaskType            string
	SystemFallbackModel string
	RolloutID           string
}

// ResolveDecision 表示解析结果与命中的作用域信息。
type ResolveDecision struct {
	RequestID          string
	PolicyVersionID    string
	RolloutID          string
	Environment        string
	TenantID           string
	AgentID            string
	TaskType           string
	ResolvedModel      string
	FallbackChain      []string
	MatchedScopeType   string
	MatchedScope       map[string]string
	PolicyFallbackUsed bool
	SystemFallbackUsed bool
}

// RuntimeResolver 执行 runtime 策略解析并落库决策快照。
type RuntimeResolver struct {
	store     *Store
	loader    activePolicyLoader
	snapshots runtimeDecisionSnapshotWriter
	cache     *sync.Map
}

type activePolicyLoader interface {
	LoadActivePolicy(ctx context.Context, environment string) (PolicyVersion, error)
}

type runtimeDecisionSnapshotWriter interface {
	Save(ctx context.Context, snapshot RuntimeDecisionSnapshotWrite) error
}

type activePolicyRepo struct {
	db *sql.DB
}

// NewRuntimeResolver 构建默认 runtime 解析器。
func NewRuntimeResolver(store *Store) *RuntimeResolver {
	if store == nil {
		return &RuntimeResolver{cache: &sync.Map{}}
	}
	return &RuntimeResolver{
		store:     store,
		loader:    &activePolicyRepo{db: store.DB()},
		snapshots: NewSnapshotRepo(store),
		cache:     &sync.Map{},
	}
}

func (r *RuntimeResolver) Store() *Store {
	if r == nil {
		return nil
	}
	return r.store
}

func (r *RuntimeResolver) InvalidateCache(environment string) {
	if r == nil || r.cache == nil {
		return
	}
	if environment == "" {
		r.cache.Range(func(key, value any) bool {
			r.cache.Delete(key)
			return true
		})
		return
	}
	r.cache.Delete(environment)
}

func (r *RuntimeResolver) loadPolicyCached(ctx context.Context, environment string) (PolicyVersion, error) {
	if r.cache != nil {
		if val, ok := r.cache.Load(environment); ok {
			return val.(PolicyVersion), nil
		}
	}
	policy, err := r.loader.LoadActivePolicy(ctx, environment)
	if err != nil {
		return PolicyVersion{}, err
	}
	if r.cache != nil {
		r.cache.Store(environment, policy)
	}
	return policy, nil
}

func (r *RuntimeResolver) Resolve(ctx context.Context, input ResolveInput) (ResolveDecision, error) {
	if r == nil || r.loader == nil || r.snapshots == nil {
		return ResolveDecision{}, errors.New("runtime resolver is not initialized")
	}

	input.RequestID = strings.TrimSpace(input.RequestID)
	input.Environment = strings.TrimSpace(input.Environment)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.TaskType = strings.TrimSpace(input.TaskType)
	input.SystemFallbackModel = strings.TrimSpace(input.SystemFallbackModel)
	input.RolloutID = strings.TrimSpace(input.RolloutID)

	if input.RequestID == "" {
		return ResolveDecision{}, fmt.Errorf("request_id is required")
	}
	if input.Environment == "" {
		return ResolveDecision{}, fmt.Errorf("environment is required")
	}
	if input.AgentID == "" {
		return ResolveDecision{}, fmt.Errorf("agent_id is required")
	}

	active, err := r.loadPolicyCached(ctx, input.Environment)
	if err != nil {
		return ResolveDecision{}, err
	}

	matches := buildScopeCandidates(active.Policy, input)
	chosen, ok := chooseHighestPriorityScope(matches)
	if !ok {
		if input.SystemFallbackModel == "" {
			return ResolveDecision{}, ErrNoMatchingPolicyScope
		}
		chosen = runtimeScopeCandidate{
			scopeType: "system_fallback",
			scope: map[string]string{
				"environment": input.Environment,
				"agent_id":    input.AgentID,
				"reason":      "no_policy_scope_match",
			},
			primaryModel: input.SystemFallbackModel,
		}
	}

	resolvedModel, remainingFallbacks, policyFallbackUsed, err := resolveModelFromCandidate(chosen)
	if err != nil {
		return ResolveDecision{}, err
	}

	systemFallbackUsed := chosen.scopeType == "system_fallback"
	decision := ResolveDecision{
		RequestID:          input.RequestID,
		PolicyVersionID:    active.ID,
		RolloutID:          input.RolloutID,
		Environment:        input.Environment,
		TenantID:           input.TenantID,
		AgentID:            input.AgentID,
		TaskType:           input.TaskType,
		ResolvedModel:      resolvedModel,
		FallbackChain:      remainingFallbacks,
		MatchedScopeType:   chosen.scopeType,
		MatchedScope:       chosen.scope,
		PolicyFallbackUsed: policyFallbackUsed,
		SystemFallbackUsed: systemFallbackUsed,
	}

	err = r.snapshots.Save(ctx, RuntimeDecisionSnapshotWrite{
		RequestID:          decision.RequestID,
		PolicyVersionID:    decision.PolicyVersionID,
		RolloutID:          decision.RolloutID,
		Environment:        decision.Environment,
		TenantID:           decision.TenantID,
		AgentID:            decision.AgentID,
		TaskType:           decision.TaskType,
		MatchedScopeType:   decision.MatchedScopeType,
		MatchedScope:       decision.MatchedScope,
		ResolvedModel:      decision.ResolvedModel,
		FallbackChain:      decision.FallbackChain,
		PolicyFallbackUsed: decision.PolicyFallbackUsed,
		SystemFallbackUsed: decision.SystemFallbackUsed,
		Success:            true,
	})
	if err != nil {
		return ResolveDecision{}, err
	}

	return decision, nil
}

func (r *activePolicyRepo) LoadActivePolicy(ctx context.Context, environment string) (PolicyVersion, error) {
	environment = strings.TrimSpace(environment)
	if environment == "" {
		return PolicyVersion{}, fmt.Errorf("environment is required")
	}

	var (
		versionID string
		status    string
		policyRaw []byte
	)
	err := r.db.QueryRowContext(ctx, `
SELECT policy_version_id, status, policy_json
FROM model_policy_versions
WHERE environment = $1 AND status = 'active'
ORDER BY COALESCE(activated_at, created_at) DESC, id DESC
LIMIT 1
`, environment).Scan(&versionID, &status, &policyRaw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PolicyVersion{}, ErrActivePolicyNotFound
		}
		return PolicyVersion{}, err
	}

	var policy RuntimePolicy
	if len(policyRaw) > 0 {
		if err := json.Unmarshal(policyRaw, &policy); err != nil {
			return PolicyVersion{}, fmt.Errorf("decode active policy json: %w", err)
		}
	}

	return PolicyVersion{
		ID:          strings.TrimSpace(versionID),
		Environment: environment,
		Status:      PolicyVersionStatus(status),
		Policy:      policy,
	}, nil
}

type runtimeScopeCandidate struct {
	scopeType    string
	scope        map[string]string
	primaryModel string
	fallbacks    []string
}

func buildScopeCandidates(policy RuntimePolicy, input ResolveInput) []runtimeScopeCandidate {
	matches := make([]runtimeScopeCandidate, 0, 2)

	if agentPolicy, ok := policy.Agents[input.AgentID]; ok {
		matches = append(matches, runtimeScopeCandidate{
			scopeType:    "agent",
			scope:        map[string]string{"environment": input.Environment, "agent_id": input.AgentID},
			primaryModel: strings.TrimSpace(agentPolicy.PrimaryModel),
			fallbacks:    normalizeModelChain(agentPolicy.FallbackChain),
		})
	}

	if defaultModel := strings.TrimSpace(policy.DefaultModel); defaultModel != "" {
		matches = append(matches, runtimeScopeCandidate{
			scopeType:    "environment",
			scope:        map[string]string{"environment": input.Environment},
			primaryModel: defaultModel,
		})
	}

	return matches
}

func chooseHighestPriorityScope(matches []runtimeScopeCandidate) (runtimeScopeCandidate, bool) {
	if len(matches) == 0 {
		return runtimeScopeCandidate{}, false
	}
	order := ScopePriorityOrder()
	rank := make(map[string]int, len(order))
	for idx, scopeType := range order {
		rank[scopeType] = idx
	}

	chosen := matches[0]
	chosenRank := scopeRank(rank, chosen.scopeType, len(order))
	for i := 1; i < len(matches); i++ {
		candidate := matches[i]
		candidateRank := scopeRank(rank, candidate.scopeType, len(order))
		if candidateRank < chosenRank {
			chosen = candidate
			chosenRank = candidateRank
		}
	}
	return chosen, true
}

func scopeRank(rank map[string]int, scopeType string, unknownBase int) int {
	if value, ok := rank[scopeType]; ok {
		return value
	}
	return unknownBase + 1
}

func resolveModelFromCandidate(candidate runtimeScopeCandidate) (string, []string, bool, error) {
	primary := strings.TrimSpace(candidate.primaryModel)
	fallbacks := normalizeModelChain(candidate.fallbacks)

	if primary != "" {
		filteredFallbacks := make([]string, 0, len(fallbacks))
		for _, model := range fallbacks {
			if model != primary {
				filteredFallbacks = append(filteredFallbacks, model)
			}
		}
		return primary, filteredFallbacks, false, nil
	}
	if len(fallbacks) > 0 {
		return fallbacks[0], append([]string(nil), fallbacks[1:]...), true, nil
	}
	return "", nil, false, ErrNoResolvedModel
}

func normalizeModelChain(chain []string) []string {
	out := make([]string, 0, len(chain))
	for _, model := range chain {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
