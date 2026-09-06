package longcontext

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestPostgresRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("LLM_GATEWAY_TEST_DSN")
	if dsn == "" { t.Skip("LLM_GATEWAY_TEST_DSN not set") }
	db, err := sql.Open("postgres", dsn); if err != nil { t.Fatal(err) }
	defer db.Close()
	r, err := NewPostgresRepository(db); if err != nil { t.Fatal(err) }
	ctx := context.Background(); id := "lct_test_integration_"+time.Now().Format("20060102150405.000000000")
	in := CreateTaskInput{ID:id,TenantID:"test-tenant",Model:"virtual-long-1m",Mode:"qa",Query:"q",InputSHA256: HashInput("integration"), IdempotencyKey: "integration-key", WorkerCount: 1,RetrievalTopK:1,FinalBudget:16000}
	defer db.ExecContext(ctx, `DELETE FROM long_context_tasks WHERE id=$1`, id)
	task, existing, err := r.CreateTask(ctx,in); if err != nil || existing || task.Status != StatusQueued { t.Fatalf("create existing=%v err=%v task=%+v",existing,err,task) }
	task2, existing, err := r.CreateTask(ctx,in); if err != nil || !existing || task2.ID != id { t.Fatalf("idempotency existing=%v err=%v task=%+v",existing,err,task2) }
	if err := r.TransitionTask(ctx,id,StatusQueued,StatusSucceeded,PhaseQueued); err == nil { t.Fatal("invalid transition accepted") }
	claimed, err := r.ClaimTask(ctx,"integration-worker",time.Minute); if err != nil || claimed == nil || claimed.ID != id { t.Fatalf("claim err=%v claimed=%+v",err,claimed) }
	if err := r.ReleaseLease(ctx,id,"integration-worker"); err != nil { t.Fatal(err) }
	if err := r.CancelTask(ctx,id,"test-tenant"); err != nil { t.Fatal(err) }
	if err := r.CancelTask(ctx,id,"test-tenant"); err != nil { t.Fatal("cancel not idempotent: ",err) }
}
