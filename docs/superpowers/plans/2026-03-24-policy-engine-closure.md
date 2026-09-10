# Policy Engine Closure Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Policy Engine 从“骨架可用”推进到真正闭环可执行，完成 `/admin/*` 角色权限强校验、provider deny 真实过滤与阻断、敏感词阻断远程联调和策略命中审计验证。

**Architecture:** 继续沿用 [`internal/policy/postgres.go`](internal/policy/postgres.go) 的显式规则表与 [`internal/httpserver/server.go`](internal/httpserver/server.go) 的前置请求链路，不新增 DSL 或新审计存储。通过对现有辅助函数和路由链路进行最小但真实的强化，使规则判断从“占位”变为“可阻断、可审计、可远程验证”。

**Tech Stack:** Go, PostgreSQL, net/http, existing audit store, existing policy store, existing verify commands

---

## File Map

- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 强化 `/admin/*` 角色权限校验
  - 实现 provider deny 的真实过滤与阻断
  - 实现敏感词阻断的真实闭环
  - 写策略命中审计
- Modify: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)
  - 增加角色权限、provider deny、敏感词与审计的真实行为测试
- Modify: [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)
  - 增强远程联调脚本，覆盖角色权限、provider deny、敏感词阻断与 `/admin/audit`
- Optional Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
  - 若需要补充轻量辅助方法，再做最小修改

---

## Chunk 1: `/admin/*` 角色权限强校验闭环

### Task 1: 为 `/admin/*` 读写差异写失败测试

**Files:**
- Modify: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 readonly 只能读**

新增测试目标：
- `readonly` 访问 `GET /admin/usage` 允许
- `readonly` 访问写接口（例如 `POST /admin/policies/models`）拒绝

建议测试名：
- `TestPolicyEngine_ReadonlyCanOnlyReadAdminEndpoints`

- [ ] **Step 2: 写失败测试，覆盖 operator/admin 差异**

新增测试目标：
- `operator` 可写策略类接口
- `admin` 全量可访问

建议测试名：
- `TestPolicyEngine_OperatorAndAdminPermissions`

- [ ] **Step 3: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(ReadonlyCanOnlyReadAdminEndpoints|OperatorAndAdminPermissions)' -v`
Expected: FAIL

- [ ] **Step 4: 最小实现 `/admin/*` 强校验**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 保留 [`X-Admin-Key`](internal/httpserver/server.go) 作为基础认证
- 在 `requireAdmin` 之后补充角色校验逻辑
- 通过 `tenant_id + subject` 查询角色
- 按“读接口 / 策略与观测写接口 / 高敏接口”三档应用权限规则

- [ ] **Step 5: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(ReadonlyCanOnlyReadAdminEndpoints|OperatorAndAdminPermissions)' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
git commit -m "feat: enforce admin endpoint role permissions"
```

---

## Chunk 2: provider deny 真实过滤与阻断闭环

### Task 2: 为 provider deny 过滤链路写失败测试

**Files:**
- Modify: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 deny 后集合为空时阻断**

测试目标：
- tenant 配置 deny provider 后，请求无法继续走 provider 选择
- 返回 `policy_error`
- 审计被写入

建议测试名：
- `TestPolicyEngine_DeniedProviderBlocksRequest`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run TestPolicyEngine_DeniedProviderBlocksRequest -v`
Expected: FAIL

- [ ] **Step 3: 最小实现真实过滤**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- provider policy 不再只检查 `PreferredModel`
- 对候选 provider/model 做真实过滤
- deny 后若无可用候选：
  - 返回 `policy_error`
  - 写审计事件

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run TestPolicyEngine_DeniedProviderBlocksRequest -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
git commit -m "feat: enforce provider deny closure"
```

---

## Chunk 3: 敏感词阻断与策略命中审计验证

### Task 3: 为敏感词阻断与审计写失败测试

**Files:**
- Modify: [`internal/httpserver/policy_handler_test.go`](internal/httpserver/policy_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 sensitive rule 阻断**

测试目标：
- 命中敏感词后返回 `policy_error`
- 响应体包含 `pattern`

建议测试名：
- `TestPolicyEngine_SensitiveRuleBlocksRequest`

- [ ] **Step 2: 写失败测试，覆盖命中审计**

测试目标：
- provider deny / role reject / sensitive block 至少其一触发时，调用审计写入

建议测试名：
- `TestPolicyEngine_AuditsPolicyHits`

- [ ] **Step 3: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(SensitiveRuleBlocksRequest|AuditsPolicyHits)' -v`
Expected: FAIL

- [ ] **Step 4: 最小实现真实阻断与审计闭环**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 保持简单字符串匹配
- 命中后立即返回阻断错误
- 审计中写入：
  - `tenant_id`
  - `policy`
  - `pattern` 或 `provider` 或 `role`
  - 请求路径

- [ ] **Step 5: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run 'TestPolicyEngine_(SensitiveRuleBlocksRequest|AuditsPolicyHits)' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/policy_handler_test.go
git commit -m "feat: complete sensitive rule blocking and audit flow"
```

---

## Chunk 4: 远程联调脚本增强

### Task 4: 增强 [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)

**Files:**
- Modify: [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go)

- [ ] **Step 1: 增加 `/admin/*` 角色权限验证输出**

脚本打印：
- readonly 访问读接口
- readonly 访问写接口
- operator/admin 访问策略接口

- [ ] **Step 2: 增加 provider deny 与敏感词阻断输出**

脚本打印：
- provider deny 命中结果
- sensitive rule 命中结果

- [ ] **Step 3: 增加 `/admin/audit` 检查输出**

脚本打印最近策略命中审计。

- [ ] **Step 4: 运行脚本并确认输出不完整**

Run: `go run ./cmd/verify/policy_engine`
Expected: 初始可能缺少完整阻断/审计验证输出。

- [ ] **Step 5: 最小修复脚本或服务端逻辑**

只修复与联调目标直接相关的问题。

- [ ] **Step 6: 运行脚本确认通过**

Run: `go run ./cmd/verify/policy_engine`
Expected: 能看到角色权限差异、provider deny、sensitive block、audit 结果。

- [ ] **Step 7: Commit**

```bash
git add cmd/verify/policy_engine/main.go internal/httpserver/server.go internal/httpserver/policy_handler_test.go
git commit -m "test: add policy engine closure verification"
```

---

## Chunk 5: 本地与远程回归

### Task 5: 执行回归验证

**Files:**
- No new files

- [ ] **Step 1: 本地回归**

Run:
- `go test ./internal/policy -v`
- `go test ./internal/httpserver -v`
- `go test ./cmd/verify/policy_engine`

Expected: PASS

- [ ] **Step 2: 同步远程并回归**

Run（远程）：
- `go test ./internal/policy ./internal/httpserver ./cmd/verify/policy_engine`

Expected: PASS

- [ ] **Step 3: 远程联调**

执行 [`cmd/verify/policy_engine/main.go`](cmd/verify/policy_engine/main.go) 并核对输出。

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize policy engine closure"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-policy-engine-closure.md`. Ready to execute?
