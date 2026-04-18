# Policy Engine 深化闭环设计文档

日期：2026-03-24

## 1. 背景

当前 Policy Engine 已经完成两轮推进：

- 第一轮增强已完成 tenant 级 RBAC / provider allow-deny / sensitive rules 的存储层与最小请求流骨架
- 第二轮深化已接入基础 `/admin/*` 权限辅助判定、provider deny / sensitive rule 前置骨架，以及策略命中审计骨架

但目前仍然属于“骨架可用、闭环未完成”的状态，关键缺口集中在：

- `/admin/*` 角色权限尚未真正成为强校验闭环
- provider deny 尚未形成真实的 provider/model 过滤闭环
- sensitive rule 还没有完成真实远程联调级验证
- 审计链路虽已接入骨架，但仍需通过远程链路证明策略命中可读可验

本阶段目标是在不引入复杂策略 DSL 的前提下，把这些能力推进到**真正可执行、可拒绝、可审计、可联调**。

## 2. 方案对比

### 方案 A：最小闭环版（推荐）

优先完成：
- `/admin/*` 角色权限强校验闭环
- provider deny 闭环
- sensitive rule 与 provider deny 的远程联调验证
- 审计命中验证

优点：
- 直接补齐最关键的安全和治理缺口
- 不扩大规则表达复杂度
- 与现有实现最兼容

缺点：
- 仍然保留较简单的角色模型和规则表达方式

### 方案 B：并行闭环版

角色权限、provider deny、sensitive rule、审计与远程联调同时全面细化。

优点：
- 一次性补齐更多能力

缺点：
- 风险较高
- 更容易把当前阶段从“闭环补齐”扩展成“大重构”

### 方案 C：联调优先版

先补远程联调和验证，再回头把权限与阻断逻辑补成强闭环。

优点：
- 快速看到行为

缺点：
- 如果核心权限逻辑不稳，联调结果不可靠

### 结论

采用方案 A：**最小闭环版**。

## 3. `/admin/*` 角色权限强校验闭环

### 3.1 基本原则

继续保留 [`X-Admin-Key`](internal/httpserver/server.go) 作为管理面基础认证，但不再把它当作唯一放行条件。

进入 `/admin/*` 后，必须结合：

- `tenant_id`
- `subject`（可由 `Authorization: Bearer <subject>` 或辅助 header 取出）
- tenant 级角色绑定

来决定是否允许访问。

### 3.2 角色模型

首版继续使用三种角色：

- `admin`
- `operator`
- `readonly`

不做角色继承，不做复杂资源级 ACL。

### 3.3 权限边界

首版不按每个接口逐个定义，而按三档分类：

1. **读接口**
   - `readonly` / `operator` / `admin` 全可访问
2. **策略与观测写接口**
   - `operator` / `admin` 可访问
3. **高敏管理员接口**
   - 仅 `admin` 可访问

### 3.4 错误与审计

当角色缺失或权限不足时：

- 返回 `policy_error` 或 `authorization_error`
- 统一写入审计事件
- 审计至少包含：
  - `tenant_id`
  - `subject`
  - `role`
  - 请求路径
  - 失败原因

## 4. provider deny 与敏感词远程联调闭环

### 4.1 provider deny 闭环

provider deny 不再只对 `PreferredModel` 做占位判断，而是要真正影响候选路径。

执行逻辑：

1. 读取 tenant 的 provider policy
2. 对命中 `deny` 的 provider，从 provider/model 候选集合中剔除
3. 若剔除后无可用 provider/model，则立即返回 `policy_error`
4. 同时写入策略命中审计

### 4.2 与 allowlist 的顺序关系

请求前置阶段顺序建议固定为：

1. model allowlist
2. provider allow/deny
3. quota
4. cache / routing / provider call

确保 deny 的语义不会被后续路由绕过。

### 4.3 敏感词阻断闭环

敏感词规则保持简单字符串匹配，不引入复杂 DSL。

行为要求：

- 对请求消息内容执行 tenant 自定义词匹配
- 命中后立即阻断
- 返回 `policy_error`
- 响应体至少包含：
  - `tenant_id`
  - `pattern`
  - 固定阻断原因
- 同时写审计事件

### 4.4 远程联调验证目标

远程联调至少覆盖：

- 角色权限拒绝
- provider deny 拒绝
- sensitive rule block 拒绝
- 命中审计可通过现有 `/admin/audit` 读出

## 5. 测试与审计验证策略

### 5.1 本地测试

#### [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

补齐：

- `/admin/*` 角色权限强校验差异
- provider deny 闭环
- sensitive rule block
- 审计写入行为

### 5.2 远程联调

通过 [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go) 完整验证：

- readonly/operator/admin 的接口访问差异
- provider deny 命中行为
- sensitive rule 命中行为
- `/admin/audit` 中策略命中事件存在

### 5.3 审计范围

首版只要求命中事件可见，不要求新的专用审计表或可视化报表。

## 6. 风险与控制

### 风险 1：角色权限收紧导致现有管理链路不可用

控制方式：
- 先按三档接口分类，不做复杂矩阵
- 在本地测试中覆盖所有核心 `/admin/*` 路径

### 风险 2：provider deny 与现有路由策略冲突

控制方式：
- 明确 provider deny 在路由前生效
- deny 后无可用候选时直接报错，不允许下游路由“补回”

### 风险 3：敏感词误杀

控制方式：
- 仅 tenant 自定义规则
- 默认不启用内置词库
- 首版只做 block，不做自动修复或替换

## 7. 成功标准

完成后应满足：

- `/admin/*` 角色权限不再只是骨架，而是真正可阻断
- provider deny 形成真实闭环
- sensitive rule 可通过远程联调真实阻断请求
- 策略命中事件能在现有审计链路中读取
- 本地测试与远程联调都能证明上述行为成立
