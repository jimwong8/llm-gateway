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
	CancelTask(context.Context, string, string) error
	TransitionTask(context.Context, string, Status, Status, Phase) error
	ClaimTask(context.Context, string, time.Duration) (*Task, error)
	RenewLease(context.Context, string, string, time.Duration) error
	ReleaseLease(context.Context, string, string) error
}

type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	if db == nil { return nil, fmt.Errorf("database is required") }
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) CreateTask(ctx context.Context, in CreateTaskInput) (*Task, bool, error) {
	if strings.TrimSpace(in.ID) == "" { return nil, false, fmt.Errorf("task id is required") }
	row := r.db.QueryRowContext(ctx, `INSERT INTO long_context_tasks (id,tenant_id,user_id,session_id,idempotency_key,model,mode,query,input_sha256,input_tokens,worker_count,retrieval_top_k,final_context_budget,max_cost,status,phase) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT DO NOTHING RETURNING id`, in.ID,in.TenantID,in.UserID,in.SessionID,in.IdempotencyKey,in.Model,in.Mode,in.Query,in.InputSHA256,in.InputTokens,in.WorkerCount,in.RetrievalTopK,in.FinalBudget,in.MaxCost,StatusQueued,PhaseQueued)
	var id string; err:=row.Scan(&id)
	if err==sql.ErrNoRows && in.IdempotencyKey!="" { t,e:=r.findByIdempotency(ctx,in.TenantID,in.IdempotencyKey); if e!=nil{return nil,false,e}; if t.InputSHA256!=in.InputSHA256{return nil,false,fmt.Errorf("idempotency key conflicts with existing input")}; return t,true,nil }
	if err!=nil{return nil,false,err}; t,err:=r.scanTask(r.db.QueryRowContext(ctx,`SELECT `+taskColumns+` FROM long_context_tasks WHERE id=$1 AND tenant_id=$2`,id,in.TenantID)); return t,false,err
}
func (r *PostgresRepository) findByIdempotency(ctx context.Context,tenant,key string)(*Task,error){return r.scanTask(r.db.QueryRowContext(ctx,`SELECT `+taskColumns+` FROM long_context_tasks WHERE tenant_id=$1 AND idempotency_key=$2`,tenant,key))}
const taskColumns=`id,tenant_id,user_id,session_id,idempotency_key,model,mode,query,status,phase,input_sha256,input_tokens,chunk_count,completed_chunks,worker_count,retrieval_top_k,final_context_budget,max_cost,used_tokens,estimated_cost,retry_count,last_error,result,lease_owner,lease_until,created_at,updated_at,started_at,finished_at`
func(r *PostgresRepository)GetTask(ctx context.Context,id,tenant string)(*Task,error){return r.scanTask(r.db.QueryRowContext(ctx,`SELECT `+taskColumns+` FROM long_context_tasks WHERE id=$1 AND tenant_id=$2`,id,tenant))}
func(r *PostgresRepository)scanTask(row *sql.Row)(*Task,error){var t Task;var result []byte;err:=row.Scan(&t.ID,&t.TenantID,&t.UserID,&t.SessionID,&t.IdempotencyKey,&t.Model,&t.Mode,&t.Query,&t.Status,&t.Phase,&t.InputSHA256,&t.InputTokens,&t.ChunkCount,&t.CompletedChunks,&t.WorkerCount,&t.RetrievalTopK,&t.FinalContextBudget,&t.MaxCost,&t.UsedTokens,&t.EstimatedCost,&t.RetryCount,&t.LastError,&result,&t.LeaseOwner,&t.LeaseUntil,&t.CreatedAt,&t.UpdatedAt,&t.StartedAt,&t.FinishedAt);if err!=nil{return nil,err};if len(result)>0{t.Result=json.RawMessage(result)};return &t,nil}
func(r *PostgresRepository)CancelTask(ctx context.Context,id,tenant string)error{res,err:=r.db.ExecContext(ctx,`UPDATE long_context_tasks SET status=$1,phase=$2,updated_at=NOW(),finished_at=COALESCE(finished_at,NOW()) WHERE id=$3 AND tenant_id=$4 AND status NOT IN ($5,$6,$7)`,StatusCanceled,PhaseQueued,id,tenant,StatusSucceeded,StatusFailed,StatusCanceled);if err!=nil{return err};n,_:=res.RowsAffected();if n>0{return nil};var status string;if err=r.db.QueryRowContext(ctx,`SELECT status FROM long_context_tasks WHERE id=$1 AND tenant_id=$2`,id,tenant).Scan(&status);err!=nil{return err};if Status(status)==StatusCanceled{return nil};return fmt.Errorf("task cannot be canceled from status %s",status)}
func(r *PostgresRepository)TransitionTask(ctx context.Context,id string,from,to Status,phase Phase)error{if err:=Transition(from,to);err!=nil{return err};res,err:=r.db.ExecContext(ctx,`UPDATE long_context_tasks SET status=$1,phase=$2,updated_at=NOW(),started_at=COALESCE(started_at,NOW()),finished_at=CASE WHEN $1 IN ('succeeded','failed','canceled') THEN NOW() ELSE finished_at END WHERE id=$3 AND status=$4`,to,phase,id,from);if err!=nil{return err};n,_:=res.RowsAffected();if n==0{return fmt.Errorf("task transition compare-and-swap failed")};return nil}
func(r *PostgresRepository)ClaimTask(ctx context.Context,worker string,lease time.Duration)(*Task,error){tx,err:=r.db.BeginTx(ctx,nil);if err!=nil{return nil,err};defer tx.Rollback();var id,tenant string;err=tx.QueryRowContext(ctx,`SELECT id,tenant_id FROM long_context_tasks WHERE status IN ('queued','ingesting','mapping','indexing','retrieving','synthesizing','verifying') AND (lease_until IS NULL OR lease_until<NOW()) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id,&tenant);if err==sql.ErrNoRows{return nil,nil};if err!=nil{return nil,err};if _,err=tx.ExecContext(ctx,`UPDATE long_context_tasks SET lease_owner=$1,lease_until=NOW()+$2::interval,updated_at=NOW() WHERE id=$3`,worker,fmt.Sprintf("%f seconds",lease.Seconds()),id);err!=nil{return nil,err};if err=tx.Commit();err!=nil{return nil,err};return r.GetTask(ctx,id,tenant)}
func(r *PostgresRepository)RenewLease(ctx context.Context,id,worker string,lease time.Duration)error{_,err:=r.db.ExecContext(ctx,`UPDATE long_context_tasks SET lease_until=NOW()+$1::interval,updated_at=NOW() WHERE id=$2 AND lease_owner=$3`,fmt.Sprintf("%f seconds",lease.Seconds()),id,worker);return err}
func(r *PostgresRepository)ReleaseLease(ctx context.Context,id,worker string)error{_,err:=r.db.ExecContext(ctx,`UPDATE long_context_tasks SET lease_owner='',lease_until=NULL,updated_at=NOW() WHERE id=$1 AND lease_owner=$2`,id,worker);return err}
func HashInput(s string)string{h:=sha256.Sum256([]byte(s));return hex.EncodeToString(h[:])}
