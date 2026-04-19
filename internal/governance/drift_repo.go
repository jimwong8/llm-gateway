package governance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrActivePolicyVersionNotFound = errors.New("active policy version not found")
	ErrActivePolicyModelNotFound   = errors.New("active policy model not found")
	ErrRecommendationNotFound      = errors.New("latest recommendation not found")
	ErrPolicyDriftNotFound         = errors.New("policy drift not found")
)

// DriftRepo 负责 policy drift 相关数据访问。
type DriftRepo struct {
	db  *sql.DB
	now func() time.Time
	seq int64
}

func NewDriftRepo(store *Store) *DriftRepo {
	if store == nil {
		return nil
	}
	return &DriftRepo{db: store.DB(), now: time.Now}
}

func (r *DriftRepo) LoadActiveModel(ctx context.Context, environment, agentID string) (string, string, error) {
	if r == nil || r.db == nil {
		return "", "", errors.New("drift repo is nil")
	}
	environment = strings.TrimSpace(environment)
	agentID = strings.TrimSpace(agentID)
	if environment == "" || agentID == "" {
		return "", "", errors.New("environment and agent_id are required")
	}

	var (
		policyVersionID string
		policyJSON      []byte
	)
	err := r.db.QueryRowContext(ctx, `
SELECT policy_version_id, policy_json
FROM model_policy_versions
WHERE environment = $1 AND status = 'active'
ORDER BY COALESCE(activated_at, created_at) DESC, id DESC
LIMIT 1
`, environment).Scan(&policyVersionID, &policyJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrActivePolicyVersionNotFound
		}
		return "", "", err
	}

	var policy RuntimePolicy
	if err := json.Unmarshal(policyJSON, &policy); err != nil {
		return "", "", err
	}

	activeModel := strings.TrimSpace(policy.DefaultModel)
	if policy.Agents != nil {
		if agent, ok := policy.Agents[agentID]; ok {
			if primary := strings.TrimSpace(agent.PrimaryModel); primary != "" {
				activeModel = primary
			}
		}
	}
	if activeModel == "" {
		return "", "", ErrActivePolicyModelNotFound
	}

	return strings.TrimSpace(policyVersionID), activeModel, nil
}

func (r *DriftRepo) LoadLatestRecommendation(ctx context.Context, environment, agentID string) (string, string, error) {
	if r == nil || r.db == nil {
		return "", "", errors.New("drift repo is nil")
	}
	environment = strings.TrimSpace(environment)
	agentID = strings.TrimSpace(agentID)
	if environment == "" || agentID == "" {
		return "", "", errors.New("environment and agent_id are required")
	}

	var recommendationID string
	var recommendedModel string
	err := r.db.QueryRowContext(ctx, `
SELECT recommendation_id, recommended_model
FROM model_recommendations
WHERE environment = $1 AND agent_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 1
`, environment, agentID).Scan(&recommendationID, &recommendedModel)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrRecommendationNotFound
		}
		return "", "", err
	}
	return strings.TrimSpace(recommendationID), strings.TrimSpace(recommendedModel), nil
}

func (r *DriftRepo) CreateDriftRecord(ctx context.Context, drift PolicyDrift) (PolicyDrift, error) {
	if r == nil || r.db == nil {
		return PolicyDrift{}, errors.New("drift repo is nil")
	}

	now := r.now().UTC()
	if strings.TrimSpace(drift.ID) == "" {
		drift.ID = r.nextID("drift")
	}
	if drift.DetectedAt.IsZero() {
		drift.DetectedAt = now
	}
	if drift.Status == "" {
		drift.Status = PolicyDriftStatusDetected
	}
	if !drift.Status.Valid() {
		return PolicyDrift{}, fmt.Errorf("invalid drift status: %s", drift.Status)
	}

	details := map[string]any{
		"tenant_id":             strings.TrimSpace(drift.TenantID),
		"active_policy_version": strings.TrimSpace(drift.ActivePolicyVersion),
		"current_model_id":      strings.TrimSpace(drift.CurrentModelID),
		"recommended_model_id":  strings.TrimSpace(drift.RecommendedModelID),
		"reason":                strings.TrimSpace(drift.Reason),
		"distance":              drift.Distance,
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return PolicyDrift{}, err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO policy_drifts (
    drift_id,
    environment,
    agent_id,
    active_model,
    recommended_model,
    drift_type,
    status,
    details,
    detected_at,
    updated_at
) VALUES ($1,$2,$3,$4,$5,'model_mismatch',$6,$7::jsonb,$8,$9)
`,
		strings.TrimSpace(drift.ID),
		strings.TrimSpace(drift.Environment),
		strings.TrimSpace(drift.AgentID),
		strings.TrimSpace(drift.CurrentModelID),
		strings.TrimSpace(drift.RecommendedModelID),
		toDBDriftStatus(drift.Status),
		string(detailsJSON),
		drift.DetectedAt.UTC(),
		now,
	)
	if err != nil {
		return PolicyDrift{}, err
	}

	return r.GetDrift(ctx, drift.ID)
}

func (r *DriftRepo) UpdateDriftStatus(ctx context.Context, driftID string, status PolicyDriftStatus, reason string) (PolicyDrift, error) {
	if r == nil || r.db == nil {
		return PolicyDrift{}, errors.New("drift repo is nil")
	}
	driftID = strings.TrimSpace(driftID)
	reason = strings.TrimSpace(reason)
	if driftID == "" {
		return PolicyDrift{}, errors.New("drift_id is required")
	}
	if !status.Valid() {
		return PolicyDrift{}, fmt.Errorf("invalid drift status: %s", status)
	}

	now := r.now().UTC()
	patch := map[string]any{
		"transition_reason": reason,
	}
	if status == PolicyDriftStatusResolved {
		patch["resolved_at"] = now
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return PolicyDrift{}, err
	}

	res, err := r.db.ExecContext(ctx, `
UPDATE policy_drifts
SET status = $2,
    updated_at = $3,
    details = details || $4::jsonb
WHERE drift_id = $1
`, driftID, toDBDriftStatus(status), now, string(patchJSON))
	if err != nil {
		return PolicyDrift{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return PolicyDrift{}, err
	}
	if affected == 0 {
		return PolicyDrift{}, ErrPolicyDriftNotFound
	}

	return r.GetDrift(ctx, driftID)
}

func (r *DriftRepo) GetDrift(ctx context.Context, driftID string) (PolicyDrift, error) {
	if r == nil || r.db == nil {
		return PolicyDrift{}, errors.New("drift repo is nil")
	}
	driftID = strings.TrimSpace(driftID)
	if driftID == "" {
		return PolicyDrift{}, errors.New("drift_id is required")
	}

	var (
		environment      string
		agentID          string
		activeModel      string
		recommendedModel string
		statusRaw        string
		detailsRaw       []byte
		detectedAt       time.Time
		updatedAt        time.Time
	)
	err := r.db.QueryRowContext(ctx, `
SELECT environment, agent_id, active_model, recommended_model, status, details, detected_at, updated_at
FROM policy_drifts
WHERE drift_id = $1
`, driftID).Scan(&environment, &agentID, &activeModel, &recommendedModel, &statusRaw, &detailsRaw, &detectedAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PolicyDrift{}, ErrPolicyDriftNotFound
		}
		return PolicyDrift{}, err
	}

	result := PolicyDrift{
		ID:                 strings.TrimSpace(driftID),
		Environment:        strings.TrimSpace(environment),
		AgentID:            strings.TrimSpace(agentID),
		Status:             fromDBDriftStatus(statusRaw),
		CurrentModelID:     strings.TrimSpace(activeModel),
		RecommendedModelID: strings.TrimSpace(recommendedModel),
		DetectedAt:         detectedAt.UTC(),
	}

	if len(detailsRaw) > 0 {
		var details map[string]any
		if err := json.Unmarshal(detailsRaw, &details); err == nil {
			if tenantID, ok := details["tenant_id"].(string); ok {
				result.TenantID = strings.TrimSpace(tenantID)
			}
			if activeVersion, ok := details["active_policy_version"].(string); ok {
				result.ActivePolicyVersion = strings.TrimSpace(activeVersion)
			}
			if reason, ok := details["reason"].(string); ok {
				result.Reason = strings.TrimSpace(reason)
			}
			if distance, ok := details["distance"].(float64); ok {
				result.Distance = distance
			}
			if resolvedAtRaw, ok := details["resolved_at"]; ok {
				if resolvedAtText, ok := resolvedAtRaw.(string); ok {
					if parsed, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(resolvedAtText)); parseErr == nil {
						result.ResolvedAt = parsed.UTC()
					}
				}
			}
		}
	}

	if result.Status == PolicyDriftStatusResolved && result.ResolvedAt.IsZero() {
		result.ResolvedAt = updatedAt.UTC()
	}

	return result, nil
}

func toDBDriftStatus(status PolicyDriftStatus) string {
	switch status {
	case PolicyDriftStatusAccepted:
		return "acknowledged"
	case PolicyDriftStatusResolved:
		return "resolved"
	default:
		return "open"
	}
}

func fromDBDriftStatus(status string) PolicyDriftStatus {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "acknowledged":
		return PolicyDriftStatusAccepted
	case "resolved":
		return PolicyDriftStatusResolved
	default:
		return PolicyDriftStatusDetected
	}
}

func (r *DriftRepo) nextID(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d_%d", prefix, r.now().UTC().UnixNano(), r.seq)
}
