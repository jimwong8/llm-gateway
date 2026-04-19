package governance

import "time"

// RecommendationStatus 表示推荐记录生命周期状态。
type RecommendationStatus string

const (
	RecommendationStatusDraft    RecommendationStatus = "draft"
	RecommendationStatusReady    RecommendationStatus = "ready"
	RecommendationStatusArchived RecommendationStatus = "archived"
	RecommendationStatusPending  RecommendationStatus = "pending"
	RecommendationStatusApproved RecommendationStatus = "approved"
	RecommendationStatusRejected RecommendationStatus = "rejected"
)

func (s RecommendationStatus) Valid() bool {
	switch s {
	case RecommendationStatusDraft,
		RecommendationStatusReady,
		RecommendationStatusArchived,
		RecommendationStatusPending,
		RecommendationStatusApproved,
		RecommendationStatusRejected:
		return true
	default:
		return false
	}
}

// ApprovalStatus 表示人工审批状态。
type ApprovalStatus string

const (
	ApprovalStatusPending    ApprovalStatus = "pending"
	ApprovalStatusApproved   ApprovalStatus = "approved"
	ApprovalStatusRejected   ApprovalStatus = "rejected"
	ApprovalStatusOverridden ApprovalStatus = "overridden"
)

func (s ApprovalStatus) Valid() bool {
	switch s {
	case ApprovalStatusPending, ApprovalStatusApproved, ApprovalStatusRejected, ApprovalStatusOverridden:
		return true
	default:
		return false
	}
}

// PolicyVersionStatus 表示策略版本状态。
type PolicyVersionStatus string

const (
	PolicyVersionDraft      PolicyVersionStatus = "draft"
	PolicyVersionApproved   PolicyVersionStatus = "approved"
	PolicyVersionActive     PolicyVersionStatus = "active"
	PolicyVersionSuperseded PolicyVersionStatus = "superseded"
	PolicyVersionRolledBack PolicyVersionStatus = "rolled_back"
)

func (s PolicyVersionStatus) Valid() bool {
	switch s {
	case PolicyVersionDraft, PolicyVersionApproved, PolicyVersionActive, PolicyVersionSuperseded, PolicyVersionRolledBack:
		return true
	default:
		return false
	}
}

// RolloutStatus 表示发布推进状态。
type RolloutStatus string

const (
	RolloutStatusPlanned    RolloutStatus = "planned"
	RolloutStatusRunning    RolloutStatus = "running"
	RolloutStatusPromoted   RolloutStatus = "promoted"
	RolloutStatusFinalized  RolloutStatus = "finalized"
	RolloutStatusHalted     RolloutStatus = "halted"
	RolloutStatusRolledBack RolloutStatus = "rolled_back"
)

func (s RolloutStatus) Valid() bool {
	switch s {
	case RolloutStatusPlanned, RolloutStatusRunning, RolloutStatusPromoted, RolloutStatusFinalized, RolloutStatusHalted, RolloutStatusRolledBack:
		return true
	default:
		return false
	}
}

func (s RolloutStatus) IsTerminal() bool {
	switch s {
	case RolloutStatusFinalized, RolloutStatusHalted, RolloutStatusRolledBack:
		return true
	default:
		return false
	}
}

// EvaluationRunStatus 表示评估运行状态。
type EvaluationRunStatus string

const (
	EvaluationRunStatusPending   EvaluationRunStatus = "pending"
	EvaluationRunStatusQueued    EvaluationRunStatus = "queued"
	EvaluationRunStatusRunning   EvaluationRunStatus = "running"
	EvaluationRunStatusSucceeded EvaluationRunStatus = "succeeded"
	EvaluationRunStatusFailed    EvaluationRunStatus = "failed"
	EvaluationRunStatusCanceled  EvaluationRunStatus = "canceled"
)

func (s EvaluationRunStatus) Valid() bool {
	switch s {
	case EvaluationRunStatusPending, EvaluationRunStatusQueued, EvaluationRunStatusRunning, EvaluationRunStatusSucceeded, EvaluationRunStatusFailed, EvaluationRunStatusCanceled:
		return true
	default:
		return false
	}
}

func (s EvaluationRunStatus) IsTerminal() bool {
	switch s {
	case EvaluationRunStatusSucceeded, EvaluationRunStatusFailed, EvaluationRunStatusCanceled:
		return true
	default:
		return false
	}
}

// PolicyDriftStatus 表示策略漂移状态。
type PolicyDriftStatus string

const (
	PolicyDriftStatusDetected PolicyDriftStatus = "detected"
	PolicyDriftStatusAccepted PolicyDriftStatus = "accepted"
	PolicyDriftStatusResolved PolicyDriftStatus = "resolved"
)

func (s PolicyDriftStatus) Valid() bool {
	switch s {
	case PolicyDriftStatusDetected, PolicyDriftStatusAccepted, PolicyDriftStatusResolved:
		return true
	default:
		return false
	}
}

// Recommendation 是候选模型推荐结果。
type Recommendation struct {
	ID               string               `json:"id"`
	TenantID         string               `json:"tenant_id"`
	Environment      string               `json:"environment"`
	AgentID          string               `json:"agent_id"`
	TaskType         string               `json:"task_type,omitempty"`
	Status           RecommendationStatus `json:"status"`
	RecommendedModel string               `json:"recommended_model,omitempty"`
	Candidates       []CandidateModel     `json:"candidates,omitempty"`
	ScoreBreakdown   ScoreBreakdown       `json:"score_breakdown,omitempty"`
	ApprovalRequired bool                 `json:"approval_required"`
	SourceRunID      string               `json:"source_run_id,omitempty"`
	Summary          string               `json:"summary,omitempty"`
	GeneratedAt      time.Time            `json:"generated_at"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at,omitempty"`
}

// CandidateModel 表示单个模型候选。
type CandidateModel struct {
	ModelID   string         `json:"model_id"`
	Provider  string         `json:"provider,omitempty"`
	Rank      int            `json:"rank,omitempty"`
	Composite float64        `json:"composite,omitempty"`
	Breakdown ScoreBreakdown `json:"breakdown,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}

// ScoreBreakdown 表示推荐评分拆解。
type ScoreBreakdown struct {
	Quality      float64 `json:"quality,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
	Latency      float64 `json:"latency,omitempty"`
	Safety       float64 `json:"safety,omitempty"`
	Availability float64 `json:"availability,omitempty"`
}

// EffectiveScope 定义审批/策略生效范围。
type EffectiveScope struct {
	Scope       string `json:"scope"`
	ProjectID   string `json:"project_id,omitempty"`
	Environment string `json:"environment"`
}

// Approval 表示审批或人工覆盖动作。
type Approval struct {
	ID               string         `json:"id"`
	RecommendationID string         `json:"recommendation_id,omitempty"`
	PolicyVersionID  string         `json:"policy_version_id,omitempty"`
	Status           ApprovalStatus `json:"status"`
	FinalModel       string         `json:"final_model,omitempty"`
	Reason           string         `json:"reason,omitempty"`
	Actor            string         `json:"actor"`
	Scope            EffectiveScope `json:"scope"`
	ApprovedAt       time.Time      `json:"approved_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

// PolicyVersion 表示治理域中的不可变策略版本。
type PolicyVersion struct {
	ID          string              `json:"id"`
	TenantID    string              `json:"tenant_id"`
	Environment string              `json:"environment"`
	Version     int64               `json:"version"`
	Status      PolicyVersionStatus `json:"status"`
	Policy      RuntimePolicy       `json:"policy"`
	Summary     string              `json:"summary,omitempty"`
	CreatedBy   string              `json:"created_by,omitempty"`
	ApprovedBy  string              `json:"approved_by,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	ApprovedAt  time.Time           `json:"approved_at,omitempty"`
	ActivatedAt time.Time           `json:"activated_at,omitempty"`
}

// RuntimePolicy 是 runtime 解析用的稳定策略快照。
type RuntimePolicy struct {
	Version      int64                  `json:"version"`
	Environment  string                 `json:"environment"`
	DefaultModel string                 `json:"default_model,omitempty"`
	Agents       map[string]AgentPolicy `json:"agents,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
}

// AgentPolicy 定义 agent 维度的模型路由规则。
type AgentPolicy struct {
	PrimaryModel  string            `json:"primary_model"`
	FallbackChain []string          `json:"fallback_chain,omitempty"`
	AllowedModels []string          `json:"allowed_models,omitempty"`
	Parameters    map[string]string `json:"parameters,omitempty"`
}

// Rollout 是一次策略版本发布推进过程。
type Rollout struct {
	ID                string                 `json:"id"`
	PolicyVersionID   string                 `json:"policy_version_id"`
	Status            RolloutStatus          `json:"status"`
	TargetEnvironment string                 `json:"target_environment"`
	RolloutMode       string                 `json:"rollout_mode,omitempty"`
	RolloutPercent    int                    `json:"rollout_percent,omitempty"`
	TriggeredBy       string                 `json:"triggered_by,omitempty"`
	TriggerReason     string                 `json:"trigger_reason,omitempty"`
	GuardSummary      string                 `json:"guard_summary,omitempty"`
	Metrics           RolloutMetricsSnapshot `json:"metrics,omitempty"`
	StartedAt         time.Time              `json:"started_at,omitempty"`
	FinishedAt        time.Time              `json:"finished_at,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at,omitempty"`
}

// RolloutMetricsSnapshot 是发布窗口聚合指标快照。
type RolloutMetricsSnapshot struct {
	WindowStart        time.Time `json:"window_start,omitempty"`
	WindowEnd          time.Time `json:"window_end,omitempty"`
	RequestsTotal      int64     `json:"requests_total,omitempty"`
	ErrorRate          float64   `json:"error_rate,omitempty"`
	P95LatencyMillis   int64     `json:"p95_latency_millis,omitempty"`
	FallbackRequests   int64     `json:"fallback_requests,omitempty"`
	FallbackRate       float64   `json:"fallback_rate,omitempty"`
	MeanCost           float64   `json:"mean_cost,omitempty"`
}

// DistributionEventType 表示策略分发生命周期事件类型。
type DistributionEventType string

const (
	DistributionEventActivated DistributionEventType = "policy_distribution.activated"
	DistributionEventRollback  DistributionEventType = "policy_distribution.rollback"
)

// DistributionEvent 表示一次策略分发事件。
type DistributionEvent struct {
	ID              string                `json:"id"`
	PolicyVersionID string                `json:"policy_version_id,omitempty"`
	RolloutID       string                `json:"rollout_id,omitempty"`
	Environment     string                `json:"environment"`
	EventType       DistributionEventType `json:"event_type"`
	Payload         map[string]any        `json:"payload,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
}

// RolloutGuardVerdict 表示 rollout 指标守卫结论。
type RolloutGuardVerdict string

const (
	RolloutGuardKeep               RolloutGuardVerdict = "keep"
	RolloutGuardPause              RolloutGuardVerdict = "pause"
	RolloutGuardRollbackSuggested  RolloutGuardVerdict = "rollback_suggested"
	RolloutGuardRollbackRequired   RolloutGuardVerdict = "rollback_required"
)

// RolloutGuardThresholds 定义指标守卫阈值。
type RolloutGuardThresholds struct {
	MinRequests                       int64   `json:"min_requests,omitempty"`
	PauseErrorRateGTE                 float64 `json:"pause_error_rate_gte,omitempty"`
	RollbackSuggestedErrorRateGTE     float64 `json:"rollback_suggested_error_rate_gte,omitempty"`
	RollbackRequiredErrorRateGTE      float64 `json:"rollback_required_error_rate_gte,omitempty"`
	PauseP95LatencyMillisGTE          int64   `json:"pause_p95_latency_millis_gte,omitempty"`
	RollbackSuggestedP95LatencyGTE    int64   `json:"rollback_suggested_p95_latency_millis_gte,omitempty"`
	RollbackRequiredP95LatencyGTE     int64   `json:"rollback_required_p95_latency_millis_gte,omitempty"`
	PauseFallbackRateGTE              float64 `json:"pause_fallback_rate_gte,omitempty"`
	RollbackSuggestedFallbackRateGTE  float64 `json:"rollback_suggested_fallback_rate_gte,omitempty"`
	RollbackRequiredFallbackRateGTE   float64 `json:"rollback_required_fallback_rate_gte,omitempty"`
}

// RolloutGuardResult 表示 rollout 指标守卫输出。
type RolloutGuardResult struct {
	Verdict RolloutGuardVerdict   `json:"verdict"`
	Summary string                `json:"summary,omitempty"`
	Metrics RolloutMetricsSnapshot `json:"metrics,omitempty"`
}

// StartRolloutInput 表示启动 rollout 的输入。
type StartRolloutInput struct {
	PolicyVersionID string `json:"policy_version_id"`
	RolloutMode     string `json:"rollout_mode,omitempty"`
	RolloutPercent  int    `json:"rollout_percent,omitempty"`
	TriggerReason   string `json:"trigger_reason,omitempty"`
	TriggeredBy     string `json:"triggered_by"`
}

// PromoteRolloutInput 表示 rollout 推进输入。
type PromoteRolloutInput struct {
	RolloutID       string `json:"rollout_id"`
	RolloutPercent  int    `json:"rollout_percent"`
	GuardSummary    string `json:"guard_summary,omitempty"`
}

// RolloutMetricsQuery 表示 rollout 指标聚合查询。
type RolloutMetricsQuery struct {
	RolloutID       string `json:"rollout_id,omitempty"`
	PolicyVersionID string `json:"policy_version_id,omitempty"`
}

// ExecuteRollbackInput 表示回滚执行输入。
type ExecuteRollbackInput struct {
	RolloutID string `json:"rollout_id"`
	Actor     string `json:"actor"`
	Reason    string `json:"reason,omitempty"`
}

// ExecuteRollbackResult 表示回滚执行结果。
type ExecuteRollbackResult struct {
	Rollout                 Rollout      `json:"rollout"`
	RestoredPolicyVersionID string       `json:"restored_policy_version_id"`
	RevertedPolicyVersionID string       `json:"reverted_policy_version_id"`
	DistributionEvent       DistributionEvent `json:"distribution_event"`
}

// EvaluationDataset 表示评估数据集注册。
type EvaluationDataset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	TaskType    string    `json:"task_type"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ScoringFormula 表示评分公式版本。
type ScoringFormula struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	FormulaJSON []byte    `json:"formula_json"`
	CreatedAt   time.Time `json:"created_at"`
}

// EvaluationDatasetInput 表示创建评估数据集输入。
type EvaluationDatasetInput struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	TaskType    string `json:"task_type"`
	Description string `json:"description,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
}

// ScoringFormulaInput 表示创建评分公式输入。
type ScoringFormulaInput struct {
	Version     string `json:"version"`
	FormulaJSON []byte `json:"formula_json"`
	CreatedBy   string `json:"created_by,omitempty"`
}

// StartEvaluationRunInput 表示发起评估运行输入。
type StartEvaluationRunInput struct {
	DatasetID        string `json:"dataset_id"`
	AgentID          string `json:"agent_id"`
	TaskType         string `json:"task_type"`
	Environment      string `json:"environment"`
	FormulaVersionID string `json:"formula_version_id,omitempty"`
}

// EvaluationRun 表示一次评估运行。
type EvaluationRun struct {
	ID               string              `json:"id"`
	DatasetID        string              `json:"dataset_id"`
	AgentID          string              `json:"agent_id"`
	TaskType         string              `json:"task_type"`
	Environment      string              `json:"environment"`
	FormulaVersionID string              `json:"formula_version_id,omitempty"`
	Status           EvaluationRunStatus `json:"status"`
	Summary          string              `json:"summary,omitempty"`
	StartedAt        time.Time           `json:"started_at,omitempty"`
	FinishedAt       time.Time           `json:"finished_at,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
}

// EvaluationResult 表示评估结果项。
type EvaluationResult struct {
	ID             string         `json:"id"`
	RunID          string         `json:"run_id"`
	ModelID        string         `json:"model_id"`
	CompositeScore float64        `json:"composite_score"`
	Breakdown      ScoreBreakdown `json:"breakdown,omitempty"`
	Passed         bool           `json:"passed"`
	Notes          string         `json:"notes,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// RuntimeDecisionSnapshot 是 runtime 决策不可变快照。
type RuntimeDecisionSnapshot struct {
	ID              string    `json:"id"`
	RequestID       string    `json:"request_id"`
	TenantID        string    `json:"tenant_id"`
	Environment     string    `json:"environment"`
	AgentID         string    `json:"agent_id"`
	TaskType        string    `json:"task_type,omitempty"`
	PolicyVersionID string    `json:"policy_version_id"`
	ResolvedModelID string    `json:"resolved_model_id"`
	FallbackUsed    bool      `json:"fallback_used"`
	LatencyMillis   int64     `json:"latency_millis,omitempty"`
	InputTokens     int64     `json:"input_tokens,omitempty"`
	OutputTokens    int64     `json:"output_tokens,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// PolicyDrift 表示活动策略与最新推荐之间的偏移记录。
type PolicyDrift struct {
	ID                  string            `json:"id"`
	TenantID            string            `json:"tenant_id"`
	Environment         string            `json:"environment"`
	AgentID             string            `json:"agent_id"`
	Status              PolicyDriftStatus `json:"status"`
	ActivePolicyVersion string            `json:"active_policy_version"`
	RecommendedModelID  string            `json:"recommended_model_id"`
	CurrentModelID      string            `json:"current_model_id"`
	Distance            float64           `json:"distance,omitempty"`
	Reason              string            `json:"reason,omitempty"`
	DetectedAt          time.Time         `json:"detected_at"`
	ResolvedAt          time.Time         `json:"resolved_at,omitempty"`
}
