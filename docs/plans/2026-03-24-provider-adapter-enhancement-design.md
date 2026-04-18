# Provider Adapter 增强设计文档

日期：2026-03-24

## 1. 背景

当前 Provider Adapter 已具备基础能力：

- [`internal/providers/provider.go`](internal/providers/provider.go) 定义了统一的 `Provider` 接口与请求/响应结构
- [`internal/providers/openai.go`](internal/providers/openai.go) 实现了一个 OpenAI 风格的适配器
- [`internal/providers/registry.go`](internal/providers/registry.go) 实现了 provider 注册、重试、失败计数与熔断逻辑

但当前 Provider Adapter 仍存在两个明显问题：

1. **错误分类过于粗糙**
   - 当前大多数错误只是字符串拼接，无法为上层 Billing / Observability / Policy / Health 提供稳定可用的错误类型标签
2. **usage 口径不稳定**
   - provider 返回的 usage 字段可能缺失或不完整，导致 Billing / Observability 在不同 provider 下口径不一致

本阶段目标是先做最小可用增强：

- 统一 provider 错误分类
- 统一 usage 三元组 `prompt_tokens / completion_tokens / total_tokens`
- 不处理真正的流式协议差异收敛，只预留后续扩展边界

## 2. 方案对比

### 方案 A：最小可用版（推荐）

在 [`internal/providers/provider.go`](internal/providers/provider.go) 中新增统一错误包装结构与 usage 归一化工具，并在 [`internal/providers/openai.go`](internal/providers/openai.go) 中落地。

优点：
- 改动最小
- 能立即改善上游模块可观测性
- 与当前 Registry 逻辑兼容

缺点：
- 仍未覆盖流式行为差异
- 仍需要后续给更多 provider 适配器补齐

### 方案 B：增强版

在方案 A 基础上，把流式响应、finish_reason、tool calling 差异也统一收敛。

优点：
- 抽象更完整

缺点：
- 当前超出范围
- 对现有接口改动更大

### 方案 C：大重构版

整体重构 Provider Adapter 协议，统一 request/response lifecycle。

优点：
- 长期最整洁

缺点：
- 风险最高
- 当前不适合

### 结论

采用方案 A：**先统一错误分类与 usage 口径归一化。**

## 3. 错误分类与 usage 归一化

### 3.1 错误分类

在 [`internal/providers/provider.go`](internal/providers/provider.go) 中新增统一错误类型或包装结构，首版至少区分：

- `auth`
- `rate_limit`
- `timeout`
- `upstream_4xx`
- `upstream_5xx`
- `network`
- `decode`

要求：
- 上层拿到 provider 错误时，不再依赖字符串 contains 做判断
- 错误结构必须保留原始 message，便于日志与审计

### 3.2 usage 归一化

统一保证 provider response 中以下三元组完整：

- `prompt_tokens`
- `completion_tokens`
- `total_tokens`

规则：
- 若 provider 已返回完整 usage，则原样使用
- 若只缺 `total_tokens`，则自动补为 `prompt_tokens + completion_tokens`
- 若全部缺失，则置零并保持结构完整

### 3.3 与 Billing / Observability 的关系

本阶段不改 [`internal/billing/postgres.go`](internal/billing/postgres.go) 的结构，只要求其使用的 usage 来源统一稳定。

## 4. 接入边界

### 4.1 [`internal/providers/openai.go`](internal/providers/openai.go)

需要增强：

- HTTP 4xx / 5xx 分类
- 网络错误分类
- decode 错误分类
- usage 归一化

### 4.2 [`internal/providers/provider.go`](internal/providers/provider.go)

需要增强：

- 新增统一 provider error 定义
- 新增 usage 归一化辅助函数或结构

### 4.3 [`internal/providers/registry.go`](internal/providers/registry.go)

本阶段不重构 Registry 主体，只要求：

- 能消费统一后的 provider error
- 保证重试逻辑不因错误包装而破坏
- 允许后续 Health / Billing / Observability 直接读取标准化错误类型

## 5. 测试与验证策略

### 5.1 Provider 单测

建议新增或补齐：

- OpenAI 4xx 分类测试
- OpenAI 5xx 分类测试
- timeout / network 错误分类测试
- decode 错误测试
- usage 补全测试

### 5.2 Registry 回归测试

补最小回归：

- 包装后的错误不会破坏现有重试逻辑
- failure count / open interval 行为保持一致

### 5.3 范围约束

本阶段不做：

- 流式响应统一协议
- finish_reason 统一语义
- tool calling 差异抽象
- provider 能力声明矩阵

## 6. 风险与控制

### 风险 1：错误包装破坏现有重试逻辑

控制方式：
- Registry 侧只做最小兼容验证
- 本阶段不调整重试框架本身

### 风险 2：usage 补全引入错误计费

控制方式：
- 仅允许最保守补全：`total = prompt + completion`
- 其余未知情况全部置零，不做推测

### 风险 3：不同 provider 后续扩展成本不一致

控制方式：
- 先把统一错误与 usage 作为公共抽象固定下来
- 后续 provider 逐个适配

## 7. 成功标准

完成后应满足：

- OpenAI provider 能稳定返回标准化错误类型
- usage 三元组始终完整
- Registry 不因错误包装而失效
- Billing / Observability 上游可直接消费统一后的 provider 输出
