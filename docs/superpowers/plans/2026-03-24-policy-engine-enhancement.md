# Policy Engine Enhancement Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为网关落地最小可用但可扩展的 Policy Engine 增强，包括 tenant 级 RBAC、provider allow/deny、敏感词规则骨架、角色化 `/admin/*` 权限与策略命中审计。

**Architecture:** 在 [`internal/policy/postgres.go`](internal/policy/postgres.go) 的 PostgreSQL 存储层上扩展显式规则表，并在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 的请求前置链路中按“鉴权 -> RBAC -> provider policy -> quota -> cache/routing”顺序执行。审计复用现有 [`internal/audit`](internal/audit) 链路，不引入复杂 DSL 或策略解释器。

**Tech Stack:** Go, PostgreSQL, net/http, existing admin auth middleware, existing audit store, existing policy store

---

## File Map

- Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
  - 扩展 tenant-role、tenant-provider-policy、tenant-sensitive-rules 三类表
  - 新增查询与 upsert 方法
- Create: [`internal/policy/postgres_test.go`](internal/policy/postgres_test.go)
  - 覆盖 schema、upsert、query 行为
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 将 Policy Engine 接入 [`/v1/chat/completions`](internal/httpserver/server.go:51)
  - 对 `/admin/*` 做角色化权限控制
  - 增加敏感词阻断与 provider policy 拦截
  - 写策略命中审计
- Create: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)
  - 覆盖 RBAC、provider deny、敏感词阻断与审计行为
- Create: [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)
  - 远程联调角色权限 / provider deny / sensitive rule 命中
- Optional Modify: [`docs/plans/2026-03-24-policy-engine-enhancement-design.md`](docs/plans/2026-03-24-policy-engine-enhancement-design.md)
  - 补记实现备注（可选）

---

## Chunk 1: 扩展策略存储层

### Task 1: 为 [`internal/policy/postgres.go`](internal/policy/postgres.go) 增加 tenant-role 模型

**Files:**
- Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
- Test: [`internal/policy/postgres_test.go`](internal/policy/postgres_test.go)

- [ ] **Step 1: 写失败测试，约束角色绑定结构与方法存在**

新增测试，先固定最小角色结构：

```go
type TenantRoleBinding struct {
    TenantID string `json:"tenant_id"`
    Subject  string `json:"subject"`
    Role     string `json:"role"`
}
```

并断言会存在以下方法：
- `UpsertRole(...)`
- `RoleFor(...)`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/policy -run TestRoleBindingShape -v`
Expected: FAIL，提示类型或方法不存在。

- [ ] **Step 3: 写最小实现**

在 [`internal/policy/postgres.go`](internal/policy/postgres.go) 中：
- 新增 `tenant_role_bindings` 表
- 新增结构体与方法
- 限定角色值只接受：`admin`、`operator`、`readonly`

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/policy -run TestRoleBindingShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/policy/postgres.go internal/policy/postgres_test.go
git commit -m "feat: add tenant role bindings"
```

### Task 2: 增加 provider allow/deny 约束表与方法

**Files:**
- Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
- Test: [`internal/policy/postgres_test.go`](internal/policy/postgres_test.go)

- [ ] **Step 1: 写失败测试**

测试目标：
- 存在 `TenantProviderPolicy` 结构
- 支持 `allow` / `deny`
- 支持 `UpsertProviderPolicy(...)`
- 支持 `ProviderPolicies(...)`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/policy -run TestProviderPolicyShape -v`
Expected: FAIL

- [ ] **Step 3: 写最小实现**

在 [`internal/policy/postgres.go`](internal/policy/postgres.go) 中：
- 新增 `tenant_provider_policies` 表
- 新增结构与方法
- `mode` 只允许 `allow` / `deny`

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/policy -run TestProviderPolicyShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/policy/postgres.go internal/policy/postgres_test.go
git commit -m "feat: add tenant provider policies"
```

### Task 3: 增加敏感词规则骨架与方法

**Files:**
- Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
- Test: [`internal/policy/postgres_test.go`](internal/policy/postgres_test.go)

- [ ] **Step 1: 写失败测试**

测试目标：
- 存在 `SensitiveRule` 结构
- 支持 `UpsertSensitiveRule(...)`
- 支持 `SensitiveRules(...)`
- `action` 首版只允许 `block`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/policy -run TestSensitiveRuleShape -v`
Expected: FAIL

- [ ] **Step 3: 写最小实现**

在 [`internal/policy/postgres.go`](internal/policy/postgres.go) 中：
- 新增 `tenant_sensitive_rules` 表
- 新增结构与查询/upsert 方法
- 不实现复杂正则，只存字符串 pattern

- [ ] **Step 4: 运行 policy 包全量测试**

Run: `go test ./internal/policy -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/policy/postgres.go internal/policy/postgres_test.go
git commit -m "feat: add sensitive rule policy storage"
```

---

## Chunk 2: 将 Policy Engine 接入请求流

### Task 4: 在 [`/v1/chat/completions`](internal/httpserver/server.go:51) 中接入 provider allow/deny 与敏感词阻断

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 provider deny**

新增测试：
- 当 tenant 命中 provider deny 导致无可用 provider/model 时，返回 `policy_error`

建议测试名：
- `TestPolicyEngine_BlocksDeniedProvider`

- [ ] **Step 2: 写失败测试，覆盖敏感词阻断**

新增测试：
- 当请求消息命中敏感词时，返回阻断错误

建议测试名：
- `TestPolicyEngine_BlocksSensitiveContent`

- [ ] **Step 3: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(BlocksDeniedProvider|BlocksSensitiveContent)' -v`
Expected: FAIL

- [ ] **Step 4: 写最小实现**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 在 quota 前置阶段插入 policy 检查
- provider deny 命中后：
  - 过滤 provider/model
  - 无剩余候选时返回 `policy_error`
- 敏感词命中后：
  - 直接阻断
  - 返回策略错误

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(BlocksDeniedProvider|BlocksSensitiveContent)' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
 git commit -m "feat: enforce provider and sensitive content policies"
```

### Task 5: 为 `/admin/*` 增加角色化权限控制

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 admin/operator/readonly 权限差异**

测试目标：
- `readonly` 只能 GET
- `operator` 可修改策略
- `admin` 拥有全量能力

建议测试名：
- `TestPolicyEngine_AdminRolePermissions`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run TestPolicyEngine_AdminRolePermissions -v`
Expected: FAIL

- [ ] **Step 3: 写最小实现**

在 [`requireAdmin()`](internal/httpserver/server.go:71) 或其周边新增角色检查逻辑：
- 基于 tenant + subject 查询角色
- 对写接口限制 `operator/admin`
- 对高敏接口限制 `admin`

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/httpserver -run TestPolicyEngine_AdminRolePermissions -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
 git commit -m "feat: add role-based admin endpoint permissions"
```

---

## Chunk 3: 增加策略命中审计

### Task 6: 复用 [`internal/audit`](internal/audit) 写策略命中审计

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，验证策略命中会产生审计事件**

至少覆盖：
- provider deny 命中
- sensitive rule 命中
- RBAC 拒绝

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run TestPolicyEngine_WritesAuditOnPolicyHit -v`
Expected: FAIL

- [ ] **Step 3: 写最小实现**

复用现有 [`writeAuditAsync(...)`](internal/httpserver/server.go) 机制：
- 为 `policy_error` 写明确事件类型/摘要
- 审计载荷至少包含：`tenant_id`、命中规则类型、命中值、请求路径

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/httpserver -run TestPolicyEngine_WritesAuditOnPolicyHit -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
 git commit -m "feat: audit policy hits"
```

---

## Chunk 4: 远程联调与回归

### Task 7: 新增 [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)

**Files:**
- Create: [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)

- [ ] **Step 1: 写远程联调脚本**

脚本应覆盖：
- admin/operator/readonly 角色差异
- provider deny 命中
- sensitive rule block 命中
- `/admin/audit` 可读到策略命中记录（若环境可用）

- [ ] **Step 2: 运行脚本并确认输出**

Run: `go run ./cmd/verify/policy_engine`
Expected: 打印各策略路径的响应摘要。

- [ ] **Step 3: 如有失败，做最小修复**

只修复与联调目标直接相关的问题。

- [ ] **Step 4: 执行全量回归**

Run:
- `go test ./internal/policy -v`
- `go test ./internal/httpserver -v`
- `go test ./cmd/server ./cmd/verify/policy_engine`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/policy/postgres.go internal/policy/postgres_test.go internal/httpserver/server.go internal/httpserver/policy_handler_test.go cmd/verify/policy_engine/main.go
 git commit -m "feat: add policy engine enhancements"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-policy-engine-enhancement.md`. Ready to execute?
