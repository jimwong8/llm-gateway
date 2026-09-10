package governance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// RuntimeDecisionSnapshotWrite 是 runtime 快照落库入参。
type RuntimeDecisionSnapshotWrite struct {
	RequestID          string
	PolicyVersionID    string
	RolloutID          string
	Environment        string
	TenantID           string
	AgentID            string
	TaskType           string
	MatchedScopeType   string
	MatchedScope       map[string]string
	ResolvedModel      string
	FallbackChain      []string
	PolicyFallbackUsed bool
	SystemFallbackUsed bool
	LatencyMS          int64
	Success            bool
	ErrorType          string
	ErrorMessage       string
	CreatedAt          time.Time
}

// SnapshotRepo 负责 runtime 决策快照持久化。
type SnapshotRepo struct {
	db *sql.DB
}

func NewSnapshotRepo(store *Store) *SnapshotRepo {
	if store == nil {
		return nil
	}
	return &SnapshotRepo{db: store.DB()}
}

func (r *SnapshotRepo) Save(ctx context.Context, snapshot RuntimeDecisionSnapshotWrite) error {
	if r == nil || r.db == nil {
		return errors.New("snapshot repo is not initialized")
	}

	snapshot.RequestID = strings.TrimSpace(snapshot.RequestID)
	snapshot.PolicyVersionID = strings.TrimSpace(snapshot.PolicyVersionID)
	snapshot.RolloutID = strings.TrimSpace(snapshot.RolloutID)
	snapshot.Environment = strings.TrimSpace(snapshot.Environment)
	snapshot.TenantID = strings.TrimSpace(snapshot.TenantID)
	snapshot.AgentID = strings.TrimSpace(snapshot.AgentID)
	snapshot.TaskType = strings.TrimSpace(snapshot.TaskType)
	snapshot.MatchedScopeType = strings.TrimSpace(snapshot.MatchedScopeType)
	snapshot.ResolvedModel = strings.TrimSpace(snapshot.ResolvedModel)
	snapshot.ErrorType = strings.TrimSpace(snapshot.ErrorType)
	snapshot.ErrorMessage = strings.TrimSpace(snapshot.ErrorMessage)

	if snapshot.RequestID == "" {
		return errors.New("request_id is required")
	}
	if snapshot.Environment == "" {
		return errors.New("environment is required")
	}
	if snapshot.AgentID == "" {
		return errors.New("agent_id is required")
	}
	if snapshot.ResolvedModel == "" {
		return errors.New("resolved_model is required")
	}

	matchedScopeJSON, err := json.Marshal(snapshotMapOrEmpty(snapshot.MatchedScope))
	if err != nil {
		return err
	}
	fallbackJSON, err := json.Marshal(snapshotSliceOrEmpty(snapshot.FallbackChain))
	if err != nil {
		return err
	}

	createdAt := snapshot.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	} else {
		createdAt = createdAt.UTC()
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO runtime_decision_snapshots (
	request_id,
	policy_version_id,
	rollout_id,
	environment,
	tenant_id,
	agent_id,
	task_type,
	matched_scope_type,
	matched_scope,
	resolved_model,
	fallback_chain,
	policy_fallback_used,
	system_fallback_used,
	latency_ms,
	success,
	error_type,
	error_message,
	created_at
) VALUES (
	$1,
	NULLIF($2, ''),
	NULLIF($3, ''),
	$4,
	NULLIF($5, ''),
	$6,
	NULLIF($7, ''),
	NULLIF($8, ''),
	$9::jsonb,
	$10,
	$11::jsonb,
	$12,
	$13,
	$14,
	$15,
	NULLIF($16, ''),
	NULLIF($17, ''),
	$18
)
`,
		snapshot.RequestID,
		snapshot.PolicyVersionID,
		snapshot.RolloutID,
		snapshot.Environment,
		snapshot.TenantID,
		snapshot.AgentID,
		snapshot.TaskType,
		snapshot.MatchedScopeType,
		string(matchedScopeJSON),
		snapshot.ResolvedModel,
		string(fallbackJSON),
		snapshot.PolicyFallbackUsed,
		snapshot.SystemFallbackUsed,
		snapshot.LatencyMS,
		snapshot.Success,
		snapshot.ErrorType,
		snapshot.ErrorMessage,
		createdAt,
	)
	if err != nil {
		return err
	}
	return nil
}

func snapshotMapOrEmpty(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	copied := make(map[string]string, len(in))
	for k, v := range in {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		copied[key] = strings.TrimSpace(v)
	}
	if len(copied) == 0 {
		return map[string]string{}
	}
	return copied
}

func snapshotSliceOrEmpty(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	for _, item := range in {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}
