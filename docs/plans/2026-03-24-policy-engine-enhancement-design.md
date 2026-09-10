# Policy Engine 增强设计文档

日期：2026-03-24

## 1. 背景

当前系统在 [`internal/policy/postgres.go`](internal/policy/postgres.go) 中仅具备最小的 tenant-model allowlist 能力，且在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中只用于对 `AllowedModels(...)` 做简单过滤。这离设计文档 [`docs/plans/2026-03-09-llm-gateway-design.md:135`](docs/plans/2026-03-09-llm-gateway-design.md:135) 所定义的 Policy Engine 目标仍有明显差距。

本阶段目标是在不引入复杂策略 DSL 的前提下，落地一套**最小可用且可扩展**的 Policy Engine 增强方案，覆盖：

- tenant 级 RBAC 角色
- provider allow/deny 约束
- 敏感词策略骨架
- 管理接口的角色化权限控制
- 策略命中审计

## 2. 方案对比

### 方案 A：基于 PostgreSQL 的显式规则表（推荐）

继续沿用 [`internal/policy/postgres.go`](internal/policy/postgres.go) 的风格，为 RBAC、provider 约束、敏感词规则分别加独立表，并在请求流中做顺序执行。

优点：
- 与当前架构一致
- 实现简单、可解释性强
- 易于通过 `/admin` 接口管理

缺点：
- 表会变多
- 规则表达力有限

### 方案 B：统一 JSON Policy 文档

为每个 tenant 保存一份 JSON policy 文档，在运行时解释执行。

优点：
- 结构集中
- 后续扩展灵活

缺点：
- 当前阶段调试成本高
- 容易形成隐式 DSL

### 方案 C：代码内置规则 + 少量数据库配置

将大部分逻辑固化在代码中，仅把开关放到数据库。

优点：
- 实现快

缺点：
- 可维护性与可审计性差
- 不符合企业治理方向

### 结论

采用方案 A：**基于 PostgreSQL 的显式规则表。**

## 3. 数据模型与口径

### 3.1 RBAC

新增 tenant 级角色绑定表，最小角色集：

- `admin`
- `operator`
- `readonly`

首版不做复杂角色继承，只做显式角色判断。

### 3.2 Provider Allow / Deny

新增 tenant-provider 约束表：

- `tenant_id`
- `provider`
- `mode`（`allow` / `deny`）

命中规则后：
- 优先过滤请求中的 provider/model 候选
- 如无可用 provider/model，则返回 `policy_error`

### 3.3 敏感词规则骨架

新增 tenant-sensitive-rules 表：

- `tenant_id`
- `pattern`
- `enabled`
- `action`（首版仅支持 `block`）

首版仅做简单文本匹配，不引入正则优先级、上下文规则或复杂 DSL。

### 3.4 策略命中审计

所有关键策略命中都记录统一审计事件，至少覆盖：

- RBAC 拒绝
- provider deny / allowlist 排空
- 敏感词阻断

## 4. 请求流集成与接口边界

### 4.1 请求流顺序

在 [`/v1/chat/completions`](internal/httpserver/server.go:51) 前置链路中，执行顺序建议为：

1. 鉴权
2. RBAC
3. provider/model policy
4. 配额与限流
5. 缓存/路由/供应商调用

### 4.2 管理接口角色化

对 `/admin/*` 做最小角色权限控制：

- `readonly`：仅允许读接口
- `operator`：允许修改策略
- `admin`：全量能力

首版不做更细粒度资源级权限。

### 4.3 Provider 约束行为

provider allow/deny 命中后：

- 先过滤 `CandidateModels`
- 再校验 `PreferredModel`
- 若最终无可用 provider/model，则返回 `policy_error`

### 4.4 敏感词行为

对用户输入与必要上下文做简单文本匹配。
命中后：
- 直接阻断请求
- 返回策略错误
- 写入审计事件

## 5. 审计与测试策略

### 5.1 审计

所有策略命中统一进入审计链路，复用现有 [`/admin/audit`](internal/httpserver/server.go:54) 的读侧能力。

### 5.2 测试分层

#### 存储层测试

针对 [`internal/policy/postgres.go`](internal/policy/postgres.go)：
- 角色绑定表
- provider 规则表
- 敏感词规则表
- 基础查询与 upsert 行为

#### 请求流测试

针对 [`internal/httpserver/server.go`](internal/httpserver/server.go)：
- `readonly/operator/admin` 权限行为
- provider deny 导致请求阻断
- 敏感词命中阻断
- `policy_error` 返回结构

#### 远程联调

脚本验证：
- 角色权限差异
- provider 策略命中
- 敏感词规则阻断
- 审计事件可见

### 5.3 范围约束

首版不实现：
- 复杂策略 DSL
- 多条件组合表达式
- 角色继承树
- 模糊/AI 敏感内容识别

## 6. 风险与控制

### 风险 1：请求流前置阶段过重

控制方式：
- 首版策略判断只做简单表查询与文本匹配
- 不引入复杂规则解释器

### 风险 2：角色与管理接口权限耦合混乱

控制方式：
- 只定义三种角色
- 将写接口和读接口边界明确硬编码

### 风险 3：敏感词阻断误杀

控制方式：
- 首版仅 tenant 自定义规则
- 默认不开启任何内置词库
- 只支持简单 `block`

## 7. 成功标准

完成后应满足：

- tenant 可绑定最小角色集
- `/admin/*` 具备基础角色化权限差异
- tenant 可配置 provider allow/deny
- tenant 可配置敏感词阻断规则
- 命中策略时可返回明确的 `policy_error`
- 策略命中事件在审计链路中可见
