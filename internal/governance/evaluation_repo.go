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
	ErrEvaluationDatasetNotFound = errors.New("evaluation dataset not found")
	ErrScoringFormulaNotFound    = errors.New("scoring formula not found")
	ErrEvaluationRunNotFound     = errors.New("evaluation run not found")
)

type EvaluationRepo struct {
	db  *sql.DB
	now func() time.Time
	seq int64
}

func NewEvaluationRepo(store *Store) *EvaluationRepo {
	return &EvaluationRepo{
		db:  store.DB(),
		now: time.Now,
	}
}

func (r *EvaluationRepo) CreateDataset(ctx context.Context, input EvaluationDatasetInput) (EvaluationDataset, error) {
	id := r.nextID("dataset")
	meta := map[string]string{}
	if v := strings.TrimSpace(input.Description); v != "" {
		meta["description"] = v
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return EvaluationDataset{}, err
	}
	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = "system"
	}
	createdAt := r.now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO evaluation_datasets
		(dataset_id, name, version, task_type, metadata, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
	`, id, strings.TrimSpace(input.Name), strings.TrimSpace(input.Version), strings.TrimSpace(input.TaskType), string(metaJSON), createdBy, createdAt)
	if err != nil {
		return EvaluationDataset{}, err
	}
	return EvaluationDataset{
		ID:          id,
		Name:        strings.TrimSpace(input.Name),
		Version:     strings.TrimSpace(input.Version),
		TaskType:    strings.TrimSpace(input.TaskType),
		Description: strings.TrimSpace(input.Description),
		CreatedAt:   createdAt,
	}, nil
}

func (r *EvaluationRepo) GetDataset(ctx context.Context, datasetID string) (EvaluationDataset, error) {
	var (
		id        string
		name      string
		version   string
		taskType  string
		metaRaw   []byte
		createdAt time.Time
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT dataset_id, name, version, task_type, metadata, created_at
		FROM evaluation_datasets
		WHERE dataset_id = $1
	`, strings.TrimSpace(datasetID)).Scan(&id, &name, &version, &taskType, &metaRaw, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EvaluationDataset{}, ErrEvaluationDatasetNotFound
		}
		return EvaluationDataset{}, err
	}
	meta := map[string]string{}
	_ = json.Unmarshal(metaRaw, &meta)
	return EvaluationDataset{
		ID:          id,
		Name:        name,
		Version:     version,
		TaskType:    taskType,
		Description: strings.TrimSpace(meta["description"]),
		CreatedAt:   createdAt.UTC(),
	}, nil
}

func (r *EvaluationRepo) CreateFormula(ctx context.Context, input ScoringFormulaInput) (ScoringFormula, error) {
	id := r.nextID("formula")
	formulaJSON := input.FormulaJSON
	if len(formulaJSON) == 0 {
		formulaJSON = []byte(`{}`)
	}
	if !json.Valid(formulaJSON) {
		return ScoringFormula{}, fmt.Errorf("formula_json must be valid json")
	}
	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = "system"
	}
	createdAt := r.now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO evaluation_scoring_formulas
		(formula_id, version, formula_json, created_by, created_at)
		VALUES ($1, $2, $3::jsonb, $4, $5)
	`, id, strings.TrimSpace(input.Version), string(formulaJSON), createdBy, createdAt)
	if err != nil {
		return ScoringFormula{}, err
	}
	return ScoringFormula{
		ID:          id,
		Version:     strings.TrimSpace(input.Version),
		FormulaJSON: append([]byte(nil), formulaJSON...),
		CreatedAt:   createdAt,
	}, nil
}

func (r *EvaluationRepo) GetFormula(ctx context.Context, formulaID string) (ScoringFormula, error) {
	var (
		id        string
		version   string
		rawJSON   []byte
		createdAt time.Time
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT formula_id, version, formula_json, created_at
		FROM evaluation_scoring_formulas
		WHERE formula_id = $1
	`, strings.TrimSpace(formulaID)).Scan(&id, &version, &rawJSON, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ScoringFormula{}, ErrScoringFormulaNotFound
		}
		return ScoringFormula{}, err
	}
	return ScoringFormula{
		ID:          id,
		Version:     version,
		FormulaJSON: append([]byte(nil), rawJSON...),
		CreatedAt:   createdAt.UTC(),
	}, nil
}

func (r *EvaluationRepo) CreateRun(ctx context.Context, input StartEvaluationRunInput, status EvaluationRunStatus) (EvaluationRun, error) {
	id := r.nextID("run")
	createdAt := r.now().UTC()
	startedAt := createdAt
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO evaluation_runs
		(run_id, dataset_id, formula_id, agent_id, task_type, environment, status, started_at, created_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9)
	`, id, strings.TrimSpace(input.DatasetID), strings.TrimSpace(input.FormulaVersionID), strings.TrimSpace(input.AgentID), strings.TrimSpace(input.TaskType), strings.TrimSpace(input.Environment), string(status), startedAt, createdAt)
	if err != nil {
		return EvaluationRun{}, err
	}
	return EvaluationRun{
		ID:               id,
		DatasetID:        strings.TrimSpace(input.DatasetID),
		AgentID:          strings.TrimSpace(input.AgentID),
		TaskType:         strings.TrimSpace(input.TaskType),
		Environment:      strings.TrimSpace(input.Environment),
		FormulaVersionID: strings.TrimSpace(input.FormulaVersionID),
		Status:           status,
		StartedAt:        startedAt,
		CreatedAt:        createdAt,
	}, nil
}

func (r *EvaluationRepo) GetRun(ctx context.Context, runID string) (EvaluationRun, error) {
	var (
		run       EvaluationRun
		formulaID sql.NullString
		startedAt sql.NullTime
		endedAt   sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT run_id, dataset_id, formula_id, agent_id, task_type, environment, status, started_at, completed_at, created_at
		FROM evaluation_runs
		WHERE run_id = $1
	`, strings.TrimSpace(runID)).Scan(
		&run.ID,
		&run.DatasetID,
		&formulaID,
		&run.AgentID,
		&run.TaskType,
		&run.Environment,
		&run.Status,
		&startedAt,
		&endedAt,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EvaluationRun{}, ErrEvaluationRunNotFound
		}
		return EvaluationRun{}, err
	}
	if formulaID.Valid {
		run.FormulaVersionID = formulaID.String
	}
	if startedAt.Valid {
		run.StartedAt = startedAt.Time.UTC()
	}
	if endedAt.Valid {
		run.FinishedAt = endedAt.Time.UTC()
	}
	run.CreatedAt = run.CreatedAt.UTC()
	return run, nil
}

func (r *EvaluationRepo) UpdateRunStatus(ctx context.Context, runID string, status EvaluationRunStatus) (EvaluationRun, error) {
	now := r.now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE evaluation_runs
		SET status = $2,
		    completed_at = CASE WHEN $2 IN ('succeeded','failed','canceled') THEN $3 ELSE completed_at END
		WHERE run_id = $1
	`, strings.TrimSpace(runID), string(status), now)
	if err != nil {
		return EvaluationRun{}, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return EvaluationRun{}, err
	}
	if rows == 0 {
		return EvaluationRun{}, ErrEvaluationRunNotFound
	}
	return r.GetRun(ctx, runID)
}

func (r *EvaluationRepo) nextID(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d_%d", prefix, r.now().UTC().UnixNano(), r.seq)
}
