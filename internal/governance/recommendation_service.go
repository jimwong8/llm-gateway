package governance

import (
	"context"
	"errors"
	"strings"
	"time"
)

// GenerateRecommendationInput 是推荐生成输入。
type GenerateRecommendationInput struct {
	TenantID     string
	AgentID      string
	TaskType     string
	Environment  string
	RequestedBy  string
	Summary      string
}

// RecommendationService 执行最小推荐流水线：读取最新成功评估、排序并持久化推荐。
type RecommendationService struct {
	repo recommendationDataStore
}

type recommendationDataStore interface {
	LoadLatestSuccessfulEvaluationResults(ctx context.Context, agentID, taskType, environment string) ([]EvaluationResultRecord, string, error)
	SaveRecommendation(ctx context.Context, rec Recommendation) (Recommendation, error)
}

func NewRecommendationService(repo recommendationDataStore) *RecommendationService {
	return &RecommendationService{repo: repo}
}

func (s *RecommendationService) Generate(ctx context.Context, input GenerateRecommendationInput) (Recommendation, error) {
	if s == nil || s.repo == nil {
		return Recommendation{}, errors.New("recommendation service is not initialized")
	}
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.TaskType = strings.TrimSpace(input.TaskType)
	input.Environment = strings.TrimSpace(input.Environment)
	if input.AgentID == "" || input.TaskType == "" || input.Environment == "" {
		return Recommendation{}, errors.New("agent_id, task_type and environment are required")
	}

	records, runID, err := s.repo.LoadLatestSuccessfulEvaluationResults(ctx, input.AgentID, input.TaskType, input.Environment)
	if err != nil {
		return Recommendation{}, err
	}
	sortRecordsByScore(records)
	if len(records) == 0 {
		return Recommendation{}, errNoSuccessfulEvaluationRun
	}

	candidates := make([]CandidateModel, 0, len(records))
	for idx, record := range records {
		candidates = append(candidates, CandidateModel{
			ModelID:   record.Model,
			Rank:      idx + 1,
			Composite: record.FinalScore,
			Breakdown: record.Breakdown,
		})
	}

	now := time.Now().UTC()
	rec := Recommendation{
		TenantID:         strings.TrimSpace(input.TenantID),
		Environment:      input.Environment,
		AgentID:          input.AgentID,
		TaskType:         input.TaskType,
		Status:           RecommendationStatusDraft,
		RecommendedModel: candidates[0].ModelID,
		ScoreBreakdown:   candidates[0].Breakdown,
		Candidates:       candidates,
		ApprovalRequired: true,
		SourceRunID:      runID,
		Summary:          strings.TrimSpace(input.Summary),
		GeneratedAt:      now,
	}
	if rec.Summary == "" {
		rec.Summary = "generated from latest successful evaluation run"
	}

	return s.repo.SaveRecommendation(ctx, rec)
}
