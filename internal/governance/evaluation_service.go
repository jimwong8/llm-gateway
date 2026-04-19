package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type EvaluationService struct {
	repo *EvaluationRepo
}

func NewEvaluationService(store *Store) *EvaluationService {
	return &EvaluationService{repo: NewEvaluationRepo(store)}
}

func NewEvaluationServiceWithRepo(repo *EvaluationRepo) *EvaluationService {
	return &EvaluationService{repo: repo}
}

func (s *EvaluationService) CreateDataset(ctx context.Context, input EvaluationDatasetInput) (string, error) {
	if s == nil || s.repo == nil {
		return "", errors.New("evaluation service is not initialized")
	}
	if strings.TrimSpace(input.Name) == "" {
		return "", fmt.Errorf("dataset name is required")
	}
	if strings.TrimSpace(input.Version) == "" {
		return "", fmt.Errorf("dataset version is required")
	}
	if strings.TrimSpace(input.TaskType) == "" {
		return "", fmt.Errorf("dataset task_type is required")
	}
	dataset, err := s.repo.CreateDataset(ctx, input)
	if err != nil {
		return "", err
	}
	return dataset.ID, nil
}

func (s *EvaluationService) CreateFormula(ctx context.Context, input ScoringFormulaInput) (string, error) {
	if s == nil || s.repo == nil {
		return "", errors.New("evaluation service is not initialized")
	}
	if strings.TrimSpace(input.Version) == "" {
		return "", fmt.Errorf("formula version is required")
	}
	if len(input.FormulaJSON) == 0 {
		return "", fmt.Errorf("formula_json is required")
	}
	formula, err := s.repo.CreateFormula(ctx, input)
	if err != nil {
		return "", err
	}
	return formula.ID, nil
}

func (s *EvaluationService) StartRun(ctx context.Context, input StartEvaluationRunInput) (EvaluationRun, error) {
	if s == nil || s.repo == nil {
		return EvaluationRun{}, errors.New("evaluation service is not initialized")
	}
	if strings.TrimSpace(input.DatasetID) == "" {
		return EvaluationRun{}, fmt.Errorf("dataset_id is required")
	}
	if strings.TrimSpace(input.AgentID) == "" {
		return EvaluationRun{}, fmt.Errorf("agent_id is required")
	}
	if strings.TrimSpace(input.TaskType) == "" {
		return EvaluationRun{}, fmt.Errorf("task_type is required")
	}
	if strings.TrimSpace(input.Environment) == "" {
		return EvaluationRun{}, fmt.Errorf("environment is required")
	}

	dataset, err := s.repo.GetDataset(ctx, input.DatasetID)
	if err != nil {
		return EvaluationRun{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(dataset.TaskType), strings.TrimSpace(input.TaskType)) {
		return EvaluationRun{}, fmt.Errorf("dataset task_type mismatch: dataset=%s input=%s", dataset.TaskType, input.TaskType)
	}

	if strings.TrimSpace(input.FormulaVersionID) != "" {
		if _, err := s.repo.GetFormula(ctx, input.FormulaVersionID); err != nil {
			return EvaluationRun{}, err
		}
	}

	return s.repo.CreateRun(ctx, input, EvaluationRunStatusRunning)
}

func (s *EvaluationService) UpdateRunStatus(ctx context.Context, runID string, status EvaluationRunStatus) (EvaluationRun, error) {
	if s == nil || s.repo == nil {
		return EvaluationRun{}, errors.New("evaluation service is not initialized")
	}
	if strings.TrimSpace(runID) == "" {
		return EvaluationRun{}, fmt.Errorf("run_id is required")
	}
	if !status.Valid() {
		return EvaluationRun{}, fmt.Errorf("invalid run status: %s", status)
	}
	return s.repo.UpdateRunStatus(ctx, runID, status)
}
