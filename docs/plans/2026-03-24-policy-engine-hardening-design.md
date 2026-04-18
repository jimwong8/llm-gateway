# Policy Engine 深化设计文档

日期：2026-03-24

## 1. 背景

当前第一轮 Policy Engine 增强已经完成：

- [`internal/policy/postgres.go`](internal/policy/postgres.go) 已具备 tenant 级 RBAC / provider allow-deny / sensitive rules 的基础存储结构
- [`internal/httpserver/server.go`](internal/httpserver/server.go) 已接入最小前置链路骨架
- [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go) 与 [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go) 已有最小测试与联调占位

但仍存在明显缺口：

- `/admin/*` 角色权限尚未真正生效
- provider deny 还未形成完整闭环
- 敏感词阻断尚未完成远程联调级验证
- 策略命中审计还未完成可读可验闭环

本阶段目标是在不扩大模型复杂度的前提下，把这些关键链路补齐到“可真实运行、可测试、可审计”的状态。

## 2. 方案对比

### 方案 A：最小深化版（推荐）

优先完成：
- `/admin/*` 真正角色权限判定
- provider deny 闭环
- 敏感词阻断联调
- 策略命中审计验证

优点：
- 与当前落地程度最匹配
- 直接补齐真正影响企业治理闭环的缺口
- 不需要再引入新表或新 DSL

缺点：
- 仍然是“最小规则引擎”，表达力有限

### 方案 B：完整深化版

在方案 A 基础上，再进一步细化角色矩阵、按接口级资源划分权限、加入更多审计维度。

优点：
- 企业治理能力更完整

缺点：
- 实现复杂度显著增加
- 超出当前阶段“先补关键闭环”的目标

### 方案 C：审计优先版

先把策略命中全部审计通，再回头做权限与阻断闭环。

优点：
- 容易快速验证

缺点：
- 不能先解决实际拦截与权限缺口

### 结论

采用方案 A：**最小深化版**。

## 3. 角色权限与 provider deny 闭环

### 3.1 `/admin/*` 角色权限

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中，把当前角色辅助函数升级为真正参与 `/admin/*` 判定。

建议最小规则：

- `readonly`：仅允许 `GET`
- `operator`：允许读接口，允许策略与观测类写操作
- `admin`：全量能力

首版不做资源级 ACL，不做角色继承。

### 3.2 provider deny 闭环

provider deny 不再只是对 `PreferredModel` 的占位判断，而是要真正影响请求前置链路：

- 先读取 tenant 的 provider policies
- 对命中 `deny` 的 provider，从请求候选中剔除
- 若最终无可用 provider/model，则直接返回 `policy_error`
- 同时写入策略命中审计

### 3.3 与现有 allowlist 的关系

当前已有 [`AllowedModels()`](internal/policy/postgres.go) 的 tenant-model allowlist。深化后应保持顺序明确：

1. 先应用 model allowlist
2. 再应用 provider allow/deny
3. 最后检查 `PreferredModel` 是否仍可用

## 4. 敏感词阻断联调与策略命中审计验证

### 4.1 敏感词阻断

首版仍保持简单字符串匹配，不引入复杂规则表达式。

行为要求：

- 命中后直接阻断 `/v1/chat/completions`
- 返回 `policy_error`
- 错误中至少包含：
  - `tenant_id`
  - `pattern`
  - 固定 message（如 `sensitive content blocked`）

### 4.2 策略命中审计

复用现有 [`writeAuditAsync()`](internal/httpserver/server.go:1046) 链路。

至少覆盖：

- RBAC 拒绝
- provider deny 命中
- sensitive rule block 命中

审计载荷建议至少包含：

- `tenant_id`
- `policy` 类型
- 命中值（如 role/provider/pattern）
- 请求路径
- 请求标识 `request_id`

### 4.3 可见性要求

首版不额外设计专用审计表或新 UI，直接要求命中事件能通过现有 [`/admin/audit`](internal/httpserver/server.go:54) 读到。

## 5. 测试与联调策略

### 5.1 本地测试

#### [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

补齐：

- `/admin/*` 角色权限差异测试
- provider deny 闭环测试
- sensitive rule block 测试
- 策略命中审计调用测试（可用骨架/替身断言）

#### [`internal/policy/postgres_test.go`](internal/policy/postgres_test.go)

补齐：

- role/provider/rule 查询与 upsert 边界
- 无匹配时的返回行为
- 非法 role / 非法 mode / 非法 action 的拒绝行为

### 5.2 远程联调

通过 [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go) 验证：

- `readonly/operator/admin` 的管理接口差异
- provider deny 导致请求被阻断
- sensitive rule 命中并返回 `policy_error`
- `/admin/audit` 可读到策略命中事件（若环境可用）

## 6. 风险与控制

### 风险 1：角色权限误封正常管理接口

控制方式：
- 只先区分 `GET` 与写操作
- 不在首版做过细的接口矩阵

### 风险 2：provider deny 与 model allowlist 交叉后导致空集

控制方式：
- 明确返回 `policy_error`
- 错误响应中带上 `tenant_id` 与命中 provider/model 信息

### 风险 3：敏感词误杀

控制方式：
- 仅 tenant 自定义规则
- 默认不加载内置词库
- 首版仅 block，不做自动替换或重写

## 7. 成功标准

完成后应满足：

- `/admin/*` 角色权限在请求入口处真实生效
- provider deny 可以形成实际阻断闭环
- sensitive rules 可在真实请求链路中触发阻断
- 关键策略命中可通过现有审计链路读取
- 本地测试与远程联调可证明这些路径真实可用
