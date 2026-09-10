package governance

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// RolloutDashboardRepo 负责查询 dashboard 展示所需的 rollout 基础行。
type RolloutDashboardRepo struct {
	db *sql.DB
}

func NewRolloutDashboardRepo(store *Store) *RolloutDashboardRepo {
	if store == nil {
		return nil
	}
	return &RolloutDashboardRepo{db: store.DB()}
}

func (r *RolloutDashboardRepo) ListRecentRollouts(ctx context.Context, limit int) ([]RolloutDashboardRollout, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("rollout dashboard repo is not initialized")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT rollout_id,
       policy_version_id,
       environment,
       rollout_percent,
       status
FROM model_rollouts
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RolloutDashboardRollout, 0, limit)
	for rows.Next() {
		var item RolloutDashboardRollout
		if err := rows.Scan(&item.RolloutID, &item.PolicyVersionID, &item.Environment, &item.RolloutPercent, &item.Status); err != nil {
			return nil, err
		}
		item.RolloutID = strings.TrimSpace(item.RolloutID)
		item.PolicyVersionID = strings.TrimSpace(item.PolicyVersionID)
		item.Environment = strings.TrimSpace(item.Environment)
		item.Status = strings.TrimSpace(item.Status)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
