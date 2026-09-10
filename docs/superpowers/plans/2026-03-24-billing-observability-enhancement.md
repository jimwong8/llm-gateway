# Billing & Observability Enhancement Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 Billing & Observability 基础上，补齐最终失败请求统计、fallback 成功口径与更完整的聚合测试覆盖。

**Architecture:** 保持单条 [`billing.UsageEvent`](internal/billing/postgres.go) 对应一次 HTTP 请求的模型，不引入双事件明细。重点增强 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中错误与 fallback 返回点的写入一致性，并通过 [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)、[`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go) 与 [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go) 补齐回归与联调覆盖。

**Tech Stack:** Go, net/http, PostgreSQL, existing billing store, existing router fallback logic, existing verify command

---

## File Map

- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 补齐最终失败请求的 billing event 写入
  - 统一 fallback 成功时的 `fallback_used=true` 成功口径
  - 必要时提取小型错误分类辅助函数
- Modify: [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)
  - 补齐 provider error rate、cache hit rate、hotspots 排序与边界测试
- Modify: [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)
  - 增加错误路径与 fallback 路径测试
- Modify: [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go)
  - 增加失败与 fallback 场景联调
- Optional Modify: [`docs/plans/2026-03-24-billing-observability-enhancement-design.md`](docs/plans/2026-03-24-billing-observability-enhancement-design.md)
  - 实现后补记已落地说明（可选，不强制）

---

## Chunk 1: 补齐错误与 fallback 写入口径

### Task 1: 为最终失败请求补齐 Observability/Billing 写入

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 provider 最终失败路径**

在 [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go) 增加测试，目标：
- 请求最终失败时会触发 billing 写入分支
- 失败事件应满足：
  - `success=false`
  - `error_type="provider_error"`
  - `fallback_used=false`
- 如果当前测试难以直接观察 DB，可通过注入 stub store 或可记录调用参数的替身对象来断言

建议测试名：
- `TestObservability_WritesFailureEventOnProviderError`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run TestObservability_WritesFailureEventOnProviderError -v`
Expected: FAIL，原因应为当前失败路径没有完整写入或不可观察。

- [ ] **Step 3: 写最小实现**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 找到 provider 调用后的最终失败返回点
- 在 `internalError(...)` 返回前写入一条 [`billing.UsageEvent`](internal/billing/postgres.go)
- 字段要求：
  - `Success=false`
  - `ErrorType="provider_error"`
  - `ErrorMessage=err.Error()`（必要时截断）
  - `CacheStatus` 延续当时路径值
  - `CacheLayer="none"`（若是 provider 阶段失败）
  - `LatencyMs` 使用请求总耗时

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run TestObservability_WritesFailureEventOnProviderError -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/observability_handler_test.go
git commit -m "feat: record billing event for final provider failures"
```

### Task 2: 为 fallback 成功路径补齐 `fallback_used=true` 口径

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 fallback 成功**

目标：
- 主模型失败但 fallback 成功时，仅写一条成功事件
- 该事件满足：
  - `success=true`
  - `fallback_used=true`
  - `provider/model/route_provider/route_model` 指向最终成功路径

建议测试名：
- `TestObservability_WritesSuccessEventOnFallbackSuccess`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run TestObservability_WritesSuccessEventOnFallbackSuccess -v`
Expected: FAIL

- [ ] **Step 3: 写最小实现**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 复用现有 `fallbackUsed` 布尔值
- 确保最终成功写入使用 fallback 后更新过的决策信息
- 不增加第二条失败明细事件

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run TestObservability_WritesSuccessEventOnFallbackSuccess -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/observability_handler_test.go
git commit -m "feat: track fallback success in billing events"
```

---

## Chunk 2: 增强聚合测试覆盖

### Task 3: 补齐 `provider_error_rate` 与 `cache_hit_rate` 聚合测试

**Files:**
- Modify: [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)

- [ ] **Step 1: 写失败测试，覆盖 provider error rate**

新增测试，构造一组事件：
- 2 条成功事件
- 1 条 `success=false` 且 `error_type="provider_error"`
- 1 条 `success=false` 且 `error_type="internal_error"`

期望：
- `provider_error_rate = 0.25`

建议测试名：
- `TestSummary_ProviderErrorRate`

- [ ] **Step 2: 写失败测试，覆盖 cache hit rate**

构造：
- 1 条 `cache_status="MISS"`
- 1 条 `cache_status="HIT"`
- 1 条 `cache_status="SEMANTIC_HIT"`

期望：
- `cache_hit_rate = 2/3`

建议测试名：
- `TestSummary_CacheHitRate`

- [ ] **Step 3: 运行测试并确认失败**

Run: `go test ./internal/billing -run 'TestSummary_(ProviderErrorRate|CacheHitRate)' -v`
Expected: FAIL 或暴露边界缺失。

- [ ] **Step 4: 最小修复 SQL 或测试夹具**

如果聚合 SQL 已正确，则只需补足测试夹具。
如果发现 SQL 口径偏差，则在 [`internal/billing/postgres.go`](internal/billing/postgres.go) 做最小修复。

- [ ] **Step 5: 运行测试并确认通过**

Run: `go test ./internal/billing -run 'TestSummary_(ProviderErrorRate|CacheHitRate)' -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/billing/postgres.go internal/billing/postgres_test.go
git commit -m "test: cover provider error rate and cache hit rate"
```

### Task 4: 补齐 hotspots 排序与边界值测试

**Files:**
- Modify: [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)

- [ ] **Step 1: 写失败测试，覆盖 hotspots 排序**

构造多租户、多模型事件，验证：
- `Tenants` 按 `requests DESC` 排序
- `Models` 按 `requests DESC` 排序
- 相同 `requests` 时按 key 升序稳定排序

建议测试名：
- `TestHotspots_SortedByRequests`

- [ ] **Step 2: 写失败测试，覆盖边界值**

至少覆盖：
- 零事件时返回空列表
- `limit<=0` 时默认取 10
- `tenant_id/provider/model` filter 生效

建议测试名：
- `TestHotspots_EmptyAndFilterCases`

- [ ] **Step 3: 运行测试并确认失败**

Run: `go test ./internal/billing -run 'TestHotspots_(SortedByRequests|EmptyAndFilterCases)' -v`
Expected: FAIL 或暴露未覆盖行为。

- [ ] **Step 4: 最小实现修复**

如排序或 limit/filter 行为不符合预期，在 [`internal/billing/postgres.go`](internal/billing/postgres.go) 做最小修复。

- [ ] **Step 5: 运行 billing 全量测试**

Run: `go test ./internal/billing -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/billing/postgres.go internal/billing/postgres_test.go
git commit -m "test: strengthen hotspots aggregation coverage"
```

---

## Chunk 3: 增强联调脚本

### Task 5: 为 [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go) 增加失败与 fallback 场景

**Files:**
- Modify: [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go)

- [ ] **Step 1: 写失败场景验证输出**

在验证脚本中增加：
- 一个可触发最终失败的请求（如使用失败注入模型）
- 打印对应返回状态与观测结果

- [ ] **Step 2: 写 fallback 成功场景验证输出**

增加：
- 一个先走失败模型、再走 fallback 模型成功的请求
- 打印 `fallback_used` 相关头或聚合结果

- [ ] **Step 3: 运行脚本并确认输出不完整或行为不对**

Run: `go run ./cmd/verify/observability`
Expected: 初次可能缺少失败/回退信息。

- [ ] **Step 4: 最小修复脚本或服务端逻辑**

只修复与验证目标直接相关的逻辑，不扩 scope。

- [ ] **Step 5: 运行脚本确认通过**

Run: `go run ./cmd/verify/observability`
Expected: 至少打印以下几类结果：
- exact hit
- semantic hit
- final failure
- fallback success
- summary/cache/providers/hotspots

- [ ] **Step 6: 提交**

```bash
git add cmd/verify/observability/main.go internal/httpserver/server.go internal/httpserver/observability_handler_test.go internal/billing/postgres_test.go
git commit -m "test: add failure and fallback observability verification"
```

---

## Chunk 4: 全量回归

### Task 6: 执行增强项回归测试

**Files:**
- No new files

- [ ] **Step 1: 运行 billing 测试**

Run: `go test ./internal/billing -v`
Expected: PASS

- [ ] **Step 2: 运行 httpserver 测试**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 3: 运行 observability 验证命令**

Run: `go test ./cmd/verify/observability`
Expected: PASS 或 `[no test files]`

- [ ] **Step 4: 运行关键包编译回归**

Run: `go test ./internal/billing ./internal/httpserver ./cmd/server ./cmd/verify/observability`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add .
git commit -m "chore: finalize billing observability enhancements"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-billing-observability-enhancement.md`. Ready to execute?
