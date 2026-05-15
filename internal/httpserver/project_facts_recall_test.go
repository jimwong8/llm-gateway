package httpserver

import (
	"testing"
	"time"

	"llm-gateway/gateway/internal/memory"
)

func TestRerankProjectFactsForRecallFiltersNonActiveAndStaleFacts(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	facts := []memory.ProjectFact{
		{
			Key:            "pg_truth",
			Value:          "PG is Truth (fresh)",
			Status:         "active",
			SourceText:     "最终决定：PG is Truth",
			LastVerifiedAt: now.Add(-24 * time.Hour),
			UpdatedAt:      now.Add(-24 * time.Hour),
		},
		{
			Key:            "redis_role_old",
			Value:          "Redis 只做热层 (stale)",
			Status:         "active",
			SourceText:     "最终决定：Redis 只做热层",
			LastVerifiedAt: now.Add(-220 * 24 * time.Hour),
			UpdatedAt:      now.Add(-200 * 24 * time.Hour),
		},
		{
			Key:            "pg_truth",
			Value:          "PG is Truth (superseded)",
			Status:         "superseded",
			SourceText:     "最终决定：PG is Truth",
			LastVerifiedAt: now.Add(-2 * time.Hour),
			UpdatedAt:      now.Add(-2 * time.Hour),
		},
		{
			Key:            "oracle_review_mode",
			Value:          "Oracle 审查默认拆小并行",
			Status:         "active",
			SourceText:     "候选方案：Oracle 审查默认拆小并行",
			LastVerifiedAt: now.Add(-3 * time.Hour),
			UpdatedAt:      now.Add(-3 * time.Hour),
		},
	}

	ranked := rerankProjectFactsForRecall(facts, now)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 active facts after filtering non-active ones, got %d (%#v)", len(ranked), ranked)
	}
	for _, fact := range ranked {
		if fact.Status != "" && fact.Status != "active" {
			t.Fatalf("expected only active facts in ranked result, got status=%q fact=%#v", fact.Status, fact)
		}
	}

	if ranked[0].Key != "pg_truth" || ranked[0].Value != "PG is Truth (fresh)" {
		t.Fatalf("expected fresh confirmed fact ranked first, got %s=%s", ranked[0].Key, ranked[0].Value)
	}
	if ranked[len(ranked)-1].Key != "redis_role_old" {
		t.Fatalf("expected stale fact ranked last due to expiration penalty, got %s=%s", ranked[len(ranked)-1].Key, ranked[len(ranked)-1].Value)
	}
}

func TestProjectFactRecallScorePenalizesExpiredFact(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	fresh := memory.ProjectFact{
		Key:            "pg_truth",
		Value:          "PG is Truth",
		Status:         "active",
		SourceText:     "最终决定：PG is Truth",
		LastVerifiedAt: now.Add(-3 * 24 * time.Hour),
		UpdatedAt:      now.Add(-2 * 24 * time.Hour),
	}
	stale := memory.ProjectFact{
		Key:            "pg_truth",
		Value:          "PG is Truth (old)",
		Status:         "active",
		SourceText:     "最终决定：PG is Truth",
		LastVerifiedAt: now.Add(-300 * 24 * time.Hour),
		UpdatedAt:      now.Add(-240 * 24 * time.Hour),
	}

	freshScore := projectFactRecallScore(fresh, now)
	staleScore := projectFactRecallScore(stale, now)
	if !(freshScore > staleScore) {
		t.Fatalf("expected fresh fact score > stale fact score, got fresh=%d stale=%d", freshScore, staleScore)
	}
}
