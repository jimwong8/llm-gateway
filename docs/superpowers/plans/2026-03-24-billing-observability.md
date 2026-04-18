# Billing & Observability Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为网关补齐增强版 Billing & Observability，基于 HTTP 请求级口径记录请求、token、成本、缓存层级、时延、错误与热点聚合，并暴露管理接口。

**Architecture:** 继续沿用 [`internal/billing/postgres.go`](internal/billing/postgres.go) 中的明细表作为唯一事实来源，扩展 [`usage_events`](internal/billing/postgres.go:31) 字段承载请求级观测数据。由 [`internal/httpserver/server.go`](internal/httpserver/server.go) 在 L1/L2/provider/fallback 各返回路径统一写入事件，再通过新增的聚合查询方法和 `/admin/observability/*` 接口提供汇总视图。

**Tech Stack:** Go, PostgreSQL, net/http, existing admin auth middleware, existing async write pattern

---

## File Map

- Modify: [`internal/billing/postgres.go`](internal/billing/postgres.go)
  - 扩展 [`UsageEvent`](internal/billing/postgres.go:8)
  - 扩展 [`usage_events`](internal/billing/postgres.go:31) schema
  - 新增 summary/cache/providers/hotspots 查询方法
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 在 [`chatCompletions`](internal/httpserver/server.go:476) 中统一计算 `latency_ms`
  - 在 L1/L2/provider/fallback/错误路径写入 billing event
  - 新增管理接口 `/admin/observability/summary`、`/admin/observability/cache`、`/admin/observability/providers`、`/admin/observability/hotspots`
- Modify: [`cmd/server/main.go`](cmd/server/main.go)
  - 保持 [`billing.NewStore()`](internal/billing/postgres.go:19) 初始化可用，并确保新 schema 自动执行
- Create: [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)
  - 聚合 SQL 查询与事件写入测试
- Create: [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)
  - 管理接口与请求链路的 HTTP 级测试
- Modify: [`docs/plans/2026-03-24-billing-observability-design.md`](docs/plans/2026-03-24-billing-observability-design.md)（如实现后需补充“已实现说明”，可选）

---

## Chunk 1: 扩展账单明细模型与存储查询

### Task 1: 扩展 [`UsageEvent`](internal/billing/postgres.go:8) 与 schema

**Files:**
- Modify: [`internal/billing/postgres.go`](internal/billing/postgres.go)
- Test: [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)

- [ ] **Step 1: 写失败测试，约束新字段存在并能写入**

在 [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go) 添加一个测试，构造包含以下字段的事件并验证查询结果存在：

```go
event := UsageEvent{
    TenantID:         "t_demo",
    UserID:           "u_demo",
    RequestID:        "req-1",
    Model:            "gpt-4o-mini",
    Provider:         "openai",
    PromptTokens:     10,
    CompletionTokens: 20,
    TotalTokens:      30,
    EstimatedCost:    0.12,
    CacheStatus:      "HIT",
    CacheLayer:       "l1_exact",
    RouteMode:        "auto",
    RouteProvider:    "openai",
    RouteModel:       "gpt-4o-mini",
    FallbackUsed:     false,
    LatencyMs:        125,
    Success:          true,
    ErrorType:        "",
    ErrorMessage:     "",
}
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/billing -run TestUsageEventInsertExpandedFields -v`
Expected: FAIL，提示 [`UsageEvent`](internal/billing/postgres.go:8) 缺字段或 SQL 列不匹配。

- [ ] **Step 3: 最小实现扩展字段**

在 [`internal/billing/postgres.go`](internal/billing/postgres.go) 中：

- 扩展 [`UsageEvent`](internal/billing/postgres.go:8)
- 在 [`ensureSchema()`](internal/billing/postgres.go:29) 中为 [`usage_events`](internal/billing/postgres.go:31) 增加字段：
  - `cache_status TEXT NOT NULL DEFAULT 'MISS'`
  - `cache_layer TEXT NOT NULL DEFAULT 'none'`
  - `route_mode TEXT NOT NULL DEFAULT 'auto'`
  - `route_provider TEXT NOT NULL DEFAULT ''`
  - `route_model TEXT NOT NULL DEFAULT ''`
  - `fallback_used BOOLEAN NOT NULL DEFAULT FALSE`
  - `latency_ms INT NOT NULL DEFAULT 0`
  - `success BOOLEAN NOT NULL DEFAULT TRUE`
  - `error_type TEXT NOT NULL DEFAULT ''`
  - `error_message TEXT NOT NULL DEFAULT ''`
- 使用 `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` 保证已有环境平滑升级
- 扩展 [`Insert()`](internal/billing/postgres.go:51) 的插入 SQL

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/billing -run TestUsageEventInsertExpandedFields -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/billing/postgres.go internal/billing/postgres_test.go
git commit -m "feat: expand usage event billing schema"
```

### Task 2: 实现 summary 聚合查询

**Files:**
- Modify: [`internal/billing/postgres.go`](internal/billing/postgres.go)
- Test: [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)

- [ ] **Step 1: 写失败测试，约束 summary 查询结果**

新增测试，插入 2-3 条不同事件，然后验证：
- 请求数
- `prompt_tokens` / `completion_tokens` / `total_tokens` 总量
- `estimated_cost` 总量
- `avg(latency_ms)`
- provider error rate
- cache hit rate

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/billing -run TestSummaryAggregation -v`
Expected: FAIL，提示 [`Summary()`](internal/billing/postgres.go) 未定义。

- [ ] **Step 3: 最小实现 [`Summary()`](internal/billing/postgres.go)**

新增：

```go
type QueryFilter struct {
    TenantID string
    Provider string
    Model    string
    From     time.Time
    To       time.Time
    Limit    int
}

type SummaryRow struct {
    Requests          int64
    PromptTokens      int64
    CompletionTokens  int64
    TotalTokens       int64
    EstimatedCost     float64
    AvgLatencyMs      float64
    ProviderErrorRate float64
    CacheHitRate      float64
}
```

实现 [`Summary(ctx, filter)`](internal/billing/postgres.go) ，统一通过一个 WHERE 构造器拼条件。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/billing -run TestSummaryAggregation -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/billing/postgres.go internal/billing/postgres_test.go
git commit -m "feat: add billing summary aggregation"
```

### Task 3: 实现 cache/providers/hotspots 聚合查询

**Files:**
- Modify: [`internal/billing/postgres.go`](internal/billing/postgres.go)
- Test: [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)

- [ ] **Step 1: 为三类聚合各写一个失败测试**

覆盖：
- cache 层级分布：`MISS/HIT/SEMANTIC_HIT` 与 `none/l1_exact/l2_semantic`
- provider 聚合：请求量、错误率、平均时延、成本
- hotspots：tenant/model Top N

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/billing -run 'Test(CacheBreakdown|ProviderBreakdown|Hotspots)' -v`
Expected: FAIL

- [ ] **Step 3: 最小实现查询方法**

新增：
- [`CacheBreakdown(ctx, filter)`](internal/billing/postgres.go)
- [`ProviderBreakdown(ctx, filter)`](internal/billing/postgres.go)
- [`Hotspots(ctx, filter)`](internal/billing/postgres.go)

其中 hotspots 可拆：

```go
type HotspotRow struct {
    Key          string
    Requests     int64
    TotalTokens  int64
    EstimatedCost float64
}
```

- [ ] **Step 4: 运行账单测试全通过**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/billing -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/billing/postgres.go internal/billing/postgres_test.go
git commit -m "feat: add observability aggregation queries"
```

---

## Chunk 2: 将 Billing & Observability 接入请求链路

### Task 4: 在 [`chatCompletions()`](internal/httpserver/server.go:476) 统一采集请求级观测字段

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 L1 命中与 provider 正常返回的事件写入**

新增 HTTP 级测试，验证请求结束后能写入：
- `cache_status`
- `cache_layer`
- `latency_ms`
- `success`
- `route_provider`
- `route_model`

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -run TestObservabilityWriteOnSuccessPaths -v`
Expected: FAIL

- [ ] **Step 3: 最小实现统一事件构造**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 在 [`chatCompletions()`](internal/httpserver/server.go:476) 开头记录：

```go
startedAt := time.Now()
```

- 新增辅助函数，例如：

```go
func (s *Server) writeUsageEventAsync(event billing.UsageEvent)
func buildUsageEvent(...)
```

- 在 L1 命中返回前写 `cache_status=HIT`, `cache_layer=l1_exact`
- 在 L2 命中返回前写 `cache_status=SEMANTIC_HIT`, `cache_layer=l2_semantic`
- 在 provider 正常返回后写 `cache_status=MISS`, `cache_layer=none`

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -run TestObservabilityWriteOnSuccessPaths -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/observability_handler_test.go
git commit -m "feat: write observability events on success paths"
```

### Task 5: 覆盖错误与 fallback 口径

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 provider 失败与 fallback 成功**

测试目标：
- 最终失败时 `success=false`
- `error_type=provider_error` 或对应错误类型
- fallback 成功时 `success=true` 且 `fallback_used=true`

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -run TestObservabilityWriteOnErrorAndFallback -v`
Expected: FAIL

- [ ] **Step 3: 最小实现错误枚举映射**

新增小型映射函数：

```go
func classifyErrorType(status int, err error) string
```

并在：
- quota 拒绝
- policy 拒绝
- auth 失败
- provider 调用失败
- internal error

这些路径统一构建账单/观测事件。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -run TestObservabilityWriteOnErrorAndFallback -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/observability_handler_test.go
git commit -m "feat: record observability events for errors and fallback"
```

---

## Chunk 3: 暴露管理接口

### Task 6: 新增 `/admin/observability/summary` 与 `/admin/observability/cache`

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)

- [ ] **Step 1: 写失败测试**

验证：
- 接口受 [`requireAdmin()`](internal/httpserver/server.go:66) 保护
- 返回 JSON 结构正确
- 查询参数 `tenant_id/from/to/provider/model` 生效

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -run 'Test(AdminObservabilitySummary|AdminObservabilityCache)' -v`
Expected: FAIL

- [ ] **Step 3: 最小实现 handler**

在 [`Handler()`](internal/httpserver/server.go:45) 注册：
- `/admin/observability/summary`
- `/admin/observability/cache`

新增 handler：
- [`adminObservabilitySummary()`](internal/httpserver/server.go)
- [`adminObservabilityCache()`](internal/httpserver/server.go)

复用统一 filter 解析函数，例如：

```go
func parseBillingFilter(r *http.Request) billing.QueryFilter
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -run 'Test(AdminObservabilitySummary|AdminObservabilityCache)' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/observability_handler_test.go
git commit -m "feat: add admin observability summary and cache endpoints"
```

### Task 7: 新增 `/admin/observability/providers` 与 `/admin/observability/hotspots`

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)

- [ ] **Step 1: 写失败测试**

覆盖：
- provider 聚合接口
- 热点租户/热点模型接口
- `limit` 生效

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -run 'Test(AdminObservabilityProviders|AdminObservabilityHotspots)' -v`
Expected: FAIL

- [ ] **Step 3: 最小实现 handler**

在 [`Handler()`](internal/httpserver/server.go:45) 注册：
- `/admin/observability/providers`
- `/admin/observability/hotspots`

新增：
- [`adminObservabilityProviders()`](internal/httpserver/server.go)
- [`adminObservabilityHotspots()`](internal/httpserver/server.go)

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -run 'Test(AdminObservabilityProviders|AdminObservabilityHotspots)' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/observability_handler_test.go
git commit -m "feat: add provider and hotspot observability endpoints"
```

---

## Chunk 4: 远程联调与回归验证

### Task 8: 补全账单与观测远程联调脚本

**Files:**
- Create: [`cmd/verify/observability.go`](cmd/verify/observability.go)
- Modify: [`cmd/verify/routing.go`](cmd/verify/routing.go)（仅当需要共享辅助函数时）

- [ ] **Step 1: 写一个可运行的远程验证脚本**

脚本需要覆盖：
- 一次普通 MISS 请求
- 一次 L1 HIT 请求
- 一次 L2 `SEMANTIC_HIT` 请求
- 一次 fallback 路径（若现有 mock provider 可触发）
- 调用 `/admin/observability/summary`
- 调用 `/admin/observability/cache`
- 调用 `/admin/observability/providers`
- 调用 `/admin/observability/hotspots`

- [ ] **Step 2: 运行脚本验证输出**

Run: `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go run ./cmd/verify/observability.go`
Expected: 输出各路径 `X-Cache`、聚合接口 JSON 摘要、无 panic。

- [ ] **Step 3: 若失败则最小修复**

仅修复本脚本暴露的实现问题，不额外扩 scope。

- [ ] **Step 4: 运行回归测试**

Run:
- `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/billing -v`
- `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go test ./internal/httpserver -v`
- `cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go build -o /dev/null ./...`

Expected: 全 PASS

- [ ] **Step 5: 提交**

```bash
git add cmd/verify/observability.go internal/billing/postgres.go internal/httpserver/server.go internal/httpserver/observability_handler_test.go internal/billing/postgres_test.go
git commit -m "feat: add billing and observability verification flow"
```

---

## Notes for Implementers

- 不要重构 [`internal/httpserver/server.go`](internal/httpserver/server.go) 的整体结构，只做局部抽取，避免扩大 diff。
- 不要引入新的聚合表或后台 job；这超出 [`docs/plans/2026-03-24-billing-observability-design.md`](docs/plans/2026-03-24-billing-observability-design.md) 的范围。
- 保持所有新增接口继续使用 [`requireAdmin()`](internal/httpserver/server.go:66)。
- 错误口径必须以“最终 HTTP 请求是否成功”为准，而不是中间 provider 尝试次数。
- 对已有远程环境，schema 变更必须使用 `ADD COLUMN IF NOT EXISTS`，避免破坏已部署实例。

Plan complete and saved to `docs/superpowers/plans/2026-03-24-billing-observability.md`. Ready to execute?