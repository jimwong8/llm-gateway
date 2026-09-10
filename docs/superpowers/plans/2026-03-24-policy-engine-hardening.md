# Policy Engine Hardening Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐 Policy Engine 的真实权限与阻断闭环，使 `/admin/*` 角色控制、provider deny 阻断、敏感词阻断与策略命中审计都能在本地和远程真实验证通过。

**Architecture:** 在现有 [`internal/policy/postgres.go`](internal/policy/postgres.go) 与 [`internal/httpserver/server.go`](internal/httpserver/server.go) 的基础上，不引入新 DSL 或新审计存储，而是把已有骨架变成真正生效的前置链路。核心思路是：请求入口先执行角色判定和策略过滤，命中后通过现有 [`writeAuditAsync()`](internal/httpserver/server.go:1046) 复用审计链路，并使用远程 verify 程序闭环验证。

**Tech Stack:** Go, PostgreSQL, net/http, existing audit store, existing policy store, existing admin auth middleware

---

## File Map

- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 真正启用 `/admin/*` 角色权限判定
  - 完成 provider deny 闭环
  - 完成敏感词阻断链路
  - 写策略命中审计
- Modify: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)
  - 补齐角色权限、provider deny、敏感词阻断与审计测试
- Modify: [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)
  - 增强远程联调脚本
- Optional Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
  - 若实际接入时需要增加轻量辅助查询方法，可最小补充

---

## Chunk 1: 让 `/admin/*` 角色权限真正生效

### Task 1: 为 [`/admin/*`](internal/httpserver/server.go:52) 写角色权限失败测试

**Files:**
- Modify: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 readonly 只读限制**

新增测试：
- `readonly` 请求 `GET /admin/usage` 应允许
- `readonly` 请求 `POST /admin/policies/models` 应拒绝

建议测试名：
- `TestPolicyEngine_ReadonlyCannotWriteAdminEndpoints`

- [ ] **Step 2: 写失败测试，覆盖 operator/admin 差异**

新增测试：
- `operator` 可写策略类接口
- `admin` 仍具备全量访问

建议测试名：
- `TestPolicyEngine_AdminRolePermissions`

- [ ] **Step 3: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(ReadonlyCannotWriteAdminEndpoints|AdminRolePermissions)' -v`
Expected: FAIL

- [ ] **Step 4: 实现最小角色判定**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 扩展 [`requireAdmin()`](internal/httpserver/server.go:73) 或新增包装器
- 从 `Authorization: Bearer <subject>` 或 [`currentSubject()`](internal/httpserver/server.go) 获取主体
- 基于 tenant + subject 查询角色
- 对 `/admin/*` 写接口应用 `readonly/operator/admin` 规则

- [ ] **Step 5: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(ReadonlyCannotWriteAdminEndpoints|AdminRolePermissions)' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
git commit -m "feat: enforce admin role permissions"
```

---

## Chunk 2: 补齐 provider deny 闭环

### Task 2: 为 provider deny 真正阻断写失败测试

**Files:**
- Modify: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 deny 后无可用 provider**

新增测试：
- tenant 命中 provider deny 后，请求被阻断
- 返回 `policy_error`
- 错误体包含 `tenant_id` 与被拒 provider 信息

建议测试名：
- `TestPolicyEngine_BlocksDeniedProvider`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run TestPolicyEngine_BlocksDeniedProvider -v`
Expected: FAIL

- [ ] **Step 3: 实现最小闭环**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 读取 [`ProviderPolicies()`](internal/policy/postgres.go)
- 对 `deny` provider 执行过滤
- 如果 deny 后没有可用 provider/model：
  - 直接返回 `policy_error`
  - 写策略命中审计

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run TestPolicyEngine_BlocksDeniedProvider -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
git commit -m "feat: enforce provider deny policy"
```

---

## Chunk 3: 完成敏感词阻断与审计验证

### Task 3: 为敏感词阻断写失败测试

**Files:**
- Modify: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 sensitive rule block**

新增测试：
- 消息命中敏感词时返回 `policy_error`
- 错误中包含 `pattern`

建议测试名：
- `TestPolicyEngine_BlocksSensitiveContent`

- [ ] **Step 2: 写失败测试，覆盖策略命中审计**

新增测试：
- provider deny、RBAC reject、sensitive block 三类路径都调用审计写入

建议测试名：
- `TestPolicyEngine_WritesAuditOnPolicyHit`

- [ ] **Step 3: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(BlocksSensitiveContent|WritesAuditOnPolicyHit)' -v`
Expected: FAIL

- [ ] **Step 4: 实现最小阻断与审计逻辑**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 使用 [`containsSensitive()`](internal/httpserver/server.go) 做简单字符串匹配
- 命中后返回阻断错误
- 通过 [`writeAuditAsync()`](internal/httpserver/server.go:1046) 写策略命中事件

- [ ] **Step 5: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(BlocksSensitiveContent|WritesAuditOnPolicyHit)' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
git commit -m "feat: audit policy hits and block sensitive content"
```

---

## Chunk 4: 增强远程联调脚本

### Task 4: 扩展 [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)

**Files:**
- Modify: [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)

- [ ] **Step 1: 增加角色权限验证路径**

脚本至少打印：
- readonly 访问读接口
- readonly 访问写接口
- operator/admin 访问策略写接口

- [ ] **Step 2: 增加 provider deny 与 sensitive rule 场景**

脚本打印：
- provider deny 命中结果
- sensitive rule block 命中结果

- [ ] **Step 3: 增加 `/admin/audit` 检查**

在环境支持时，打印最近审计事件，确认策略命中已落审计链路。

- [ ] **Step 4: 运行脚本并确认输出完整**

Run: `go run ./cmd/verify/policy_engine`
Expected: 输出角色权限差异、provider deny、sensitive block、audit 结果。

- [ ] **Step 5: Commit**

```bash
git add cmd/verify/policy_engine/main.go
 git commit -m "test: add policy engine hardening verification"
```

---

## Chunk 5: 回归验证

### Task 5: 执行本地与远程回归

**Files:**
- No new files

- [ ] **Step 1: 本地回归**

Run:
- `go test ./internal/policy -v`
- `go test ./internal/httpserver -v`
- `go test ./cmd/verify/policy_engine`

Expected: PASS

- [ ] **Step 2: 远程同步并回归**

Run（远程）：
- `go test ./internal/policy ./internal/httpserver ./cmd/verify/policy_engine`

Expected: PASS

- [ ] **Step 3: 如环境允许，远程联调**

执行 [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go) 并确认策略链路输出。

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize policy engine hardening"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-policy-engine-hardening.md`. Ready to execute?
