package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type rolloutMetricsAggregator interface {
	Aggregate(ctx context.Context, query RolloutMetricsQuery) (RolloutMetricsSnapshot, error)
}

// RolloutMetricsService 负责 rollout 指标聚合与 guard verdict 判定。
type RolloutMetricsService struct {
	repo rolloutMetricsAggregator
}

func NewRolloutMetricsService(store *Store) *RolloutMetricsService {
	if store == nil {
		return &RolloutMetricsService{}
	}
	return &RolloutMetricsService{repo: NewRolloutMetricsRepo(store)}
}

func NewRolloutMetricsServiceWithRepo(repo rolloutMetricsAggregator) *RolloutMetricsService {
	return &RolloutMetricsService{repo: repo}
}

func (s *RolloutMetricsService) Aggregate(ctx context.Context, query RolloutMetricsQuery) (RolloutMetricsSnapshot, error) {
	if s == nil || s.repo == nil {
		return RolloutMetricsSnapshot{}, errors.New("rollout metrics service is not initialized")
	}
	query.RolloutID = strings.TrimSpace(query.RolloutID)
	query.PolicyVersionID = strings.TrimSpace(query.PolicyVersionID)
	return s.repo.Aggregate(ctx, query)
}

func (s *RolloutMetricsService) EvaluateGuards(ctx context.Context, query RolloutMetricsQuery, thresholds RolloutGuardThresholds) (RolloutGuardResult, error) {
	metrics, err := s.Aggregate(ctx, query)
	if err != nil {
		return RolloutGuardResult{}, err
	}
	result := RolloutGuardResult{Verdict: RolloutGuardKeep, Metrics: metrics, Summary: "metrics within guardrails"}
	if thresholds.MinRequests > 0 && metrics.RequestsTotal < thresholds.MinRequests {
		result.Verdict = RolloutGuardPause
		result.Summary = fmt.Sprintf("insufficient sample size: requests=%d < %d", metrics.RequestsTotal, thresholds.MinRequests)
		return result, nil
	}
	if thresholds.RollbackRequiredErrorRateGTE > 0 && metrics.ErrorRate >= thresholds.RollbackRequiredErrorRateGTE {
		result.Verdict = RolloutGuardRollbackRequired
		result.Summary = fmt.Sprintf("error_rate %.4f >= rollback_required %.4f", metrics.ErrorRate, thresholds.RollbackRequiredErrorRateGTE)
		return result, nil
	}
	if thresholds.RollbackRequiredP95LatencyGTE > 0 && metrics.P95LatencyMillis >= thresholds.RollbackRequiredP95LatencyGTE {
		result.Verdict = RolloutGuardRollbackRequired
		result.Summary = fmt.Sprintf("p95_latency %d >= rollback_required %d", metrics.P95LatencyMillis, thresholds.RollbackRequiredP95LatencyGTE)
		return result, nil
	}
	if thresholds.RollbackSuggestedErrorRateGTE > 0 && metrics.ErrorRate >= thresholds.RollbackSuggestedErrorRateGTE {
		result.Verdict = RolloutGuardRollbackSuggested
		result.Summary = fmt.Sprintf("error_rate %.4f >= rollback_suggested %.4f", metrics.ErrorRate, thresholds.RollbackSuggestedErrorRateGTE)
		return result, nil
	}
	if thresholds.RollbackSuggestedP95LatencyGTE > 0 && metrics.P95LatencyMillis >= thresholds.RollbackSuggestedP95LatencyGTE {
		result.Verdict = RolloutGuardRollbackSuggested
		result.Summary = fmt.Sprintf("p95_latency %d >= rollback_suggested %d", metrics.P95LatencyMillis, thresholds.RollbackSuggestedP95LatencyGTE)
		return result, nil
	}
	if thresholds.RollbackRequiredFallbackRateGTE > 0 && metrics.FallbackRate >= thresholds.RollbackRequiredFallbackRateGTE {
		result.Verdict = RolloutGuardRollbackRequired
		result.Summary = fmt.Sprintf("fallback_rate %.4f >= rollback_required %.4f", metrics.FallbackRate, thresholds.RollbackRequiredFallbackRateGTE)
		return result, nil
	}
	if thresholds.RollbackSuggestedFallbackRateGTE > 0 && metrics.FallbackRate >= thresholds.RollbackSuggestedFallbackRateGTE {
		result.Verdict = RolloutGuardRollbackSuggested
		result.Summary = fmt.Sprintf("fallback_rate %.4f >= rollback_suggested %.4f", metrics.FallbackRate, thresholds.RollbackSuggestedFallbackRateGTE)
		return result, nil
	}
	if thresholds.PauseFallbackRateGTE > 0 && metrics.FallbackRate >= thresholds.PauseFallbackRateGTE {
		result.Verdict = RolloutGuardPause
		result.Summary = fmt.Sprintf("fallback_rate %.4f >= pause %.4f", metrics.FallbackRate, thresholds.PauseFallbackRateGTE)
		return result, nil
	}
	if thresholds.PauseErrorRateGTE > 0 && metrics.ErrorRate >= thresholds.PauseErrorRateGTE {
		result.Verdict = RolloutGuardPause
		result.Summary = fmt.Sprintf("error_rate %.4f >= pause %.4f", metrics.ErrorRate, thresholds.PauseErrorRateGTE)
		return result, nil
	}
	if thresholds.PauseP95LatencyMillisGTE > 0 && metrics.P95LatencyMillis >= thresholds.PauseP95LatencyMillisGTE {
		result.Verdict = RolloutGuardPause
		result.Summary = fmt.Sprintf("p95_latency %d >= pause %d", metrics.P95LatencyMillis, thresholds.PauseP95LatencyMillisGTE)
		return result, nil
	}
	return result, nil
}
