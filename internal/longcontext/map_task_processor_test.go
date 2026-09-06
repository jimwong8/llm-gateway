package longcontext

import("context";"testing";"time")
type procClaimer struct{chunk *Chunk; released bool}
func(p *procClaimer)ClaimChunk(context.Context,string,string,time.Duration)(*Chunk,error){return p.chunk,nil}
func(p *procClaimer)ReleaseLease(context.Context,string,string)error{p.released=true;return nil}
type procCaller struct{raw string}
func(p procCaller)CallMap(context.Context,string,Chunk)(string,error){return p.raw,nil}
type procResults struct{saved bool}
func(p *procResults)SaveMapResult(context.Context,string,string,string,string,MapResult)error{p.saved=true;return nil}
type procAttempts struct{count int}
func(p *procAttempts)RecordAttempt(context.Context,Attempt)error{p.count++;return nil}
func(p *procAttempts)MarkChunkFailed(context.Context,string,string,string)error{return nil}
type procCompleted struct{count int}
func(p *procCompleted)AddCompletedChunk(context.Context,string,int,int,float64)error{p.count++;return nil}
func TestMapTaskProcessorSuccess(t *testing.T){c:=&procClaimer{chunk:&Chunk{ID:"c",TaskID:"t",StartChar:0,EndChar:16,Content:"alpha beta gamma"}};rs:=&procResults{};a:=&procAttempts{};done:=&procCompleted{};p:=&MapTaskProcessor{Claimer:c,Selector:NewChannelSelector([]string{"k"},time.Minute),Caller:procCaller{raw:`{"chunk_summary":"s","claims":[{"claim":"f","quote":"beta","start_char":6,"end_char":10,"confidence":0.9}]}`},Results:rs,Attempts:a,Completed:done,WorkerID:"w",Model:"m"};if err:=p.ProcessTask(context.Background(),&Task{ID:"t"});err!=nil{t.Fatal(err)};if !rs.saved||a.count!=1||done.count!=1||c.released{t.Fatalf("saved=%v attempts=%d completed=%d released=%v",rs.saved,a.count,done.count,c.released)}}
