package longcontext

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Repository interface {
	CreateTask(context.Context, CreateTaskInput) (*Task, bool, error)
	GetTask(context.Context, string, string) (*Task, error)
	ListTasks(context.Context, string, int, int) ([]*Task, int, error)
	CancelTask(context.Context, string, string) error
	TransitionTask(context.Context, string, Status, Status, Phase) error
	ClaimTask(context.Context, string, time.Duration) (*Task, error)
	RenewLease(context.Context, string, string, time.Duration) error
	ReleaseLease(context.Context, string, string) error
	PersistChunks(context.Context, string, string, string, int, int) error
}

type PostgresRepository struct{ db *sql.DB }

func (r *PostgresRepository) HealthCheck(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) CreateTask(ctx context.Context, in CreateTaskInput) (*Task, bool, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, false, fmt.Errorf("task id is required")
	}
	row := r.db.QueryRowContext(ctx, `INSERT INTO long_context_tasks (id,tenant_id,user_id,session_id,idempotency_key,model,mode,query,input_sha256,input_tokens,worker_count,retrieval_top_k,final_context_budget,max_cost,status,phase,lease_owner,lease_until) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) ON CONFLICT DO NOTHING RETURNING id`, in.ID, in.TenantID, in.UserID, in.SessionID, in.IdempotencyKey, in.Model, in.Mode, in.Query, in.InputSHA256, in.InputTokens, in.WorkerCount, in.RetrievalTopK, in.FinalBudget, in.MaxCost, StatusQueued, PhaseQueued, leaseOwner(in.LeaseOwner, in.LeaseDuration), leaseUntil(in.LeaseOwner, in.LeaseDuration))
	var id string
	err := row.Scan(&id)
	if err == sql.ErrNoRows && in.IdempotencyKey != "" {
		t, e := r.findByIdempotency(ctx, in.TenantID, in.IdempotencyKey)
		if e != nil {
			return nil, false, e
		}
		if t.InputSHA256 != in.InputSHA256 {
			return nil, false, fmt.Errorf("idempotency key conflicts with existing input")
		}
		return t, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	t, err := r.scanTask(r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM long_context_tasks WHERE id=$1 AND tenant_id=$2`, id, in.TenantID))
	RecordTaskCreated()
	return t, false, err
}
func (r *PostgresRepository) findByIdempotency(ctx context.Context, tenant, key string) (*Task, error) {
	return r.scanTask(r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM long_context_tasks WHERE tenant_id=$1 AND idempotency_key=$2`, tenant, key))
}

const taskColumns = `id,tenant_id,user_id,session_id,idempotency_key,model,mode,query,status,phase,input_sha256,input_tokens,chunk_count,completed_chunks,worker_count,retrieval_top_k,final_context_budget,max_cost,used_tokens,estimated_cost,retry_count,last_error,result,lease_owner,lease_until,created_at,updated_at,started_at,finished_at`

func (r *PostgresRepository) GetTask(ctx context.Context, id, tenant string) (*Task, error) {
	return r.scanTask(r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM long_context_tasks WHERE id=$1 AND tenant_id=$2`, id, tenant))
}
func (r *PostgresRepository) ListTasks(ctx context.Context, tenantID string, limit, offset int) ([]*Task, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM long_context_tasks WHERE tenant_id=$1`, tenantID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+taskColumns+` FROM long_context_tasks WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t, err := r.scanTask(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		out = append(out, t)
	}
	return out, total, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *PostgresRepository) scanTask(row scanner) (*Task, error) {
	var t Task
	var result []byte
	err := row.Scan(&t.ID, &t.TenantID, &t.UserID, &t.SessionID, &t.IdempotencyKey, &t.Model, &t.Mode, &t.Query, &t.Status, &t.Phase, &t.InputSHA256, &t.InputTokens, &t.ChunkCount, &t.CompletedChunks, &t.WorkerCount, &t.RetrievalTopK, &t.FinalContextBudget, &t.MaxCost, &t.UsedTokens, &t.EstimatedCost, &t.RetryCount, &t.LastError, &result, &t.LeaseOwner, &t.LeaseUntil, &t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.FinishedAt)
	if err != nil {
		return nil, err
	}
	if len(result) > 0 {
		t.Result = json.RawMessage(result)
	}
	return &t, nil
}
func (r *PostgresRepository) CancelTask(ctx context.Context, id, tenant string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE long_context_tasks SET status=$1,phase=$2,updated_at=NOW(),finished_at=COALESCE(finished_at,NOW()) WHERE id=$3 AND tenant_id=$4 AND status NOT IN ($5,$6,$7)`, StatusCanceled, PhaseQueued, id, tenant, StatusSucceeded, StatusFailed, StatusCanceled)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return nil
	}
	var status string
	if err = r.db.QueryRowContext(ctx, `SELECT status FROM long_context_tasks WHERE id=$1 AND tenant_id=$2`, id, tenant).Scan(&status); err != nil {
		return err
	}
	if Status(status) == StatusCanceled {
		return nil
	}
	return fmt.Errorf("task cannot be canceled from status %s", status)
}
func (r *PostgresRepository) TransitionTask(ctx context.Context, id string, from, to Status, phase Phase) error {
	if err := Transition(from, to); err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE long_context_tasks SET status=$1,phase=$2,updated_at=NOW(),started_at=COALESCE(started_at,NOW()),finished_at=CASE WHEN $1 IN ('succeeded','failed','canceled') THEN NOW() ELSE finished_at END WHERE id=$3 AND status=$4`, to, phase, id, from)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task transition compare-and-swap failed")
	}
	RecordTaskFailed(string(to))
	return nil
}

// FailTask moves a non-terminal task to failed and records the cause.
func (r *PostgresRepository) FailTask(ctx context.Context, id, tenant, message string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE long_context_tasks SET status=$1, phase=$2, last_error=$3, lease_owner='', lease_until=NULL, updated_at=NOW(), finished_at=NOW() WHERE id=$4 AND tenant_id=$5 AND status NOT IN ($6,$7,$8)`, StatusFailed, PhaseQueued, message, id, tenant, StatusSucceeded, StatusFailed, StatusCanceled)
	return err
}

func (r *PostgresRepository) ClaimTask(ctx context.Context, worker string, lease time.Duration) (*Task, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id, tenant string
	err = tx.QueryRowContext(ctx, `SELECT id,tenant_id FROM long_context_tasks WHERE status IN ('queued','ingesting','mapping','indexing','retrieving','synthesizing','verifying') AND (lease_until IS NULL OR lease_until<NOW()) AND (lease_owner='' OR lease_owner NOT LIKE 'hermes-chat-%') ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &tenant)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE long_context_tasks SET lease_owner=$1,lease_until=NOW()+$2::interval,updated_at=NOW() WHERE id=$3`, worker, fmt.Sprintf("%f seconds", lease.Seconds()), id); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTask(ctx, id, tenant)
}
func (r *PostgresRepository) PersistChunks(ctx context.Context, taskID, tenantID, text string, chunkTokens, overlapTokens int) error {
	parts := SplitText(text, chunkTokens, overlapTokens)
	if len(parts) == 0 {
		return fmt.Errorf("input produced no chunks")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, part := range parts {
		id := fmt.Sprintf("%s_chunk_%06d", taskID, part.Ordinal)
		if _, err := tx.ExecContext(ctx, `INSERT INTO long_context_chunks (id,task_id,ordinal,start_char,end_char,start_token,end_token,content,content_sha256,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending') ON CONFLICT (task_id,ordinal) DO NOTHING`, id, taskID, part.Ordinal, part.StartChar, part.EndChar, part.StartChar/4, part.EndChar/4, part.Content, HashInput(part.Content)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE long_context_tasks SET chunk_count=$1,updated_at=NOW() WHERE id=$2 AND tenant_id=$3`, len(parts), taskID, tenantID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepository) RenewLease(ctx context.Context, id, worker string, lease time.Duration) error {
	res, err := r.db.ExecContext(ctx, `UPDATE long_context_tasks SET lease_until=NOW()+$1::interval,updated_at=NOW() WHERE id=$2 AND lease_owner=$3`, fmt.Sprintf("%f seconds", lease.Seconds()), id, worker)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("lease lost: task %s is no longer owned by %s", id, worker)
	}
	return nil
}

// DirectClaim unconditionally sets the lease_owner and lease_until for a task.
// Use this when claiming a freshly-created task that has no lease owner yet
// (e.g. chat path) — RenewLease would fail because it matches on the current
// owner which may be empty.
func (r *PostgresRepository) DirectClaim(ctx context.Context, id, tenant, worker string, lease time.Duration) error {
	_, err := r.db.ExecContext(ctx, `UPDATE long_context_tasks SET lease_owner=$1,lease_until=NOW()+$2::interval,updated_at=NOW() WHERE id=$3 AND tenant_id=$4`, worker, fmt.Sprintf("%f seconds", lease.Seconds()), id, tenant)
	return err
}
func (r *PostgresRepository) ReleaseLease(ctx context.Context, id, worker string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE long_context_tasks SET lease_owner='',lease_until=NULL,updated_at=NOW() WHERE id=$1 AND lease_owner=$2`, id, worker)
	return err
}

// StuckTask represents a non-terminal task that may be stuck.
type StuckTask struct {
	ID              string
	TenantID        string
	Status          string
	Phase           string
	CompletedChunks int
	ChunkCount      int
	PendingChunks   int
	FailedChunks    int
	LeaseOwner      string
	LeaseUntil      string
	LastError       string
	UpdatedAt       time.Time
}

// ListStuckTasks returns non-terminal tasks sorted by oldest update first.
func (r *PostgresRepository) ListStuckTasks(ctx context.Context, limit int) ([]StuckTask, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.tenant_id, t.status, t.phase, t.completed_chunks, t.chunk_count,
			COALESCE(t.lease_owner, ''), CASE WHEN t.lease_until IS NOT NULL THEN t.lease_until::text ELSE '' END AS lease_until,
			COALESCE(t.last_error, ''), t.updated_at,
			COALESCE(ch.pending_count, 0), COALESCE(ch.failed_count, 0)
		FROM long_context_tasks t
		LEFT JOIN LATERAL (
			SELECT COUNT(*) FILTER (WHERE c.status IN ('pending','mapping')) AS pending_count,
					COUNT(*) FILTER (WHERE c.status = 'failed') AS failed_count
			FROM long_context_chunks c WHERE c.task_id = t.id
		) ch ON TRUE
		WHERE t.status NOT IN ('succeeded','failed','canceled')
		ORDER BY t.updated_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]StuckTask, 0)
	for rows.Next() {
		var t StuckTask
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Status, &t.Phase, &t.CompletedChunks,
			&t.ChunkCount, &t.LeaseOwner, &t.LeaseUntil, &t.LastError, &t.UpdatedAt,
			&t.PendingChunks, &t.FailedChunks); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// RecoverStaleLeases resets chunks and tasks with expired leases on startup
// so they can be reclaimed by workers after a crash or restart.
func (r *PostgresRepository) RecoverStaleLeases(ctx context.Context) error {
	// Reset chunks stuck in mapping/retrieving with expired leases back to pending.
	_, err := r.db.ExecContext(ctx, `UPDATE long_context_chunks SET status='pending', map_lease_until=NULL WHERE status IN ('mapping','retrieving') AND map_lease_until IS NOT NULL AND map_lease_until < NOW()`)
	if err != nil {
		return fmt.Errorf("recover stale chunks: %w", err)
	}
	// Tasks with expired leases will be auto-reclaimed by ClaimTask's
	// WHERE lease_until IS NULL OR lease_until < NOW() clause.
	// Just clear stale lease_owner for visibility.
	_, err = r.db.ExecContext(ctx, `UPDATE long_context_tasks SET lease_owner='' WHERE status NOT IN ('succeeded','failed','canceled') AND lease_until IS NOT NULL AND lease_until < NOW()`)
	if err != nil {
		return fmt.Errorf("recover stale tasks: %w", err)
	}
	return nil
}

func HashInput(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// StoreResult persists the final synthesis result JSON into the task's result column.
func (r *PostgresRepository) StoreResult(ctx context.Context, taskID string, result json.RawMessage) error {
	_, err := r.db.ExecContext(ctx, `UPDATE long_context_tasks SET result=$1,updated_at=NOW() WHERE id=$2`, result, taskID)
	return err
}

func leaseOwner(owner string, d time.Duration) interface{} {
	// lease_owner has a NOT NULL constraint; use empty string (matching
	// FailTask's behavior) instead of SQL NULL when no owner is supplied.
	if owner == "" {
		return ""
	}
	return owner
}

func leaseUntil(owner string, d time.Duration) interface{} {
	if owner == "" {
		return nil
	}
	if d <= 0 {
		d = 5 * time.Minute
	}
	return time.Now().Add(d)
}

// LoadChunks returns all chunks for a task ordered by ordinal.
func (r *PostgresRepository) LoadChunks(ctx context.Context, taskID string) ([]Chunk, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,task_id,ordinal,start_char,end_char,start_token,end_token,content,content_sha256,status,map_attempts,map_result,map_model,map_provider,created_at,updated_at FROM long_context_chunks WHERE task_id=$1 ORDER BY ordinal`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Chunk
	for rows.Next() {
		var c Chunk
		var raw []byte
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Ordinal, &c.StartChar, &c.EndChar, &c.StartToken, &c.EndToken, &c.Content, &c.ContentHash, &c.Status, &c.MapAttempts, &raw, &c.MapModel, &c.MapProvider, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			c.MapResult = json.RawMessage(raw)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindMappedChunkByHash returns the most recent successfully mapped chunk with
// identical content from a DIFFERENT task (for cross-task result reuse).
func (r *PostgresRepository) FindMappedChunkByHash(ctx context.Context, contentHash, excludeTaskID string) (*Chunk, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,task_id,ordinal,start_char,end_char,start_token,end_token,content,content_sha256,status,map_attempts,map_result,map_model,map_provider,created_at,updated_at
		FROM long_context_chunks
		WHERE content_sha256=$1 AND status='mapped' AND task_id<>$2 AND map_result IS NOT NULL
		ORDER BY updated_at DESC LIMIT 1`, contentHash, excludeTaskID)
	var c Chunk
	var raw []byte
	err := row.Scan(&c.ID, &c.TaskID, &c.Ordinal, &c.StartChar, &c.EndChar, &c.StartToken, &c.EndToken, &c.Content, &c.ContentHash, &c.Status, &c.MapAttempts, &raw, &c.MapModel, &c.MapProvider, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		c.MapResult = json.RawMessage(raw)
	}
	return &c, nil
}

// CopyMappedResult marks a chunk as mapped by reusing a prior result verbatim.
func (r *PostgresRepository) CopyMappedResult(ctx context.Context, chunkID, taskID, model, provider string, resultMap json.RawMessage) error {
	_, err := r.db.ExecContext(ctx, `UPDATE long_context_chunks SET status='mapped',map_result=$1,map_model=$2,map_provider=CASE WHEN $3 LIKE '% (reused)' THEN $3 ELSE $3||' (reused)' END,map_lease_until=NULL,updated_at=NOW() WHERE id=$4 AND task_id=$5`, resultMap, model, provider, chunkID, taskID)
	return err
}
