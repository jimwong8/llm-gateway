# virtual-long-1m 编排器设计

> 状态：设计冻结，尚未接入生产请求路径
> 目标：让 256K 上下文模型可靠处理约 1M tokens 的文档/会话材料，不伪造单模型 1M 全局注意力。

## 1. 结论

`virtual-long-1m` 是一个任务编排能力，不是普通上游模型别名。

它必须通过：

```text
长输入
  -> 分块
  -> 多 worker 提取结构化证据
  -> PostgreSQL 持久化
  -> BM25/向量/KG 混合检索
  -> 冲突检测与证据回查
  -> 256K final reader
  -> 带引用的结果
```

禁止将 `virtual-long-1m` 直接映射到任意单一 256K provider；禁止把四个独立模型的自然语言输出当作共享 hidden state 或 KV cache。

## 2. 当前网关事实

当前项目是 Go HTTP 网关，核心入口为：

- `internal/httpserver/server.go`
- `internal/providers/registry.go`
- `internal/providers/openai.go`
- `internal/router/router.go`
- `internal/memory/postgres.go`
- `internal/cache/redis.go`
- `internal/audit/postgres.go`
- `internal/billing/postgres.go`

已经存在：

- OpenAI-compatible provider registry；
- 按 channel 隔离的上游凭据；
- PostgreSQL 审计、计费、模型限额、消息和 session summary；
- Redis/L1 cache；
- 健康检查、熔断、fallback、模型治理；
- NVIDIA NIM channel 和 AUTO 注册。

尚不存在：

- long task 表；
- chunk/evidence 表；
- 可恢复的编排 worker；
- 长上下文任务 API；
- 任务级取消/重试/租约机制。

因此首期必须新增专用任务路径，不能复用普通同步 chat handler 来承载 1M 输入。

## 3. 对外 API 契约

### 3.1 创建任务

```http
POST /v1/long-context/tasks
Authorization: Bearer <gateway-key>
Content-Type: application/json
Idempotency-Key: <client-generated-key>
```

请求：

```json
{
  "model": "virtual-long-1m",
  "mode": "qa",
  "query": "问题或分析目标",
  "input": {
    "text": "原始长文本"
  },
  "options": {
    "chunk_tokens": 16000,
    "chunk_overlap_tokens": 800,
    "worker_count": 4,
    "retrieval_top_k": 32,
    "final_context_budget": 180000,
    "max_cost": 0.5,
    "require_citations": true
  },
  "session_id": "optional-session-id",
  "tenant_id": "optional-tenant-id",
  "user_id": "optional-user-id"
}
```

响应：

```http
202 Accepted
Location: /v1/long-context/tasks/lct_<id>
```

```json
{
  "id": "lct_<id>",
  "status": "queued",
  "model": "virtual-long-1m",
  "created_at": "...",
  "status_url": "/v1/long-context/tasks/lct_<id>",
  "result_url": "/v1/long-context/tasks/lct_<id>/result"
}
```

### 3.2 查询任务

```http
GET /v1/long-context/tasks/{id}
```

状态：

```text
queued -> ingesting -> mapping -> indexing -> retrieving -> synthesizing -> verifying -> succeeded
                                                                                \-> failed
                                                                                \-> canceled
```

响应必须包含：

- 当前阶段；
- 总 chunk 数和已完成 chunk 数；
- 重试次数；
- 当前/累计 token 使用量；
- 当前/累计成本；
- 最后错误的安全摘要；
- `updated_at` 和 worker lease 信息；
- 是否可恢复。

### 3.3 取消任务

```http
POST /v1/long-context/tasks/{id}/cancel
```

取消必须幂等。已经 `succeeded` 的任务不能被改回 canceled；正在运行的 worker 通过 DB 状态和 context cancellation 停止后续调用。

### 3.4 获取结果

```http
GET /v1/long-context/tasks/{id}/result
```

结果：

```json
{
  "task_id": "lct_<id>",
  "answer": "最终答案",
  "citations": [
    {
      "chunk_id": "chunk_003",
      "start_char": 12030,
      "end_char": 12890,
      "quote": "原文证据"
    }
  ],
  "conflicts": [],
  "models_used": [
    {"model": "...", "provider": "...", "role": "map"}
  ],
  "usage": {
    "input_tokens": 1000000,
    "worker_tokens": 480000,
    "final_reader_tokens": 120000,
    "total_tokens": 600000,
    "estimated_cost": 0.0
  },
  "verification": {
    "status": "passed",
    "citation_coverage": 0.92,
    "uncited_claims": 0
  }
}
```

## 4. 数据模型

新增 migration：`internal/db/migrations/032_long_context_tasks.sql`

### 4.1 `long_context_tasks`

```sql
id TEXT PRIMARY KEY,
tenant_id TEXT,
user_id TEXT,
session_id TEXT,
idempotency_key TEXT,
model TEXT NOT NULL,
mode TEXT NOT NULL,
query TEXT NOT NULL,
status TEXT NOT NULL,
phase TEXT NOT NULL,
input_sha256 TEXT NOT NULL,
input_tokens BIGINT NOT NULL DEFAULT 0,
chunk_count INTEGER NOT NULL DEFAULT 0,
completed_chunks INTEGER NOT NULL DEFAULT 0,
worker_count INTEGER NOT NULL DEFAULT 4,
retrieval_top_k INTEGER NOT NULL DEFAULT 32,
final_context_budget INTEGER NOT NULL DEFAULT 180000,
max_cost DOUBLE PRECISION,
used_tokens BIGINT NOT NULL DEFAULT 0,
estimated_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
retry_count INTEGER NOT NULL DEFAULT 0,
last_error TEXT NOT NULL DEFAULT '',
result JSONB,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
started_at TIMESTAMPTZ,
finished_at TIMESTAMPTZ,
lease_owner TEXT,
lease_until TIMESTAMPTZ
```

唯一约束：

```sql
UNIQUE (tenant_id, idempotency_key)
```

### 4.2 `long_context_chunks`

保存原文，不只保存摘要：

```sql
id TEXT PRIMARY KEY,
task_id TEXT NOT NULL REFERENCES long_context_tasks(id) ON DELETE CASCADE,
ordinal INTEGER NOT NULL,
start_char BIGINT NOT NULL,
end_char BIGINT NOT NULL,
start_token BIGINT NOT NULL,
end_token BIGINT NOT NULL,
content TEXT NOT NULL,
content_sha256 TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'pending',
map_attempts INTEGER NOT NULL DEFAULT 0,
map_result JSONB,
map_model TEXT,
map_provider TEXT,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (task_id, ordinal)
```

### 4.3 `long_context_evidence`

```sql
id BIGSERIAL PRIMARY KEY,
task_id TEXT NOT NULL REFERENCES long_context_tasks(id) ON DELETE CASCADE,
chunk_id TEXT NOT NULL REFERENCES long_context_chunks(id) ON DELETE CASCADE,
claim TEXT NOT NULL,
quote TEXT NOT NULL,
start_char BIGINT NOT NULL,
end_char BIGINT NOT NULL,
entities JSONB NOT NULL DEFAULT '[]',
relations JSONB NOT NULL DEFAULT '[]',
confidence DOUBLE PRECISION NOT NULL DEFAULT 0.0,
verification_status TEXT NOT NULL DEFAULT 'unverified',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### 4.4 `long_context_attempts`

```sql
id BIGSERIAL PRIMARY KEY,
task_id TEXT NOT NULL REFERENCES long_context_tasks(id) ON DELETE CASCADE,
chunk_id TEXT,
phase TEXT NOT NULL,
model TEXT NOT NULL,
provider TEXT NOT NULL,
request_id TEXT,
status TEXT NOT NULL,
input_tokens INTEGER NOT NULL DEFAULT 0,
output_tokens INTEGER NOT NULL DEFAULT 0,
estimated_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
latency_ms INTEGER NOT NULL DEFAULT 0,
error_type TEXT NOT NULL DEFAULT '',
error_message TEXT NOT NULL DEFAULT '',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

敏感信息规则：API key 不进任何任务/attempt/result 字段；原文访问必须遵循 tenant/user 权限；日志只记 hash、长度和安全错误摘要。

## 5. 分块策略

默认不使用 220K 大块。默认 chunk 16K，原因：

- 更适合精确检索；
- 失败重试成本低；
- 能并行调度；
- 适合跨块引用；
- 256K final reader 可以容纳多轮证据和冲突材料。

建议：

```text
chunk_tokens = 16K
overlap = 800 tokens
单任务最大原始输入 = 1.2M tokens
worker_count = 4
```

代码必须按 token 估算/切分，而不是按字节粗暴切割。若暂时没有与上游一致的 tokenizer，第一版应使用保守字符估算，并把 `input_tokens` 标记为 estimated，不能冒充精确值。

## 6. Map 阶段

每个 chunk 发送独立请求，要求 JSON 结构化输出：

```json
{
  "claims": [
    {
      "claim": "...",
      "quote": "必须逐字来自 chunk",
      "start_char": 0,
      "end_char": 100,
      "entities": [],
      "relations": [],
      "confidence": 0.0,
      "needs_cross_chunk_validation": true
    }
  ],
  "chunk_summary": "不超过指定预算",
  "open_dependencies": [],
  "conflicts_seen": []
}
```

服务端必须：

1. 校验 JSON schema；
2. 校验 quote 确实出现在 chunk 原文；
3. 校验字符位置和 chunk 边界；
4. 拒绝越界引用；
5. 记录原始响应 hash 和 attempt；
6. 失败只重试当前 chunk；
7. 达到重试上限后任务进入 failed/degraded，不静默丢块。

## 7. 检索阶段

首期采用混合检索：

```text
BM25/tsvector         关键词和精确实体
向量检索              语义相关性
KG/关系检索            跨块实体和关系
时间/ordinal 邻近      保持上下文连续性
```

候选排序建议：

```text
0.35 semantic
0.25 lexical
0.20 entity/relation
0.10 source confidence
0.10 positional/neighbor expansion
```

最终送入 reader 的内容必须包含：

- query；
- 相关 chunk 原文片段；
- 结构化 claims；
- 原文 quote；
- chunk ordinal 和位置；
- 冲突组；
- 需要回查的 claims。

不要只把摘要送入 final reader。

## 8. 冲突和核验

### 8.1 冲突检测

冲突来源：

- 同一实体同一属性的不同值；
- 时间线前后矛盾；
- 不同 chunk 对同一事件的不同描述；
- worker 摘要与原文 quote 不一致。

冲突状态：

```text
unverified -> corroborated
unverified -> disputed
unverified -> rejected
```

### 8.2 Final reader

Final reader 只能使用已检索的证据包。提示中必须要求：

- 每个重要结论绑定 citation；
- 不得引用不存在的 chunk；
- 无证据时明确说无法确认；
- 出现冲突时列出不同证据并说明采用理由；
- 不把 worker 摘要当作原文；
- 不输出内部 API key、租户字段和调度信息。

### 8.3 Verifier

Verifier 是独立的一次低成本请求或确定性检查，至少检查：

- quote 是否原文存在；
- citation chunk 是否属于任务；
- claim 是否能被 quote 支持；
- JSON 是否完整；
- 是否有未处理 conflict；
- 是否超过成本/输出预算。

生产模式建议：确定性检查失败时先回查原文；语义 verifier 只作为第二层，不替代字符级证据校验。

## 9. 调度和恢复

首期可以使用进程内 worker pool，但任务状态必须落 PostgreSQL，不能只放内存。

调度规则：

1. 创建任务使用 idempotency key；
2. worker 通过 `SELECT ... FOR UPDATE SKIP LOCKED` 领取任务/chunk；
3. 写入 `lease_owner` 和 `lease_until`；
4. worker 每次完成一个 chunk 就提交事务；
5. lease 超时后其他 worker 可回收；
6. 服务重启后从数据库恢复 queued/running 任务；
7. 已完成 chunk 不重复调用；
8. 所有外部调用带 timeout；
9. 任务取消后禁止新 chunk 调用；
10. 最终阶段使用 compare-and-swap，避免两个 worker 同时生成结果。

如果后续任务规模变大，再把 worker 拆成独立进程或接 Redis Streams；首期不引入额外消息系统。

## 10. Provider 选择

`virtual-long-1m` 不应直接参与普通 AUTO 评分。

Map 阶段：

- 可用的 NVIDIA NIM free 模型按实际能力和响应质量选择；
- 失败可切换到其他已探测 provider；
- 单个 provider 的 429 不得熔断同 channel 其他模型；
- reasoning 模型必须有足够 max_tokens；
- 记录实际 route model/provider。

Final reader：

- 优先选择已验证支持工具调用、结构化输出和稳定正文的模型；
- 不默认使用只返回 reasoning_content 的模型；
- final reader 的窗口保守控制在 180K–200K，留出 system prompt、工具 schema 和输出预算。

NVIDIA 模型进入普通 AUTO 池不代表它们自动适合 long-context map/final 两个角色。应为角色建立单独 capability：

```text
map: 结构化抽取、低成本、稳定 JSON
reader: 长输入、全局综合、工具/JSON 稳定
verifier: 低成本、引用核验
```

## 11. 成本、限流和安全

任务创建前估算最坏成本：

```text
map_cost + retrieval/followup_cost + final_reader_cost + verifier_cost
```

超过 `max_cost` 直接拒绝或要求显式确认，不依赖 provider 默认值。

限制：

- 单任务最大原始 token；
- 最大 chunk 数；
- 单 chunk 最大重试；
- 每 provider 并发数；
- 每 tenant 并发任务数；
- 全局 worker 数；
- final reader 最大输出；
- 任务保留期和原文删除策略。

## 12. 分阶段实施

### Phase 1：契约和数据层

- 新增 migration 032；
- 新增 `internal/longcontext` package；
- 实现 task/chunk/attempt/evidence repository；
- 增加状态机和 lease；
- 加单元测试和 PostgreSQL 集成测试。

### Phase 2：同步最小竖切

- 实现单任务、单 worker、少量 chunk；
- 新增 `POST/GET /v1/long-context/tasks`；
- 先支持 `mode=qa`；
- map -> retrieve -> final -> verify；
- 不接入普通 `/v1/chat/completions`。

### Phase 3：并行 worker 和恢复

- worker pool；
- SKIP LOCKED；
- lease recovery；
- cancellation；
- retry/backoff；
- metrics 和 audit。

### Phase 4：混合检索和冲突核验

- PostgreSQL FTS；
- 现有 embedding 服务接入；
- KG evidence lookup；
- conflict groups；
- citation coverage。

### Phase 5：生产接入

- 在 `/v1/models` 暴露 `virtual-long-1m`，但 metadata 明确标注 `task=long_context`；
- 对 `model=virtual-long-1m` 返回 202 task semantics，不走普通 upstream path；
- Hermes 侧增加专用调用适配，而不是把它当成普通 streaming chat；
- 灰度、限额、监控、回滚。

## 13. 明确不做

- 不共享不同模型的 hidden states/KV cache；
- 不把 4 个模型响应简单拼接成 1M prompt；
- 不把单一 256K provider 声称为 1M；
- 不把所有 NVIDIA 84 个模型全部加入 AUTO；
- 不依赖一次成功请求证明长上下文能力；
- 不用普通摘要替代可回查的原文证据；
- 不在任务失败时静默删除 chunk 或降低结果可信度。

## 14. 验收标准

### 功能

- 1M token 级输入可创建任务并返回 task id；
- 任务可查询、取消、恢复；
- 服务重启后不丢任务；
- 已完成 chunk 不重复调用；
- 结果包含可验证 citation；
- 跨 chunk 依赖和冲突可显式呈现。

### 可靠性

- 任意单 chunk provider 失败不会丢失整任务状态；
- provider 429、timeout、5xx 分类处理不同；
- 任务有最大运行时间和最大成本；
- lease 超时可回收；
- 并发任务不会重复 finalize。

### 真实性

- 每个返回 citation 的 quote 都能在数据库原文中找到；
- route model/provider/attempt 可审计；
- `virtual-long-1m` 不会把原始 1M 输入直接转发给 256K 上游；
- `/v1/chat/completions` 普通路径行为不回归。

### 运维

- Prometheus 指标：任务数、阶段延迟、chunk 成功率、重试率、citation coverage、成本、provider 分布；
- 管理端能查看失败原因和任务阶段；
- 保留旧二进制和 migration 回滚策略；
- 可通过 feature flag 关闭新入口而不影响普通 AUTO。
