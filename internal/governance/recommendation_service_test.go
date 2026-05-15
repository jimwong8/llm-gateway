package governance

import (
	"context"
	"errors"
	"testing"
)

type recommendationRepoStub struct {
	resultsByKey map[string][]EvaluationResultRecord
	runByKey     map[string]string
	saved        []Recommendation
}

func newRecommendationRepoStub() *recommendationRepoStub {
	return &recommendationRepoStub{
		resultsByKey: map[string][]EvaluationResultRecord{},
		runByKey:     map[string]string{},
		saved:        nil,
	}
}

func (r *recommendationRepoStub) key(agentID, taskType, environment string) string {
	return agentID + "|" + taskType + "|" + environment
}

func (r *recommendationRepoStub) seed(agentID, taskType, environment, runID string, results []EvaluationResultRecord) {
	k := r.key(agentID, taskType, environment)
	r.resultsByKey[k] = results
	r.runByKey[k] = runID
}

func (r *recommendationRepoStub) LoadLatestSuccessfulEvaluationResults(_ context.Context, agentID, taskType, environment string) ([]EvaluationResultRecord, string, error) {
	k := r.key(agentID, taskType, environment)
	results, ok := r.resultsByKey[k]
	if !ok || len(results) == 0 {
		return nil, "", errNoSuccessfulEvaluationRun
	}
	copied := make([]EvaluationResultRecord, len(results))
	copy(copied, results)
	return copied, r.runByKey[k], nil
}

func (r *recommendationRepoStub) SaveRecommendation(_ context.Context, rec Recommendation) (Recommendation, error) {
	r.saved = append(r.saved, rec)
	return rec, nil
}

func TestGenerateRecommendationPicksBestScoringModel(t *testing.T) {
	repo := newRecommendationRepoStub()
	repo.seed("code-reviewer", "code_review", "prod", "run-new-success", []EvaluationResultRecord{
		{
			RunID:      "run-new-success",
			Model:      "model-a",
			FinalScore: 0.88,
			Breakdown:  ScoreBreakdown{Quality: 0.9, Cost: 0.6, Latency: 0.7, Safety: 0.85},
		},
		{
			RunID:      "run-new-success",
			Model:      "model-b",
			FinalScore: 0.93,
			Breakdown:  ScoreBreakdown{Quality: 0.95, Cost: 0.7, Latency: 0.8, Safety: 0.92},
		},
	})
	svc := NewRecommendationService(repo)

	rec, err := svc.Generate(context.Background(), GenerateRecommendationInput{
		AgentID:     "code-reviewer",
		TaskType:    "code_review",
		Environment: "prod",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if rec.RecommendedModel != "model-b" {
		t.Fatalf("expected model-b, got %s", rec.RecommendedModel)
	}
	if rec.SourceRunID != "run-new-success" {
		t.Fatalf("expected source run run-new-success, got %s", rec.SourceRunID)
	}
	if !rec.ApprovalRequired {
		t.Fatalf("expected approval_required=true")
	}
	if rec.Status != RecommendationStatusDraft {
		t.Fatalf("expected status draft, got %s", rec.Status)
	}
	if len(rec.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(rec.Candidates))
	}
	if rec.Candidates[0].ModelID != "model-b" || rec.Candidates[0].Rank != 1 {
		t.Fatalf("unexpected top candidate: %+v", rec.Candidates[0])
	}
	if rec.Candidates[0].Breakdown.Quality != 0.95 {
		t.Fatalf("expected top breakdown quality 0.95, got %f", rec.Candidates[0].Breakdown.Quality)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected one persisted recommendation, got %d", len(repo.saved))
	}
}

func TestGenerateRecommendationReturnsErrorWhenNoSuccessfulRun(t *testing.T) {
	repo := newRecommendationRepoStub()
	svc := NewRecommendationService(repo)

	_, err := svc.Generate(context.Background(), GenerateRecommendationInput{
		AgentID:     "code-reviewer",
		TaskType:    "code_review",
		Environment: "prod",
	})
	if !errors.Is(err, errNoSuccessfulEvaluationRun) {
		t.Fatalf("expected errNoSuccessfulEvaluationRun, got %v", err)
	}
}
