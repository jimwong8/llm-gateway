# Quota Observability Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为配额与限流能力补齐实时观测，提供当前分钟用量、拒绝次数、剩余额度估算与最近 N 分钟趋势接口，并接入现有 [`/admin/observability/*`](internal/httpserver/server.go:55) 体系。

**Architecture:** 在现有 [`internal/quota/redis.go`](internal/quota/redis.go) 的 Redis 分钟桶基础上扩展 `used/rejected` 指标与趋势回读逻辑，不新增 PostgreSQL 事实表。管理侧通过 [`internal/httpserver/server.go`](internal/httpserver/server.go) 新增 quota 观测接口，直接从 Redis 读取实时状态并返回结构化 JSON。

**Tech Stack:** Go, Redis, net/http, existing admin auth middleware, existing quota limiter, existing observability admin endpoints

---

## File Map

- Modify: [`internal/quota/redis.go`](internal/quota/redis.go)
  - 扩展 [`Limiter`](internal/quota/redis.go:11)
  - 在 [`Allow()`](internal/quota/redis.go:20) 中维护 `used/rejected`
  - 新增 quota summary / trend 查询方法
- Create: [`internal/quota/redis_test.go`](internal/quota/redis_test.go)
  - 覆盖 `Allow()`、`used/rejected`、趋势与剩余额度估算
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 新增 [`/admin/observability/quota`](internal/httpserver/server.go:55)
  - 新增 [`/admin/observability/quota/trends`](internal/httpserver/server.go:55)
  - 解析 quota 查询参数
- Create: [`internal/httpserver/quota_observability_handler_test.go`](internal/httpserver/quota_observability_handler_test.go)
  - 覆盖 handler 注册、参数解析、返回结构
- Modify: [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go)
  - 增加 quota summary / trends 联调
- Optional Modify: [`docs/plans/2026-03-24-quota-observability-design.md`](docs/plans/2026-03-24-quota-observability-design.md)
  - 补充实现备注（可选）

---

## Chunk 1: 扩展 Redis 限流器的数据模型与查询接口

### Task 1: 为 [`internal/quota/redis.go`](internal/quota/redis.go) 增加实时观测结构与方法

**Files:**
- Modify: [`internal/quota/redis.go`](internal/quota/redis.go)
- Test: [`internal/quota/redis_test.go`](internal/quota/redis_test.go)

- [ ] **Step 1: 写失败测试，约束 quota summary 结构**

在 [`internal/quota/redis_test.go`](internal/quota/redis_test.go) 增加一个测试，先定义预期结构：

```go
type Summary struct {
    TenantID  string `json:"tenant_id"`
    Used      int64  `json:"used"`
    Limit     int    `json:"limit"`
    Remaining int64  `json:"remaining"`
    Rejected  int64  `json:"rejected"`
    RejectRate float64 `json:"reject_rate"`
}
```

测试至少断言：
- 零值 summary 中 `Remaining >= 0`
- `RejectRate` 在 `used=0` 时为 0

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/quota -run TestQuotaSummaryShape -v`
Expected: FAIL，提示新结构或方法不存在。

- [ ] **Step 3: 最小实现 quota summary 结构与接口**

在 [`internal/quota/redis.go`](internal/quota/redis.go) 新增：

```go
type Summary struct { ... }
type TrendPoint struct { ... }
func (l *Limiter) Summary(ctx context.Context, tenantID string) (Summary, error)
func (l *Limiter) Trends(ctx context.Context, tenantID string, windowMinutes int) ([]TrendPoint, error)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/quota -run TestQuotaSummaryShape -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/quota/redis.go internal/quota/redis_test.go
git commit -m "feat: add quota observability summary types"
```

### Task 2: 在 [`Allow()`](internal/quota/redis.go:20) 中维护 `used/rejected` 计数

**Files:**
- Modify: [`internal/quota/redis.go`](internal/quota/redis.go)
- Test: [`internal/quota/redis_test.go`](internal/quota/redis_test.go)

- [ ] **Step 1: 写失败测试，覆盖通过与拒绝路径**

新增测试：
- 第一次请求：`used=1`, `rejected=0`, `allowed=true`
- 超过上限后的请求：`used` 增长，`rejected` 增长，`allowed=false`

建议测试名：
- `TestAllowTracksUsedAndRejected`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/quota -run TestAllowTracksUsedAndRejected -v`
Expected: FAIL

- [ ] **Step 3: 最小实现 `used/rejected` key 写入**

在 [`Allow()`](internal/quota/redis.go:20) 中：
- 统一维护 `quota:rpm:used:<tenant>:<minute>`
- 在拒绝分支维护 `quota:rpm:rejected:<tenant>:<minute>`
- 保持 TTL 与当前分钟 key 一致

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/quota -run TestAllowTracksUsedAndRejected -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/quota/redis.go internal/quota/redis_test.go
git commit -m "feat: track quota used and rejected counters"
```

### Task 3: 实现趋势读取与剩余额度估算

**Files:**
- Modify: [`internal/quota/redis.go`](internal/quota/redis.go)
- Test: [`internal/quota/redis_test.go`](internal/quota/redis_test.go)

- [ ] **Step 1: 写失败测试，覆盖趋势与剩余额度**

新增测试：
- 最近 5 分钟返回固定长度点集
- 每个点都包含 `minute`, `used`, `rejected`, `remaining_estimate`
- `remaining_estimate = max(limit - used, 0)`

建议测试名：
- `TestTrendsAndRemainingEstimate`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/quota -run TestTrendsAndRemainingEstimate -v`
Expected: FAIL

- [ ] **Step 3: 最小实现趋势查询**

在 [`internal/quota/redis.go`](internal/quota/redis.go) 中：
- 使用当前 UTC 时间向前回溯 `windowMinutes`
- 拼 Redis key 批量读取或逐条读取
- 返回稳定时间序列（按分钟升序）

- [ ] **Step 4: 运行 quota 全量测试**

Run: `go test ./internal/quota -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/quota/redis.go internal/quota/redis_test.go
git commit -m "feat: add quota trend queries"
```

---

## Chunk 2: 暴露管理接口

### Task 4: 新增 [`/admin/observability/quota`](internal/httpserver/server.go:55)

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/quota_observability_handler_test.go`](internal/httpserver/quota_observability_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖路由与基础返回结构**

测试内容：
- 接口已注册
- 未初始化 quota store 时返回 503
- `tenant_id` 参数解析正常

建议测试名：
- `TestAdminObservabilityQuota_ServiceUnavailable`
- `TestAdminObservabilityQuota_RouteRegistered`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run 'TestAdminObservabilityQuota_' -v`
Expected: FAIL

- [ ] **Step 3: 最小实现 handler**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 注册 `/admin/observability/quota`
- 新增 `adminObservabilityQuota(...)`
- 复用或新增参数解析函数，例如：

```go
func parseQuotaFilter(r *http.Request) (tenantID string, windowMinutes int, limit int)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/httpserver -run 'TestAdminObservabilityQuota_' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/quota_observability_handler_test.go
git commit -m "feat: add quota observability summary endpoint"
```

### Task 5: 新增 [`/admin/observability/quota/trends`](internal/httpserver/server.go:55)

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/quota_observability_handler_test.go`](internal/httpserver/quota_observability_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 trends 路由与结构**

测试内容：
- 路由存在
- `window_minutes` 正确解析
- 返回 `tenant_id`, `window_minutes`, `points`

建议测试名：
- `TestAdminObservabilityQuotaTrends_RouteRegistered`
- `TestParseQuotaFilter`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run 'Test(AdminObservabilityQuotaTrends_|ParseQuotaFilter)' -v`
Expected: FAIL

- [ ] **Step 3: 最小实现 trends handler**

新增：
- `adminObservabilityQuotaTrends(...)`

并注册到 [`Handler()`](internal/httpserver/server.go:47)。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/httpserver -run 'Test(AdminObservabilityQuotaTrends_|ParseQuotaFilter)' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/quota_observability_handler_test.go
git commit -m "feat: add quota observability trends endpoint"
```

---

## Chunk 3: 增强联调验证

### Task 6: 为 [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go) 增加 quota 联调场景

**Files:**
- Modify: [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go)

- [ ] **Step 1: 写 quota summary 场景**

在脚本中增加：
- 生成一个唯一 `tenant_id`
- 连续发送请求逼近 rpm 上限
- 调用 `/admin/observability/quota`

- [ ] **Step 2: 写 quota trends 场景**

增加：
- 调用 `/admin/observability/quota/trends?tenant_id=...&window_minutes=5`
- 打印趋势点集

- [ ] **Step 3: 运行脚本并确认输出不完整**

Run: `go run ./cmd/verify/observability`
Expected: 初始可能没有 quota 输出或结构不完整。

- [ ] **Step 4: 最小修复脚本/接口**

只修复与 quota 观测直接相关的逻辑。

- [ ] **Step 5: 运行脚本确认通过**

Run: `go run ./cmd/verify/observability`
Expected: 输出 quota summary 与 quota trends。

- [ ] **Step 6: 提交**

```bash
git add cmd/verify/observability/main.go internal/httpserver/server.go internal/quota/redis.go internal/httpserver/quota_observability_handler_test.go internal/quota/redis_test.go
git commit -m "test: add quota observability verification flow"
```

---

## Chunk 4: 全量回归

### Task 7: 回归测试与编译验证

**Files:**
- No new files

- [ ] **Step 1: 运行 quota 测试**

Run: `go test ./internal/quota -v`
Expected: PASS

- [ ] **Step 2: 运行 httpserver 测试**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 3: 运行 observability 验证命令**

Run: `go test ./cmd/verify/observability`
Expected: PASS 或 `[no test files]`

- [ ] **Step 4: 运行关键包回归**

Run: `go test ./internal/quota ./internal/httpserver ./cmd/server ./cmd/verify/observability`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add .
git commit -m "chore: finalize quota observability"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-quota-observability.md`. Ready to execute?
