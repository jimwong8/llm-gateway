# Billing & Observability 增强设计文档

日期：2026-03-24

## 1. 背景

当前系统已经完成 Billing & Observability 的第一阶段落地：

- [`internal/billing/postgres.go`](internal/billing/postgres.go) 已支持请求级事件入库与基础聚合
- [`internal/httpserver/server.go`](internal/httpserver/server.go) 已支持 `/admin/observability/*` 管理接口
- [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go) 已具备基础 handler 测试骨架
- [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go) 已具备基础联调能力

当前增强目标不是重做数据模型，而是在现有单事件模型上补齐错误路径、fallback 路径以及更完整的聚合测试，提升统计口径的准确性与回归可信度。

## 2. 方案对比

### 方案 A：单事件增强（推荐）

继续使用单条 [`billing.UsageEvent`](internal/billing/postgres.go) 表达一次 HTTP 请求，只补齐失败路径与 fallback 标记，并扩展测试覆盖。

优点：
- 与现有实现兼容
- 改动最小
- 不引入新的账单口径复杂度
- 最符合当前已确认的统计要求

缺点：
- 无法表达一次请求内部多阶段失败细节

### 方案 B：错误与 fallback 事件拆分

为主 provider 失败、fallback 成功各写一条事件。

优点：
- 可观察性更强

缺点：
- 会破坏现有按 HTTP 请求级聚合口径
- 会影响 summary、error rate、request count 解释方式
- 超出当前阶段范围

### 方案 C：只补测试，不补写入逻辑

不增强实际写入链路，仅扩展测试覆盖。

优点：
- 风险最低

缺点：
- 不能修正真实统计缺口
- 对最终数据质量帮助有限

### 结论

采用方案 A：**单事件增强**。

## 3. 增强数据口径

### 3.1 基本原则

继续沿用 [`internal/billing/postgres.go`](internal/billing/postgres.go) 中的单事件模型，不引入双事件明细。

### 3.2 错误路径口径

当一次请求最终失败时：

- 必须写入一条 `UsageEvent`
- `success=false`
- `error_type` 必须有明确枚举值
- `error_message` 保留精简错误信息
- `latency_ms` 必须记录到失败返回前的完整耗时

### 3.3 fallback 路径口径

当主 provider 失败但 fallback 成功时：

- 仍只写一条事件
- `success=true`
- `fallback_used=true`
- `provider` / `model` / `route_provider` / `route_model` 以最终成功路径为准
- 不额外增加一条“主路由失败事件”

### 3.4 聚合指标口径

- `provider_error_rate`：仅统计最终失败且 `error_type='provider_error'` 的请求
- `cache_hit_rate`：只与 `cache_status` 有关，不受 fallback 影响
- `request count`：仍然严格按 HTTP 请求条数统计

## 4. 测试增强范围

### 4.1 [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)

增强目标：

- 增加 provider 最终失败路径测试
- 增加 fallback 成功路径测试
- 验证 `fallback_used=true` 时整体仍按成功统计
- 验证错误场景下 observability handler 返回结构不被破坏

### 4.2 [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)

增强目标：

- 增加 `provider_error_rate` 聚合测试
- 增加 `cache_hit_rate` 聚合测试
- 增加 `hotspots` 排序正确性测试
- 增加边界值测试：零请求、全失败、全缓存命中、混合 provider

### 4.3 [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go)

增强目标：

- 补充一个最终失败请求验证场景
- 补充一个 fallback 成功验证场景
- 继续保留 summary / cache / providers / hotspots 输出

## 5. 代码改动范围

### 5.1 [`internal/httpserver/server.go`](internal/httpserver/server.go)

需要增强：

- provider 最终失败路径的 `writeBillingAsync(...)`
- fallback 成功路径下 `fallback_used=true` 的一致性
- 错误分类函数（如有必要，可抽成局部辅助函数）

### 5.2 [`internal/billing/postgres.go`](internal/billing/postgres.go)

尽量不改 schema，只增强：

- 聚合 SQL 的边界处理
- 查询返回结构的稳定性

### 5.3 测试文件

- [`internal/httpserver/observability_handler_test.go`](internal/httpserver/observability_handler_test.go)
- [`internal/billing/postgres_test.go`](internal/billing/postgres_test.go)
- [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go)

## 6. 风险与控制

### 风险 1：错误路径写入造成双写或漏写

控制方式：
- 明确只有“最终返回点”负责写失败事件
- 测试覆盖 provider 失败、fallback 成功、缓存命中、正常成功四类主路径

### 风险 2：fallback 成功后 provider 归属混乱

控制方式：
- 统一以最终成功 provider/model 为事件主字段
- 增加 fallback 成功断言测试

### 风险 3：聚合 SQL 与现有数据不兼容

控制方式：
- 保持 schema 不扩张
- 通过边界数据构造验证 summary/providers/cache/hotspots 行为

## 7. 验证标准

增强完成后，应满足：

- provider 最终失败请求可被统计到 `success=false` 与 `provider_error_rate`
- fallback 成功请求可正确显示 `fallback_used=true`
- cache hit rate 与 error rate 聚合测试稳定通过
- [`cmd/verify/observability/main.go`](cmd/verify/observability/main.go) 能展示成功、失败、fallback 三类典型路径
