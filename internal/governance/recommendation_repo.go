package governance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var errNoSuccessfulEvaluationRun = errors.New("no successful evaluation run found")

// EvaluationResultRecord 表示单条可用于推荐计算的评估结果记录。
type EvaluationResultRecord struct {
	RunID      string
	Model      string
	FinalScore float64
	Breakdown  ScoreBreakdown
}

// RecommendationRepo 负责从评估结果生成推荐所需的数据访问与推荐落库。
type RecommendationRepo struct {
	db *sql.DB
}

func NewRecommendationRepo(store *Store) *RecommendationRepo {
	if store == nil {
		return nil
	}
	return &RecommendationRepo{db: store.db}
}

func (r *RecommendationRepo) LoadLatestSuccessfulEvaluationResults(ctx context.Context, agentID, taskType, environment string) ([]EvaluationResultRecord, string, error) {
	if r == nil || r.db == nil {
		return nil, "", errors.New("recommendation repo is nil")
	}
	agentID = strings.TrimSpace(agentID)
	taskType = strings.TrimSpace(taskType)
	environment = strings.TrimSpace(environment)
	if agentID == "" || taskType == "" || environment == "" {
		return nil, "", errors.New("agent_id, task_type and environment are required")
	}

	var runID string
	err := r.db.QueryRowContext(ctx, `
SELECT run_id
FROM evaluation_runs
WHERE agent_id = $1 AND task_type = $2 AND environment = $3 AND status = 'succeeded'
ORDER BY COALESCE(completed_at, created_at) DESC, id DESC
LIMIT 1
`, agentID, taskType, environment).Scan(&runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", errNoSuccessfulEvaluationRun
		}
		return nil, "", err
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT model, final_score, metrics
FROM evaluation_results
WHERE run_id = $1
ORDER BY final_score DESC, id ASC
`, runID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := make([]EvaluationResultRecord, 0, 8)
	for rows.Next() {
		var model string
		var finalScore float64
		var metricsRaw []byte
		if err := rows.Scan(&model, &finalScore, &metricsRaw); err != nil {
			return nil, "", err
		}
		record := EvaluationResultRecord{
			RunID:      runID,
			Model:      strings.TrimSpace(model),
			FinalScore: finalScore,
		}
		if len(metricsRaw) > 0 {
			record.Breakdown = parseBreakdownJSON(metricsRaw)
		}
		if record.Model == "" {
			continue
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) == 0 {
		return nil, "", fmt.Errorf("no evaluation results found for run %s", runID)
	}
	return out, runID, nil
}

func parseBreakdownJSON(raw []byte) ScoreBreakdown {
	var source map[string]float64
	if err := json.Unmarshal(raw, &source); err != nil {
		return ScoreBreakdown{}
	}
	return ScoreBreakdown{
		Quality:      source["quality"],
		Cost:         source["cost"],
		Latency:      source["latency"],
		Safety:       source["safety"],
		Availability: source["availability"],
	}
}

func (r *RecommendationRepo) SaveRecommendation(ctx context.Context, rec Recommendation) (Recommendation, error) {
	if r == nil || r.db == nil {
		return Recommendation{}, errors.New("recommendation repo is nil")
	}

	if strings.TrimSpace(rec.ID) == "" {
		rec.ID = fmt.Sprintf("rec-%d", time.Now().UTC().UnixNano())
	}
	now := time.Now().UTC()
	if rec.GeneratedAt.IsZero() {
		rec.GeneratedAt = now
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = rec.GeneratedAt
	}
	rec.UpdatedAt = now

	candidatesJSON, err := json.Marshal(rec.Candidates)
	if err != nil {
		return Recommendation{}, err
	}
	breakdownJSON, err := json.Marshal(rec.ScoreBreakdown)
	if err != nil {
		return Recommendation{}, err
	}

	status := strings.TrimSpace(string(rec.Status))
	if status == "" {
		status = "pending"
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO model_recommendations (
    recommendation_id,
    agent_id,
    task_type,
    environment,
    recommended_model,
    candidates,
    score_breakdown,
    approval_required,
    status,
    created_by,
    created_at,
    updated_at
) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,$9,'system',$10,$11)
`,
		rec.ID,
		rec.AgentID,
		rec.TaskType,
		rec.Environment,
		rec.RecommendedModel,
		string(candidatesJSON),
		string(breakdownJSON),
		rec.ApprovalRequired,
		status,
		rec.CreatedAt,
		rec.UpdatedAt,
	)
	if err != nil {
		return Recommendation{}, err
	}
	return rec, nil
}

func sortRecordsByScore(records []EvaluationResultRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].FinalScore == records[j].FinalScore {
			return records[i].Model < records[j].Model
		}
		return records[i].FinalScore > records[j].FinalScore
	})
}
