# 配额与限流增强观测设计文档

日期：2026-03-24

## 1. 背景

当前系统已有基础限流能力，核心逻辑在 [`internal/quota/redis.go`](internal/quota/redis.go) 的 [`Allow()`](internal/quota/redis.go:20) 中实现；同时 Billing & Observability 已具备统一管理入口，相关接口挂载在 [`internal/httpserver/server.go`](internal/httpserver/server.go:47) 下的 `/admin/observability/*` 体系中。

本阶段目标是在**不引入新的持久化事实表**前提下，增强配额与限流的可观测性，使管理端能够直接查看：

- 当前分钟已用量
- 当前分钟拒绝次数
- 429 命中率
- 剩余额度估算
- 最近 N 分钟趋势
- 租户维度 TopN 热点限流情况

## 2. 方案对比

### 方案 A：增强现有 Redis 限流器（推荐）

直接扩展 [`internal/quota/redis.go`](internal/quota/redis.go) 中的 Redis key 结构，在 `Allow()` 写入时顺带维护 `used`、`rejected` 等实时指标，再通过管理接口读取并聚合。

优点：
- 改动最小
- 实时性最好
- 不引入第二套事实源
- 与当前限流行为完全一致

缺点：
- 只能看到短窗口实时数据
- 不适合长期历史分析

### 方案 B：新增独立 quota 观测存储层

每次限流判断后再写一份观测事件到 PostgreSQL 或另一套 Redis 读模型。

优点：
- 查询维度灵活
- 更适合长期报表

缺点：
- 实现复杂
- 会引入两套数据源一致性问题

### 方案 C：仅在 HTTP 层统计 429

不增强 [`internal/quota/redis.go`](internal/quota/redis.go)，只在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中统计限流返回。

优点：
- 最快落地

缺点：
- 无法准确得到当前分钟已用量与剩余额度估算
- 趋势能力弱

### 结论

采用方案 A：**增强现有 Redis 限流器，并把观测结果接入现有 `/admin/observability/*` 体系。**

## 3. 数据模型与统计口径

### 3.1 基础原则

不新增 PostgreSQL 事实表，继续以 Redis 作为实时限流状态源。

### 3.2 Redis 指标模型

在现有 `quota:rpm:<tenant>:<yyyyMMddHHmm>` 计数 key 基础上，补充：

- `quota:rpm:used:<tenant>:<minute>`：当前分钟用量
- `quota:rpm:rejected:<tenant>:<minute>`：当前分钟拒绝次数

其中：
- `used` 表示所有尝试次数（含通过与拒绝）或仅通过次数，需要统一定义；推荐定义为**总请求进入限流器的次数**，再用 `rejected` 单独表达拒绝量
- `remaining_estimate = max(rpm - used, 0)`

### 3.3 统计口径

以租户维度为核心，提供以下指标：

- `used`：当前分钟已用量
- `limit`：租户 RPM 配额上限
- `remaining`：当前分钟剩余额度估算
- `rejected`：当前分钟拒绝次数
- `reject_rate`：`rejected / used`，当 `used=0` 时为 0
- `trends`：最近 N 分钟按分钟聚合的 `used`、`rejected`、`remaining_estimate`

### 3.4 时间窗口

默认支持最近：

- 5 分钟
- 15 分钟
- 60 分钟

前端或管理端通过 `window_minutes` 参数选择。

## 4. 接口与查询维度

### 4.1 接口设计

继续并入现有 [`/admin/observability/*`](internal/httpserver/server.go:55) 体系，新增：

- [`/admin/observability/quota`](internal/httpserver/server.go:55)
- [`/admin/observability/quota/trends`](internal/httpserver/server.go:55)

### 4.2 查询参数

统一支持：

- `tenant_id`
- `window_minutes`
- `limit`

如无 `tenant_id`，可返回全局 TopN 或默认租户聚合视图；若实现成本较高，可首版要求 `tenant_id` 必填。

### 4.3 返回内容

#### quota

返回：

- `tenant_id`
- `used`
- `limit`
- `remaining`
- `rejected`
- `reject_rate`

#### quota/trends

返回：

- `tenant_id`
- `window_minutes`
- `points: []`
  - `minute`
  - `used`
  - `rejected`
  - `remaining_estimate`

## 5. 写入时机、趋势计算与测试策略

### 5.1 写入时机

在 [`internal/quota/redis.go`](internal/quota/redis.go) 的 [`Allow()`](internal/quota/redis.go:20) 中完成：

- 每次请求进入时增加当前分钟 `used`
- 若超额拒绝，则增加当前分钟 `rejected`

### 5.2 趋势计算

不做离线任务，直接按分钟 key 回读最近 N 分钟 Redis 数据，在服务端组装时间序列。

优点：
- 实现简单
- 实时性好
- 与 Redis 限流状态天然一致

### 5.3 测试策略

#### 单元测试

在 [`internal/quota/redis.go`](internal/quota/redis.go) 增加：

- `Allow()` 正常通过测试
- `Allow()` 超限拒绝测试
- `used/rejected` key 写入测试
- 趋势读取与剩余额度估算测试

#### Handler 测试

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 对应的 observability handler 增加：

- `/admin/observability/quota` 返回结构测试
- `/admin/observability/quota/trends` 返回结构测试
- 参数解析与边界值测试

#### 远程联调

通过远程压力或脚本验证：

- 连续请求触发 429
- quota summary 能看到 `rejected > 0`
- quota trends 能看到最近窗口数据变化

## 6. 风险与控制

### 风险 1：Redis key 口径不一致

控制方式：
- 明确 `used` 与 `rejected` 的定义
- 在测试中固定断言

### 风险 2：趋势查询开销过大

控制方式：
- 首版窗口控制在 60 分钟内
- 每次最多读取固定数量分钟桶

### 风险 3：管理接口与 Billing 统计口径混淆

控制方式：
- 在返回 JSON 中明确字段名为 `used / rejected / reject_rate`
- 不把 quota 数据直接混写进 billing summary

## 7. 成功标准

本阶段完成后，管理员应能够：

- 实时看到某租户当前分钟已用配额与剩余额度
- 确认当前分钟是否发生限流拒绝
- 查看最近 N 分钟限流趋势
- 在不查 Redis 原始 key 的前提下理解当前限流状态
