package governance

import (
	"context"
	"errors"
	"strings"
)

var ErrRolloutDashboardServiceNotInitialized = errors.New("rollout dashboard service is not initialized")

type rolloutDashboardRolloutLister interface {
	ListRecentRollouts(ctx context.Context, limit int) ([]RolloutDashboardRollout, error)
}

type rolloutDashboardMetricsAggregator interface {
	Aggregate(ctx context.Context, query RolloutMetricsQuery) (RolloutMetricsSnapshot, error)
}

// RolloutDashboardService 聚合 rollout 列表与运行时指标，供管理台 dashboard 使用。
type RolloutDashboardService struct {
	rollouts rolloutDashboardRolloutLister
	metrics  rolloutDashboardMetricsAggregator
}

func NewRolloutDashboardService(store *Store) *RolloutDashboardService {
	if store == nil {
		return &RolloutDashboardService{}
	}
	return &RolloutDashboardService{
		rollouts: NewRolloutDashboardRepo(store),
		metrics:  NewRolloutMetricsService(store),
	}
}

func NewRolloutDashboardServiceWithRepoAndMetrics(repo rolloutDashboardRolloutLister, metrics rolloutDashboardMetricsAggregator) *RolloutDashboardService {
	return &RolloutDashboardService{rollouts: repo, metrics: metrics}
}

func (s *RolloutDashboardService) ListRows(ctx context.Context, query RolloutDashboardQuery) ([]RolloutDashboardRow, error) {
	if s == nil || s.rollouts == nil || s.metrics == nil {
		return nil, ErrRolloutDashboardServiceNotInitialized
	}
	if query.Limit <= 0 {
		query.Limit = 20
	}

	rollouts, err := s.rollouts.ListRecentRollouts(ctx, query.Limit)
	if err != nil {
		return nil, err
	}
	rows := make([]RolloutDashboardRow, 0, len(rollouts))
	for _, rollout := range rollouts {
		metrics, err := s.metrics.Aggregate(ctx, RolloutMetricsQuery{
			RolloutID:       strings.TrimSpace(rollout.RolloutID),
			PolicyVersionID: strings.TrimSpace(rollout.PolicyVersionID),
		})
		if err != nil {
			return nil, err
		}
		rows = append(rows, RolloutDashboardRow{
			RolloutID:       rollout.RolloutID,
			PolicyVersionID: rollout.PolicyVersionID,
			Environment:     rollout.Environment,
			Percent:         rollout.RolloutPercent,
			Status:          rollout.Status,
			ErrorRate:       metrics.ErrorRate,
			P95Latency:      metrics.P95LatencyMillis,
			FallbackRate:    metrics.FallbackRate,
			SampleCount:     metrics.RequestsTotal,
		})
	}
	return rows, nil
}
