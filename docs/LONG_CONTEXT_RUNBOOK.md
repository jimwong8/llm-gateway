# 虚拟长上下文 (virtual-long-1m) 运维手册

> 最后更新: 2026-08-31  —  包含 map/reader 模型切换、reasoning_effort 修复、重试机制、gate 清理、worker 模式、lease 恢复等所有已实施改动。

## 1. 开关与配置

所有配置通过 `/home/jimwong/projects/llm-gateway/.env` 控制：

```bash
# 总开关
LONG_CONTEXT_ENABLED=true

# 后台 worker 数量：1 表示自动消费队列
LONG_CONTEXT_WORKERS=1

# worker map/reader 模型（不使用 NVIDIA）
LONG_CONTEXT_WORKER_CHANNEL=sensenova-file-01
LONG_CONTEXT_WORKER_MODEL=sensenova-6.8-flash-lite
LONG_CONTEXT_WORKER_READER_CHANNEL=sensenova-file-01
LONG_CONTEXT_WORKER_READER_MODEL=sensenova-6.8-flash-lite
LONG_CONTEXT_WORKER_READER_TOP_K=4

# 单请求限制
LONG_CONTEXT_MAX_INPUT_BYTES=33554432   # 32 MiB

# Channel gate / rate limit（防止上游过载）
PROVIDER_CHANNEL_MAX_CONCURRENCY=3
PROVIDER_CHANNEL_MIN_INTERVAL_MS=500
PROVIDER_CHANNEL_RPM=30

# Circuit breaker
PROVIDER_FAILURE_THRESHOLD=10
PROVIDER_OPEN_SECONDS=10
```

修改后重启：

```bash
systemctl restart llm-gateway@1 llm-gateway@2
```

## 2. 模型说明

### 为什么用 sensenova-6.8-flash-lite？

| 模型 | 问题 |
|------|------|
| deepseek-v4-flash | 强制 reasoning，content 不稳定（偶发空） |
| glm-5.2 | 全部 reasoning tokens，content 恒空 |
| sensenova-6.8-flash-lite | 加 `reasoning_effort=none` 后稳定输出 JSON |

### reasoning_effort 机制

`ChatCompletionRequest` 新增 `ReasoningEffort` 字段（`omitempty`）。
- map/reader 调用时注入 `reasoning_effort: "none"` → 关闭上游 thinking 模式
- 普通 AUTO 聊天路径不设置 → 零影响（字段不序列化）
- NVIDIA NIM 走 `prepareNIMRequest`，有独立的 `chat_template_kwargs.thinking` 控制

## 3. 状态与指标

### 健康检查

```bash
curl -sS http://127.0.0.1:8090/healthz
```

### expvar 指标

```bash
curl -sS http://127.0.0.1:8090/debug/vars | python3 -m json.tool | grep longcontext
```

关键字段：
- `longcontext.tasks.created`
- `longcontext.tasks.completed.by_status`
- `longcontext.chunks.processed.by_status`
- `longcontext.worker.claims` / `longcontext.worker.errors`
- `longcontext.durations.seconds`

### 数据库状态速查

```sql
-- 任务状态分布
SELECT status, phase, COUNT(*) FROM long_context_tasks
  WHERE created_at > NOW() - INTERVAL '24 hours'
  GROUP BY status, phase;

-- 卡在 mapping 的 chunk（过期 lease）
SELECT task_id, id, status, map_lease_until
  FROM long_context_chunks
  WHERE status = 'mapping' AND (map_lease_until IS NULL OR map_lease_until < NOW());

-- 按任务查看 evidence
SELECT task_id, COUNT(*) AS evidence
  FROM long_context_evidence
  GROUP BY task_id ORDER BY evidence DESC;

-- 卡住的任务
SELECT id, tenant_id, status, phase, completed_chunks, chunk_count, last_error
FROM long_context_tasks
WHERE status NOT IN ('succeeded','failed','canceled')
  AND updated_at < NOW() - INTERVAL '10 minutes';
```

## 4. 卡住任务处理

### 4.1 重置卡住的 chunk

```sql
UPDATE long_context_chunks
SET status='pending', map_lease_until=NULL, map_attempts=0
WHERE task_id='<TASK_ID>' AND status IN ('mapping','retrieving');
```

### 4.2 清理 stale channel gate

```bash
redis-cli DEL 'llm-gateway:channel-gate:sensenova-file-01'
```

或重启服务（启动时自动清理所有 gate key）：

```bash
systemctl restart llm-gateway@1 llm-gateway@2
```

### 4.3 取消整个任务

```sql
UPDATE long_context_tasks
SET status='canceled', phase='queued', finished_at=NOW(), updated_at=NOW()
WHERE id='<TASK_ID>';
```

## 5. Admin 手动运行

### 5.1 创建任务

```bash
curl -sS -X POST http://127.0.0.1:8090/v1/long-context/tasks \
  -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "test-tenant",
    "query": "Summarize this document",
    "input": {"text": "..."},
    "options": {
      "worker_count": 1,
      "retrieval_top_k": 4,
      "final_context_budget": 180000,
      "chunk_tokens": 16000,
      "chunk_overlap_tokens": 2000
    }
  }'
```

### 5.2 手动推进（不依赖 worker）

```bash
curl -sS -X POST http://127.0.0.1:8090/admin/long-context/run \
  -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{
    "task_id": "<TASK_ID>",
    "tenant_id": "test-tenant",
    "channel": "sensenova-file-01",
    "model": "sensenova-6.8-flash-lite",
    "max_chunks": 20,
    "reader_model": "sensenova-6.8-flash-lite",
    "reader_channel": "sensenova-file-01",
    "reader_top_k": 4
  }'
```

Admin run 行为：
- 单 chunk 失败后继续处理剩余 chunk（不 break）
- Channel 不可用时阻塞等待最多 60 秒（`PickWithTimeout`）
- 所有 chunk mapped 后自动进入 synthesis 阶段
- `MaxRetries=3`：失败重试最多 3 次

## 6. Worker 模式

设 `LONG_CONTEXT_WORKERS=1`，worker 自动领取 `queued` 状态的任务。

Worker 生命周期：
1. 启动时清理 stale channel gates（Redis key）
2. 启动时恢复 stale leases（chunk 回 pending，task 清 lease_owner）
3. `ClaimTask` 用 `FOR UPDATE SKIP LOCKED` 防并发竞争
4. `ProcessTask` 执行 ingest → map → retrieval → synthesis → verifying
5. SIGTERM 时 cancel worker context，优雅退出

## 7. 重试机制

### Map 阶段

- 每次失败记录 `long_context_attempts`
- `MapTaskProcessor.fail()` 检查 `MapAttempts >= MaxRetries(3)`
- 未超限：释放 chunk 回 `pending`，可被重领
- 超限：标记 `failed`，不再重试
- `ProcessTask` 在 claim 后也检查 `MapAttempts > MaxRetries`

### 非致命错误

- 上游 `empty content` → 记录 failed，chunk 回 pending 重试
- 上游 timeout → 记录 failed，chunk 回 pending 重试
- JSON 解析失败 → 记录 failed，chunk 回 pending 重试（下次可能通过 `RepairMapJSON` 修复）

## 8. 回归测试标准

### 测试用例

| 类型 | 文本长度 | chunk_tokens | 预期 chunks |
|------|----------|-------------|-------------|
| 短文本 | ~242 字符 | 800 | 1 |
| 中文本 | ~1210 字符 | 200 | 3-5 |
| 长文本 | ~3630 字符 | 200 | 8-12 |

### 验收标准

每项测试必须验证：
- `status=succeeded`
- `processed_chunks == successes`（或 successes > 0 且有 evidence）
- `evidence_count > 0`
- `citation_coverage == 1`
- answer 包含 `[E<n]` 引用
- 无 NVIDIA channel 参与
- 测试耗时 < 120 秒

## 9. 部署备份

每次修改前备份二进制和源码：

```bash
cd /home/jimwong/projects/llm-gateway
cp -a llm-gateway llm-gateway.bak-before-<change>-$(date +%Y%m%d-%H%M%S)
# 修改源码后
su jimwong -c 'go build -buildvcs=false -o /tmp/lgw_<change> ./cmd/server'
install -o root -g root -m 0755 /tmp/lgw_<change> llm-gateway
systemctl restart llm-gateway@1 llm-gateway@2
```

### 已实施改动历史

| 日期 | 改动 | 备份 |
|------|------|------|
| 2026-08-31 | reasoning_effort=none + MaxTokens 8192 | llm-gateway.bak-before-reasoning-* |
| 2026-08-31 | PickWithTimeout + BlockingSelector | llm-gateway.bak-before-blocking-* |
| 2026-08-31 | MaxRetries=3 + fail() 重试语义 | llm-gateway.bak-before-retry-* |
| 2026-08-31 | CleanupStaleGates 启动清理 | llm-gateway.bak-before-gate-* |
| 2026-08-31 | synth fix (mapErr 清除) | llm-gateway.bak-before-synthfix-* |
| 2026-08-31 | RecoverStaleLeases + SIGTERM cancel | llm-gateway.bak-before-lease-* |
| 2026-08-31 | CountPendingChunks 含 mapping | llm-gateway.bak-before-pending-fix-* |

## 10. 故障排查

### 任务卡在 mapping

1. 检查 chunk 状态：是否有过期 lease 的 mapping chunk
2. 检查 attempts：是否有连续失败
3. 检查 channel gate：`redis-cli HGETALL llm-gateway:channel-gate:<channel>`
4. 检查上游连通性：`curl -sS https://token.sensenova.cn/v1/models`
5. 重启服务（自动清理 gate + 恢复 lease）

### Worker 不消费队列

1. 确认 `LONG_CONTEXT_WORKERS > 0`
2. 确认 `LONG_CONTEXT_WORKER_CHANNEL/MODEL` 已配置
3. 检查日志：`journalctl -u llm-gateway@1 | grep worker`
4. 检查任务状态：`status` 应为 `queued`
5. 检查 DB：任务是否有 chunks

### Answer 为空

1. 检查 synthesis 是否执行（`status` 应为 `succeeded` 或 `verifying`）
2. 检查 evidence 数量：`SELECT count(*) FROM long_context_evidence WHERE task_id='...'`
3. 检查 `citation_coverage`：应为 1
