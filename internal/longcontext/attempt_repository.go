package longcontext

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Attempt struct {
	TaskID string
	ChunkID string
	Phase string
	Model string
	Provider string
	RequestID string
	Status string
	InputTokens int
	OutputTokens int
	EstimatedCost float64
	LatencyMs int
	ErrorType string
	ErrorMessage string
}

type AttemptRepository interface {
	RecordAttempt(context.Context, Attempt) error
	MarkChunkFailed(context.Context, string, string, string) error
}

func (r *PostgresRepository) RecordAttempt(ctx context.Context, a Attempt) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO long_context_attempts (task_id,chunk_id,phase,model,provider,request_id,status,input_tokens,output_tokens,estimated_cost,latency_ms,error_type,error_message) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, a.TaskID,a.ChunkID,a.Phase,a.Model,a.Provider,a.RequestID,a.Status,a.InputTokens,a.OutputTokens,a.EstimatedCost,a.LatencyMs,a.ErrorType,a.ErrorMessage)
	return err
}

func (r *PostgresRepository) MarkChunkFailed(ctx context.Context, chunkID, taskID, message string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE long_context_chunks SET status='failed',updated_at=NOW() WHERE id=$1 AND task_id=$2 AND status='mapping'`, chunkID, taskID)
	if err != nil { return err }
	n, _ := res.RowsAffected(); if n != 1 { return fmt.Errorf("chunk failure compare-and-swap failed") }
	_, err = r.db.ExecContext(ctx, `UPDATE long_context_tasks SET last_error=$1,updated_at=NOW() WHERE id=$2`, message, taskID)
	return err
}

func (r *PostgresRepository) AddCompletedChunk(ctx context.Context, taskID string, inputTokens, outputTokens int, cost float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE long_context_tasks SET completed_chunks=completed_chunks+1,used_tokens=used_tokens+$1,estimated_cost=estimated_cost+$2,updated_at=NOW() WHERE id=$3`, inputTokens+outputTokens,cost,taskID)
	return err
}

func (r *PostgresRepository) GetAttemptCount(ctx context.Context, taskID string) (int, error) {
	var n int; err:=r.db.QueryRowContext(ctx,`SELECT count(*) FROM long_context_attempts WHERE task_id=$1`,taskID).Scan(&n);return n,err
}

var _ = sql.ErrNoRows
var _ = time.Second
