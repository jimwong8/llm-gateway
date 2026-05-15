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

var (
	ErrApprovalNotFound          = errors.New("approval not found")
	ErrApprovalNotApproved       = errors.New("approval is not approved")
	ErrPolicyVersionNotFound     = errors.New("policy version not found")
	ErrInvalidVersionTransition  = errors.New("invalid policy version transition")
	ErrNoEnvironmentInScope      = errors.New("approval scope missing environment")
	ErrNoModelResolvedFromSource = errors.New("unable to resolve target model from approval source")
)

type approvalSource struct {
	ApprovalID       string
	RecommendationID string
	Decision         string
	FinalModel       string
	Environment      string
	AgentID          string
	RecommendedModel string
}

type VersionRepo struct {
	db  *sql.DB
	now func() time.Time
	seq atomic.Int64
}

func NewVersionRepo(store *Store) *VersionRepo {
	if store == nil {
		return nil
	}
	return &VersionRepo{
		db:  store.DB(),
		now: time.Now,
	}
}

func (r *VersionRepo) CreateDraftFromApproval(ctx context.Context, approvalID, createdBy string) (PolicyVersion, error) {
	if r == nil || r.db == nil {
		return PolicyVersion{}, errors.New("version repo is nil")
	}
	approvalID = strings.TrimSpace(approvalID)
	createdBy = strings.TrimSpace(createdBy)
	if approvalID == "" {
		return PolicyVersion{}, errors.New("approval_id is required")
	}
	if createdBy == "" {
		createdBy = "system"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PolicyVersion{}, err
	}
	defer func() { _ = tx.Rollback() }()

	source, err := r.loadApprovalSourceTx(ctx, tx, approvalID)
	if err != nil {
		return PolicyVersion{}, err
	}
	if source.Decision != string(ApprovalStatusApproved) {
		return PolicyVersion{}, ErrApprovalNotApproved
	}
	if strings.TrimSpace(source.Environment) == "" {
		return PolicyVersion{}, ErrNoEnvironmentInScope
	}

	nextVersion, err := r.nextVersionNumberTx(ctx, tx, source.Environment)
	if err != nil {
		return PolicyVersion{}, err
	}

	resolvedModel := strings.TrimSpace(source.FinalModel)
	if resolvedModel == "" {
		resolvedModel = strings.TrimSpace(source.RecommendedModel)
	}
	if resolvedModel == "" {
		return PolicyVersion{}, ErrNoModelResolvedFromSource
	}

	policy := RuntimePolicy{
		Version:     nextVersion,
		Environment: source.Environment,
	}
	if strings.TrimSpace(source.AgentID) != "" {
		policy.Agents = map[string]AgentPolicy{
			source.AgentID: {
				PrimaryModel: resolvedModel,
			},
		}
	} else {
		policy.DefaultModel = resolvedModel
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return PolicyVersion{}, err
	}

	versionID := r.nextID("policy_version")
	createdAt := r.now().UTC()
	_, err = tx.ExecContext(ctx, `
INSERT INTO model_policy_versions (
    policy_version_id,
    environment,
    status,
    policy_json,
    source_approval_id,
    created_by,
    created_at
) VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7)
`, versionID, source.Environment, string(PolicyVersionDraft), string(policyJSON), source.ApprovalID, createdBy, createdAt)
	if err != nil {
		return PolicyVersion{}, err
	}

	if err := tx.Commit(); err != nil {
		return PolicyVersion{}, err
	}

	return PolicyVersion{
		ID:          versionID,
		Environment: source.Environment,
		Version:     nextVersion,
		Status:      PolicyVersionDraft,
		Policy:      policy,
		CreatedBy:   createdBy,
		CreatedAt:   createdAt,
	}, nil
}

func (r *VersionRepo) ApproveVersion(ctx context.Context, versionID, approvedBy string) (PolicyVersion, error) {
	if r == nil || r.db == nil {
		return PolicyVersion{}, errors.New("version repo is nil")
	}
	versionID = strings.TrimSpace(versionID)
	approvedBy = strings.TrimSpace(approvedBy)
	if versionID == "" {
		return PolicyVersion{}, errors.New("policy_version_id is required")
	}
	if approvedBy == "" {
		return PolicyVersion{}, errors.New("approved_by is required")
	}

	now := r.now().UTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE model_policy_versions
SET status = $2,
    approved_by = $3,
    approved_at = $4
WHERE policy_version_id = $1
  AND status = $5
`, versionID, string(PolicyVersionApproved), approvedBy, now, string(PolicyVersionDraft))
	if err != nil {
		return PolicyVersion{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return PolicyVersion{}, err
	}
	if affected == 0 {
		current, getErr := r.GetVersion(ctx, versionID)
		if getErr != nil {
			return PolicyVersion{}, getErr
		}
		return PolicyVersion{}, fmt.Errorf("%w: current status is %s", ErrInvalidVersionTransition, current.Status)
	}

	return r.GetVersion(ctx, versionID)
}

func (r *VersionRepo) ActivateVersion(ctx context.Context, versionID string) (PolicyVersion, error) {
	if r == nil || r.db == nil {
		return PolicyVersion{}, errors.New("version repo is nil")
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return PolicyVersion{}, errors.New("policy_version_id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PolicyVersion{}, err
	}
	defer func() { _ = tx.Rollback() }()

	current, err := r.getVersionTx(ctx, tx, versionID)
	if err != nil {
		return PolicyVersion{}, err
	}
	if current.Status != PolicyVersionApproved {
		return PolicyVersion{}, fmt.Errorf("%w: current status is %s", ErrInvalidVersionTransition, current.Status)
	}

	_, err = tx.ExecContext(ctx, `
UPDATE model_policy_versions
SET status = $2
WHERE environment = $1
  AND status = $3
  AND policy_version_id <> $4
`, current.Environment, string(PolicyVersionSuperseded), string(PolicyVersionActive), versionID)
	if err != nil {
		return PolicyVersion{}, err
	}

	activatedAt := r.now().UTC()
	_, err = tx.ExecContext(ctx, `
UPDATE model_policy_versions
SET status = $2,
    activated_at = $3
WHERE policy_version_id = $1
`, versionID, string(PolicyVersionActive), activatedAt)
	if err != nil {
		return PolicyVersion{}, err
	}

	if err := tx.Commit(); err != nil {
		return PolicyVersion{}, err
	}

	return r.GetVersion(ctx, versionID)
}

func (r *VersionRepo) GetVersion(ctx context.Context, versionID string) (PolicyVersion, error) {
	if r == nil || r.db == nil {
		return PolicyVersion{}, errors.New("version repo is nil")
	}
	return r.getVersionTx(ctx, r.db, strings.TrimSpace(versionID))
}

func (r *VersionRepo) ActiveVersionCountByEnvironment(ctx context.Context, environment string) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("version repo is nil")
	}
	var cnt int
	err := r.db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM model_policy_versions
WHERE environment = $1 AND status = $2
`, strings.TrimSpace(environment), string(PolicyVersionActive)).Scan(&cnt)
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (r *VersionRepo) GetDiffBaseVersion(ctx context.Context, current PolicyVersion) (PolicyVersion, string, error) {
	if r == nil || r.db == nil {
		return PolicyVersion{}, "", errors.New("version repo is nil")
	}
	if strings.TrimSpace(current.SourceApprovalID) != "" {
		base, err := r.findBySourceApprovalAndEnvironment(ctx, current.SourceApprovalID, current.Environment, current.ID)
		if err != nil {
			return PolicyVersion{}, "", err
		}
		if strings.TrimSpace(base.ID) != "" {
			return base, "source", nil
		}
	}

	base, err := r.findPreviousByCreatedAt(ctx, current)
	if err != nil {
		return PolicyVersion{}, "", err
	}
	if strings.TrimSpace(base.ID) == "" {
		return PolicyVersion{}, "none", nil
	}
	return base, "previous", nil
}

func (r *VersionRepo) loadApprovalSourceTx(ctx context.Context, q queryer, approvalID string) (approvalSource, error) {
	var (
		src      approvalSource
		scopeRaw []byte
	)
	err := q.QueryRowContext(ctx, `
SELECT approval_id, recommendation_id, decision, COALESCE(final_model, ''), effective_scope
FROM model_approvals
WHERE approval_id = $1
`, approvalID).Scan(&src.ApprovalID, &src.RecommendationID, &src.Decision, &src.FinalModel, &scopeRaw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return approvalSource{}, ErrApprovalNotFound
		}
		return approvalSource{}, err
	}
	src.Decision = strings.TrimSpace(src.Decision)
	src.FinalModel = strings.TrimSpace(src.FinalModel)

	if len(scopeRaw) > 0 {
		var scope EffectiveScope
		if err := json.Unmarshal(scopeRaw, &scope); err == nil {
			src.Environment = strings.TrimSpace(scope.Environment)
		}
	}

	if strings.TrimSpace(src.RecommendationID) != "" {
		err := q.QueryRowContext(ctx, `
SELECT agent_id, recommended_model
FROM model_recommendations
WHERE recommendation_id = $1
`, src.RecommendationID).Scan(&src.AgentID, &src.RecommendedModel)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return approvalSource{}, err
		}
		src.AgentID = strings.TrimSpace(src.AgentID)
		src.RecommendedModel = strings.TrimSpace(src.RecommendedModel)
	}

	return src, nil
}

func (r *VersionRepo) nextVersionNumberTx(ctx context.Context, q queryer, environment string) (int64, error) {
	rows, err := q.QueryContext(ctx, `
SELECT policy_json
FROM model_policy_versions
WHERE environment = $1
`, environment)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var maxVersion int64
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return 0, err
		}
		var policy RuntimePolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			continue
		}
		if policy.Version > maxVersion {
			maxVersion = policy.Version
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return maxVersion + 1, nil
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (r *VersionRepo) getVersionTx(ctx context.Context, q queryer, versionID string) (PolicyVersion, error) {
	var (
		version               PolicyVersion
		statusRaw             string
		policyRaw             []byte
		sourceApprovalID      sql.NullString
		approvedBy            sql.NullString
		approvedAt            sql.NullTime
		activatedAt           sql.NullTime
		createdAt             time.Time
		createdBy             string
		environment           string
		selectedPolicyVersion string
	)
	err := q.QueryRowContext(ctx, `
SELECT policy_version_id, environment, status, policy_json, source_approval_id, created_by, approved_by, approved_at, activated_at, created_at
FROM model_policy_versions
WHERE policy_version_id = $1
`, versionID).Scan(
		&selectedPolicyVersion,
		&environment,
		&statusRaw,
		&policyRaw,
		&sourceApprovalID,
		&createdBy,
		&approvedBy,
		&approvedAt,
		&activatedAt,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PolicyVersion{}, ErrPolicyVersionNotFound
		}
		return PolicyVersion{}, err
	}

	var policy RuntimePolicy
	if len(policyRaw) > 0 {
		if err := json.Unmarshal(policyRaw, &policy); err != nil {
			return PolicyVersion{}, err
		}
	}

	version = PolicyVersion{
		ID:          selectedPolicyVersion,
		Environment: strings.TrimSpace(environment),
		Status:      PolicyVersionStatus(strings.TrimSpace(statusRaw)),
		Policy:      policy,
		Version:     policy.Version,
		CreatedBy:   strings.TrimSpace(createdBy),
		CreatedAt:   createdAt.UTC(),
	}
	if approvedBy.Valid {
		version.ApprovedBy = strings.TrimSpace(approvedBy.String)
	}
	if approvedAt.Valid {
		version.ApprovedAt = approvedAt.Time.UTC()
	}
	if activatedAt.Valid {
		version.ActivatedAt = activatedAt.Time.UTC()
	}
	if sourceApprovalID.Valid {
		version.SourceApprovalID = strings.TrimSpace(sourceApprovalID.String)
		version.Summary = fmt.Sprintf("source_approval_id=%s", version.SourceApprovalID)
	}

	return version, nil
}

func (r *VersionRepo) findBySourceApprovalAndEnvironment(ctx context.Context, sourceApprovalID, environment, excludeVersionID string) (PolicyVersion, error) {
	sourceApprovalID = strings.TrimSpace(sourceApprovalID)
	environment = strings.TrimSpace(environment)
	excludeVersionID = strings.TrimSpace(excludeVersionID)
	if sourceApprovalID == "" || environment == "" {
		return PolicyVersion{}, nil
	}

	var versionID string
	err := r.db.QueryRowContext(ctx, `
SELECT policy_version_id
FROM model_policy_versions
WHERE source_approval_id = $1
  AND environment = $2
  AND policy_version_id <> $3
ORDER BY created_at DESC, id DESC
LIMIT 1
`, sourceApprovalID, environment, excludeVersionID).Scan(&versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PolicyVersion{}, nil
		}
		return PolicyVersion{}, err
	}
	return r.GetVersion(ctx, strings.TrimSpace(versionID))
}

func (r *VersionRepo) findPreviousByCreatedAt(ctx context.Context, current PolicyVersion) (PolicyVersion, error) {
	if strings.TrimSpace(current.ID) == "" || strings.TrimSpace(current.Environment) == "" {
		return PolicyVersion{}, nil
	}
	var versionID string
	err := r.db.QueryRowContext(ctx, `
SELECT policy_version_id
FROM model_policy_versions
WHERE environment = $1
  AND policy_version_id <> $2
  AND created_at < $3
ORDER BY created_at DESC, id DESC
LIMIT 1
`, strings.TrimSpace(current.Environment), strings.TrimSpace(current.ID), current.CreatedAt).Scan(&versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PolicyVersion{}, nil
		}
		return PolicyVersion{}, err
	}
	return r.GetVersion(ctx, strings.TrimSpace(versionID))
}

func (r *VersionRepo) nextID(prefix string) string {
	n := r.seq.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, r.now().UTC().UnixNano(), n)
}
