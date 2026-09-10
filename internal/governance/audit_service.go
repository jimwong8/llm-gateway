package governance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const (
	governanceEventRecommendationGenerated = "governance.recommendation.generated"
	governanceEventApprovalDecided         = "governance.approval.decided"
	governanceEventPolicyVersionCreated    = "governance.policy.version.created"
	governanceEventRolloutStarted          = "governance.rollout.started"
	governanceEventRolloutPromoted         = "governance.rollout.promoted"
	governanceEventRollbackExecuted        = "governance.rollback.executed"
	governanceEventDriftDetected           = "governance.drift.detected"

	AuditActorSystem = "system"
)

// GovernanceAuditRepo 提供治理审计事件持久化。
type GovernanceAuditRepo struct {
	db  *sql.DB
	now func() time.Time
	seq atomic.Int64
}

func NewGovernanceAuditRepo(store *Store) *GovernanceAuditRepo {
	if store == nil {
		return nil
	}
	return &GovernanceAuditRepo{db: store.DB(), now: time.Now}
}

func (r *GovernanceAuditRepo) EmitGovernanceEvent(ctx context.Context, eventType string, actorID string, entityType string, entityID string, payload map[string]any) error {
	if r == nil || r.db == nil {
		return errors.New("governance audit repo is nil")
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return errors.New("event_type is required")
	}
	actorID = strings.TrimSpace(actorID)
	entityType = strings.TrimSpace(entityType)
	entityID = strings.TrimSpace(entityID)
	if actorID == "" {
		actorID = AuditActorSystem
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO governance_audit_logs (
    event_id,
    event_type,
    actor_id,
    entity_type,
    entity_id,
    payload,
    created_at
) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),$6::jsonb,$7)
`, r.nextID("gov_audit"), eventType, actorID, entityType, entityID, string(payloadJSON), r.now().UTC())
	return err
}

func (r *GovernanceAuditRepo) nextID(prefix string) string {
	n := r.seq.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, r.now().UTC().UnixNano(), n)
}

// GovernanceAuditService 对治理事件提供薄封装。
type GovernanceAuditService struct {
	emitter governanceAuditEmitter
}

func NewGovernanceAuditService(emitter governanceAuditEmitter) *GovernanceAuditService {
	return &GovernanceAuditService{emitter: emitter}
}

func (s *GovernanceAuditService) RecommendationGenerated(ctx context.Context, actorID, recommendationID string, payload map[string]any) error {
	return s.emit(ctx, governanceEventRecommendationGenerated, actorID, "model_recommendation", recommendationID, payload)
}

func (s *GovernanceAuditService) ApprovalDecided(ctx context.Context, actorID, recommendationID string, payload map[string]any) error {
	return s.emit(ctx, governanceEventApprovalDecided, actorID, "model_recommendation", recommendationID, payload)
}

func (s *GovernanceAuditService) PolicyVersionCreated(ctx context.Context, actorID, policyVersionID string, payload map[string]any) error {
	return s.emit(ctx, governanceEventPolicyVersionCreated, actorID, "model_policy_version", policyVersionID, payload)
}

func (s *GovernanceAuditService) RolloutStarted(ctx context.Context, actorID, rolloutID string, payload map[string]any) error {
	return s.emit(ctx, governanceEventRolloutStarted, actorID, "model_rollout", rolloutID, payload)
}

func (s *GovernanceAuditService) RolloutPromoted(ctx context.Context, actorID, rolloutID string, payload map[string]any) error {
	return s.emit(ctx, governanceEventRolloutPromoted, actorID, "model_rollout", rolloutID, payload)
}

func (s *GovernanceAuditService) RollbackExecuted(ctx context.Context, actorID, policyVersionID string, payload map[string]any) error {
	return s.emit(ctx, governanceEventRollbackExecuted, actorID, "model_policy_version", policyVersionID, payload)
}

func (s *GovernanceAuditService) DriftDetected(ctx context.Context, actorID, driftID string, payload map[string]any) error {
	return s.emit(ctx, governanceEventDriftDetected, actorID, "policy_drift", driftID, payload)
}

func (s *GovernanceAuditService) emit(ctx context.Context, eventType, actorID, entityType, entityID string, payload map[string]any) error {
	if s == nil || s.emitter == nil {
		return nil
	}
	return s.emitter.EmitGovernanceEvent(ctx, eventType, strings.TrimSpace(actorID), strings.TrimSpace(entityType), strings.TrimSpace(entityID), payload)
}
