# virtual-long-1m Phase 1 实施计划

> 本计划只覆盖可恢复的数据层、状态机和最小 API 契约；不把 `virtual-long-1m` 直接转发给单一 256K 上游。

## 目标

实现一个 feature-flag 保护的长上下文任务骨架：可以创建任务、持久化原始输入摘要和状态、查询/取消任务，并为后续 Map/Reduce worker 提供稳定的 repository 与 lease 接口。

第一阶段不接入普通 `/v1/chat/completions`，不启用生产长任务执行，不修改现有 AUTO 路由。

## 现状约束

远端项目：`/home/jimwong/projects/llm-gateway`

当前已有：

- Go HTTP server；
- `providers.Registry`；
- PostgreSQL；
- Redis；
- audit/billing/limits；
- channel 隔离和 NVIDIA 10-key pool。

当前没有：

- long-context task repository；
- chunk/evidence/attempt 表；
- long-context worker；
- task API；
- cancellation/lease recovery。

远端工作树已经存在大量既有未提交变更。实施时必须只添加本计划列出的文件，不使用 `git reset`、批量清理或覆盖既有修改。

## Phase 1 文件范围

新增：

```text
internal/db/migrations/032_long_context_tasks.sql
internal/longcontext/model.go
internal/longcontext/state.go
internal/longcontext/repository.go
internal/longcontext/repository_test.go
internal/httpserver/long_context_handler.go
internal/httpserver/long_context_handler_test.go
```

修改：

```text
internal/httpserver/server.go
cmd/server/main.go
```

不修改：

```text
internal/router/router.go
internal/providers/openai.go
普通 /v1/chat/completions 逻辑
现有 channels 表数据
NVIDIA key
```

## Task 1：数据库 migration

创建 `032_long_context_tasks.sql`，包含：

- `long_context_tasks`
- `long_context_chunks`
- `long_context_evidence`
- `long_context_attempts`

必须包含：

- 外键和 cascade；
- `(tenant_id, idempotency_key)` 唯一索引；
- `(task_id, ordinal)` 唯一索引；
- status/phase 查询索引；
- lease 查询索引；
- created_at/updated_at 索引。

原文内容可以保存于 chunk 表，但 API key、Authorization、内部凭据必须永不落表。

验证：

```bash
# 使用实际 llm-gateway PostgreSQL 执行 migration
psql ... -f internal/db/migrations/032_long_context_tasks.sql
\dt long_context_*
```

## Task 2：领域模型

`internal/longcontext/model.go` 定义：

```go
type Task struct {
    ID string
    TenantID string
    UserID string
    SessionID string
    IdempotencyKey string
    Model string
    Mode string
    Query string
    Status string
    Phase string
    InputSHA256 string
    InputTokens int64
    ChunkCount int
    CompletedChunks int
    WorkerCount int
    RetrievalTopK int
    FinalContextBudget int
    MaxCost *float64
    UsedTokens int64
    EstimatedCost float64
    RetryCount int
    LastError string
    Result json.RawMessage
    LeaseOwner string
    LeaseUntil *time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
    StartedAt *time.Time
    FinishedAt *time.Time
}
```

状态常量：

```text
queued
ingesting
mapping
indexing
retrieving
synthesizing
verifying
succeeded
failed
canceled
```

状态迁移必须集中在 `state.go`，禁止 handler 直接拼接状态字符串。非法迁移返回 error。

建议允许：

```text
queued -> ingesting/canceled
 ingesting -> mapping/failed/canceled
 mapping -> indexing/failed/canceled
 indexing -> retrieving/failed/canceled
 retrieving -> synthesizing/failed/canceled
 synthesizing -> verifying/failed/canceled
 verifying -> succeeded/failed/canceled
```

终态不可再迁移。

## Task 3：Repository

`internal/longcontext/repository.go` 提供接口：

```go
type Repository interface {
    CreateTask(ctx context.Context, input CreateTaskInput) (*Task, error)
    GetTask(ctx context.Context, id string, tenantID string) (*Task, error)
    CancelTask(ctx context.Context, id string, tenantID string) error
    TransitionTask(ctx context.Context, id string, from, to, phase string) error
    ClaimTask(ctx context.Context, workerID string, lease time.Duration) (*Task, error)
    RenewLease(ctx context.Context, id, workerID string, lease time.Duration) error
    ReleaseLease(ctx context.Context, id, workerID string) error
}
```

`ClaimTask` 必须使用事务：

```sql
SELECT ...
FROM long_context_tasks
WHERE status IN ('queued','ingesting','mapping','indexing','retrieving','synthesizing','verifying')
  AND (lease_until IS NULL OR lease_until < NOW())
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT 1
```

然后在同一事务中写入 `lease_owner` 和 `lease_until`。

`CancelTask` 必须是幂等的：

- queued/running -> canceled；
- canceled -> 成功返回；
- succeeded/failed -> 不改状态，返回明确错误或 no-op，必须统一定义并测试。

idempotency：

- 相同 tenant + idempotency key + 相同 input hash：返回已有任务；
- 相同 key + 不同 input hash：HTTP 409；
- 空 key 允许创建普通任务，但不能获得幂等保证。

## Task 4：API handler

新增路由：

```text
POST /v1/long-context/tasks
GET  /v1/long-context/tasks/{id}
POST /v1/long-context/tasks/{id}/cancel
GET  /v1/long-context/tasks/{id}/result
```

第一阶段 API 行为：

- 创建任务只入库，返回 202；
- 不启动 Map worker；
- `feature flag` 默认 false；
- flag false 时返回 404 或明确 `long_context_disabled`，不能悄悄走普通 chat；
- 查询/取消只允许拥有 tenant/user scope 的调用方；
- result 在非 succeeded 时返回 409/404，不能返回空成功结果。

创建请求限制：

```text
model 必须为 virtual-long-1m
mode 必须为 qa/summary/analysis
query 非空
input.text 非空
最大原文大小先限制为 32MiB（Phase 1），后续再扩大到 1M token
worker_count: 1..10
chunk_tokens: 4000..32000
chunk_overlap_tokens: 0..4000
final_context_budget: 16000..200000
max_cost > 0 时才启用成本上限
```

第一阶段不接受 URL 抓取，避免 SSRF；只接受 inline text。

响应必须包含：

```json
{
  "id": "lct_...",
  "status": "queued",
  "phase": "queued",
  "model": "virtual-long-1m",
  "status_url": "/v1/long-context/tasks/lct_...",
  "result_url": "/v1/long-context/tasks/lct_..."
}
```

## Task 5：feature flag 和 server wiring

新增配置字段：

```text
LONG_CONTEXT_ENABLED=false
LONG_CONTEXT_MAX_INPUT_BYTES=33554432
LONG_CONTEXT_WORKERS=0
```

`LONG_CONTEXT_WORKERS=0` 表示 Phase 1 不启动 worker。

在 `Server` 中以可选依赖方式挂载 handler/repository。feature flag 默认 false，确保升级后普通 API 行为完全不变。

## Task 6：测试顺序

严格采用 RED-GREEN：

1. 状态机非法迁移测试；
2. idempotency 相同 hash 返回同一 task；
3. idempotency 不同 hash 返回冲突；
4. cancel 幂等测试；
5. lease claim 不重复测试；
6. feature flag false 不进入普通 chat；
7. API 202 响应契约；
8. result 非终态不返回成功结果。

PostgreSQL 测试不连接生产库。使用临时数据库或 repository integration profile；如果环境没有测试 PostgreSQL，先完成纯单元测试，再把集成测试标记为显式环境测试，不得伪称通过。

## Task 7：安全和隐私

必须做到：

- 明文 API key 不进入 task/chunk/result/attempt；
- 日志只记录 task id、hash 前缀、大小、phase、provider、model；
- query/input 不能完整写入普通日志；
- tenant scope 不能由 URL 参数覆盖认证上下文；
- 取消和查询必须检查 tenant/user ownership；
- 任意异常只返回安全错误摘要；
- 原文按 task retention 清理；
- 结果下载不能绕过认证。

## Phase 1 验收矩阵

| 验收项 | 证据 |
|---|---|
| migration 可重复执行 | 第二次执行无错误 |
| 创建任务持久化 | DB 回读 task |
| 同 key 同 hash 幂等 | 两次返回同 ID |
| 同 key 不同 hash 冲突 | HTTP 409 |
| 非法状态迁移被拒绝 | 单元测试 |
| lease 不重复领取 | 并发集成测试 |
| cancel 幂等 | 两次调用结果一致 |
| feature flag 默认关闭 | API 不进入普通 chat |
| 普通 chat 无回归 | 现有 HTTP tests |
| 凭据不落日志 | grep/redaction 检查 |
| 服务重启不丢任务 | DB 中任务仍存在 |

## Phase 2 预留

完成 Phase 1 后，再实现：

- chunk tokenizer/切分；
- 4~10 worker pool；
- NVIDIA key/channel 选择器；
- 每 key/model 独立 limiter；
- Map prompt 和 JSON schema；
- quote 确定性校验；
- evidence 入库；
- hybrid retrieval；
- final reader；
- verifier；
- metrics/audit/billing；
- 断点恢复。

在 Phase 2 完成前，不将 `virtual-long-1m` 加入普通 Hermes provider 的 default model，不让普通 Chat Completion 客户端误用该名称。
