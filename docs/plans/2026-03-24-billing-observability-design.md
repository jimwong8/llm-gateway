# Billing & Observability 设计文档

日期：2026-03-24

## 1. 背景与目标

基于现有 [`internal/billing/postgres.go`](internal/billing/postgres.go) 的请求计费明细能力，在不引入独立聚合存储的前提下，扩展出一套增强版 Billing & Observability 方案。

本阶段目标：

- 继续以 HTTP 请求为统计口径
- 统一记录请求次数、token 用量、请求成本、缓存命中状态、命中层级、平均时延、provider 错误率
- 提供热点租户、热点模型、缓存层级分布等管理接口
- 保持与现有 [`internal/httpserver/server.go`](internal/httpserver/server.go) 请求链路和管理端风格一致
- 为后续流式请求、多次模型调用明细、离线聚合表演进预留空间

非目标：

- 本阶段不实现流式粒度统计
- 本阶段不实现一次 HTTP 请求内的多 provider 调用明细拆分
- 本阶段不引入独立小时/天聚合表

## 2. 方案对比与结论

### 方案 A：增强现有明细表并实时聚合查询

直接扩展 [`usage_events`](internal/billing/postgres.go:31) 为统一事实表，所有管理统计从明细表实时聚合得出。

优点：

- 改动最小，落地最快
- 与现有异步写入链路兼容
- 易于复用当前 PostgreSQL 存储能力

缺点：

- 数据量上升后聚合查询成本会提高
- 复杂报表性能受限于在线 SQL

### 方案 B：明细表 + 聚合读模型

保留明细表，同时新增汇总表或物化视图。

优点：

- 更适合中长期规模化查询
- 热点接口响应更稳定

缺点：

- 写入和回填逻辑复杂
- 当前阶段明显超出 MVP 范围

### 方案 C：日志优先，数据库弱化

将大部分观测打入结构化日志，数据库仅保留 token 和成本。

优点：

- 短期开发最快

缺点：

- 无法良好支持管理端聚合接口
- 后续仍需返工

### 结论

采用方案 A：**增强现有 [`internal/billing/postgres.go`](internal/billing/postgres.go) 并通过明细表实时聚合查询。**

## 3. 数据模型与统计口径

### 3.1 基础原则

继续以 [`usage_events`](internal/billing/postgres.go:31) 作为唯一事实明细表，每次 [`/v1/chat/completions`](internal/httpserver/server.go:47) 请求最多记录一条事件。

### 3.2 新增字段建议

在现有 `tenant_id / user_id / request_id / model / provider / prompt_tokens / completion_tokens / total_tokens / estimated_cost / created_at` 基础上增加：

- `cache_status`：如 `MISS`、`HIT`、`SEMANTIC_HIT`
- `cache_layer`：如 `l1_exact`、`l2_semantic`、`none`
- `route_mode`：如 `auto`、`manual`、`policy`
- `route_provider`
- `route_model`
- `fallback_used`
- `latency_ms`
- `success`
- `error_type`
- `error_message`

### 3.3 统计口径

本阶段所有统计均为 HTTP 请求级别：

- 请求次数：按事件条数统计
- token 用量：按 `prompt_tokens`、`completion_tokens`、`total_tokens` 汇总
- 请求成本：按 `estimated_cost` 汇总
- 平均时延：按 `latency_ms` 平均值
- provider 错误率：`success=false` 且 `error_type=provider_error` 的占比
- 缓存命中率：`cache_status != MISS` 的占比
- 命中层级分布：按 `cache_layer` 分组统计
- 热点租户/热点模型：按请求数或成本排序的 Top N

## 4. 接口与查询维度

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中新增只读管理接口，并沿用现有 `/admin/*` 风格。

### 4.1 建议接口

- `/admin/observability/summary`
- `/admin/observability/cache`
- `/admin/observability/providers`
- `/admin/observability/hotspots`

### 4.2 查询参数

统一支持：

- `tenant_id`
- `provider`
- `model`
- `from`
- `to`
- `limit`

### 4.3 返回内容

#### summary

返回：

- 总请求数
- token 总量
- 成本总量
- 平均时延
- provider 错误率
- 缓存命中率

#### cache

返回：

- `MISS/HIT/SEMANTIC_HIT` 分布
- `l1_exact/l2_semantic/none` 分布

#### providers

返回按 provider 聚合的：

- 请求量
- token
- 成本
- 平均时延
- 错误率

#### hotspots

返回：

- 热点租户 Top N
- 热点模型 Top N

## 5. 写入时机与错误口径

### 5.1 写入时机

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中记录请求开始时间，并在返回前统一计算 `latency_ms`。

以下路径都必须写入一条 billing/observability 事件：

- L1 命中
- L2 命中
- provider 正常返回
- provider 失败
- fallback 成功

### 5.2 成功与失败定义

- `success=true`：最终对客户端成功返回 HTTP 200
- `success=false`：最终对客户端返回错误
- `fallback_used=true`：主路径失败但备用路径成功，整体仍视为成功

### 5.3 错误类型枚举

建议有限枚举：

- `provider_error`
- `rate_limit_error`
- `policy_error`
- `authentication_error`
- `internal_error`

## 6. 存储与查询实现

### 6.1 [`internal/billing/postgres.go`](internal/billing/postgres.go)

需要扩展：

- `UsageEvent` 结构体字段
- `usage_events` 表结构
- 插入逻辑
- 聚合查询方法：
  - `Summary(...)`
  - `CacheBreakdown(...)`
  - `ProviderBreakdown(...)`
  - `Hotspots(...)`

### 6.2 索引建议

新增或补充索引：

- `(tenant_id, created_at DESC)`
- `(provider, created_at DESC)`
- `(model, created_at DESC)`
- `(cache_status, created_at DESC)`
- `(success, created_at DESC)`

## 7. 与现有链路的集成方式

### 7.1 请求链路

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：

- 记录开始时间
- 在 L1/L2/provider/fallback 各返回点统一构建 `UsageEvent`
- 复用现有异步写入模式

### 7.2 与缓存层协同

将现有 `X-Cache` 头与内部 `cache_status/cache_layer` 做统一映射：

- `MISS` -> `cache_layer=none`
- `HIT` -> `cache_layer=l1_exact`
- `SEMANTIC_HIT` -> `cache_layer=l2_semantic`

### 7.3 与路由层协同

从 [`internal/router/router.go`](internal/router/router.go) 的决策结果中提取：

- `route_mode`
- `route_provider`
- `route_model`
- `fallback_used`

## 8. 测试策略

### 8.1 单元测试

针对 [`internal/billing/postgres.go`](internal/billing/postgres.go) 增加：

- 插入字段完整性测试
- summary 聚合测试
- cache breakdown 测试
- provider/hotspots 聚合测试

### 8.2 接口测试

针对 [`internal/httpserver/server.go`](internal/httpserver/server.go) 增加：

- L1 命中写账单测试
- L2 命中写账单测试
- provider 正常响应写账单测试
- provider 失败写账单测试
- fallback 成功写账单测试

### 8.3 远程联调

远程环境验证：

- 连续两次相同请求：首发 `MISS`、二次 `HIT`
- 相似请求：`SEMANTIC_HIT`
- provider 失败后 fallback 成功：`fallback_used=true`
- 管理接口返回 summary / cache / providers / hotspots

## 9. 演进路径

本设计为后续扩展预留：

- 流式请求 token 逐段统计
- 一次 HTTP 请求内多次 provider 调用明细
- 小时/天聚合表
- OpenTelemetry / Prometheus 指标对接
- 企业级成本分摊与账单导出
