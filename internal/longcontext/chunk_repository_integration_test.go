package longcontext

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
	_ "github.com/lib/pq"
)

func TestChunkMapIntegration(t *testing.T) {
	dsn:=os.Getenv("LLM_GATEWAY_TEST_DSN"); if dsn==""{t.Skip("missing dsn")}
	db,err:=sql.Open("postgres",dsn);if err!=nil{t.Fatal(err)};defer db.Close();r,_:=NewPostgresRepository(db);ctx:=context.Background()
	id:="lct_chunk_test_"+time.Now().Format("20060102150405.000000000"); tenant:="chunk-test-tenant"
	defer db.ExecContext(ctx,`DELETE FROM long_context_tasks WHERE id=$1`,id)
	task,_,err:=r.CreateTask(ctx,CreateTaskInput{ID:id,TenantID:tenant,Model:"virtual-long-1m",Mode:"qa",Query:"q",InputSHA256:HashInput("alpha beta"),WorkerCount:1,RetrievalTopK:1,FinalBudget:16000});if err!=nil{t.Fatal(err)}
	_ = task
	if err=r.PersistChunks(ctx,id,tenant,"alpha beta gamma",10,0);err!=nil{t.Fatal(err)}
	c,err:=r.ClaimChunk(ctx,id,"chunk-worker",time.Minute);if err!=nil||c==nil{t.Fatalf("claim err=%v chunk=%+v",err,c)}
	result:=MapResult{Claims:[]MapClaim{{Claim:"beta exists",Quote:"beta",StartChar:6,EndChar:10,Confidence:.9}},ChunkSummary:"alpha beta"}
	if err=r.SaveMapResult(ctx,c.ID,id,"deepseek-ai/deepseek-v4-pro-0813","openai",result);err!=nil{t.Fatal(err)}
	var status string; if err=db.QueryRowContext(ctx,`SELECT status FROM long_context_chunks WHERE id=$1`,c.ID).Scan(&status);err!=nil{t.Fatal(err)};if status!="mapped"{t.Fatalf("status=%s",status)};_ = t
}
