# Change Approval and Release Flow Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为控制面配置引入 Draft / Pending / Released 三态审批与发布流程，并与版本化、热更新、多实例同步自然衔接。

**Architecture:** 不做复杂多人审批，先围绕 [`draft`](docs/plans/2026-03-24-change-approval-and-release-flow-design.md)、[`pending`](docs/plans/2026-03-24-change-approval-and-release-flow-design.md)、[`released`](docs/plans/2026-03-24-change-approval-and-release-flow-design.md) 三态定义配置流转。只有 Released 版本才会更新生效指针并触发热更新 / 多实例同步；回滚通过生成新的 Released 版本完成，而不是改写旧记录。

**Tech Stack:** Go, PostgreSQL, net/http, existing control-plane config management, runtime reload, config versioning abstractions

---

## File Map

- Create: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
  - 定义 Draft / Pending / Released 状态模型与状态流转逻辑
- Create: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)
  - 覆盖状态流转、冻结、发布、回滚语义
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
  - 与 release state 衔接当前生效指针
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 增加 draft/pending/released 相关接口
- Create: [`internal/httpserver/release_flow_handler_test.go`](internal/httpserver/release_flow_handler_test.go)
  - 覆盖控制面审批与发布接口
- Create: [`cmd/verify/release_flow/main.go`](cmd/verify/release_flow/main.go)
  - 验证 Draft -> Pending -> Released -> Rollback 路径

---

## Chunk 1: 定义状态模型与流转语义

### Task 1: 创建 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)

**Files:**
- Create: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Test: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)

- [ ] **Step 1: Write the failing test**

在 [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go) 中先定义：

```go
type ReleaseState string

const (
    ReleaseStateDraft    ReleaseState = "draft"
    ReleaseStatePending  ReleaseState = "pending"
    ReleaseStateReleased ReleaseState = "released"
)
```

并测试这些状态值存在且稳定。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestReleaseStateShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中实现：
- [`ReleaseState`](internal/controlplane/release_flow.go)
- `ReleaseRecord`（至少带 `module`、`tenant_id`、`version`、`state`、`created_at`）

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestReleaseStateShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/release_flow_test.go
git commit -m "feat: add release flow state model"
```

### Task 2: 实现 Draft -> Pending -> Released 基本流转

**Files:**
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Test: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- Draft 可进入 Pending
- Pending 可进入 Released
- Draft 不可直接跳过 Pending 进入 Released

建议测试名：
- `TestReleaseFlowTransitions`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestReleaseFlowTransitions -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中实现：
- `SubmitForApproval(...)`
- `ApproveRelease(...)`
- 最小状态机校验

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestReleaseFlowTransitions -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/release_flow_test.go
git commit -m "feat: add draft pending released transitions"
```

### Task 3: 实现 Released 只读与回滚生成新版本语义

**Files:**
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- Released 版本不可直接编辑
- 回滚不会改写旧版本
- 回滚会生成新的 Released 版本记录或等价事件

建议测试名：
- `TestReleasedVersionIsImmutable`
- `TestRollbackCreatesNewReleasedVersion`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run 'Test(ReleasedVersionIsImmutable|RollbackCreatesNewReleasedVersion)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中：
- 禁止对 Released 做原地编辑
- 提供 `RollbackToVersion(...)`
- 与 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go) 的 active pointer 衔接

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run 'Test(ReleasedVersionIsImmutable|RollbackCreatesNewReleasedVersion)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/versioning.go internal/controlplane/release_flow_test.go
git commit -m "feat: add immutable release and rollback semantics"
```

---

## Chunk 2: 暴露控制面接口

### Task 4: 增加审批与发布接口

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/release_flow_handler_test.go`](internal/httpserver/release_flow_handler_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- 接口存在：
  - `/admin/control-plane/config/draft`
  - `/admin/control-plane/config/submit`
  - `/admin/control-plane/config/release`
- 不返回 404

建议测试名：
- `TestReleaseFlowRoutesRegistered`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestReleaseFlowRoutesRegistered -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中注册并创建 handler：
- `adminControlPlaneDraft`
- `adminControlPlaneSubmit`
- `adminControlPlaneRelease`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestReleaseFlowRoutesRegistered -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/release_flow_handler_test.go
git commit -m "feat: add release flow control plane routes"
```

### Task 5: 增加 rollback 接口与最小返回结构

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/release_flow_handler_test.go`](internal/httpserver/release_flow_handler_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- `/admin/control-plane/config/rollback` 存在
- 返回结构至少包含：
  - `module`
  - `tenant_id`
  - `from_version`
  - `to_version`
  - `state`

建议测试名：
- `TestReleaseFlowRollbackRouteAndShape`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestReleaseFlowRollbackRouteAndShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中增加 rollback handler 与统一返回结构。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestReleaseFlowRollbackRouteAndShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/release_flow_handler_test.go
 git commit -m "feat: add release flow rollback endpoint"
```

---

## Chunk 3: 增加验证程序

### Task 6: 新增 [`cmd/verify/release_flow/main.go`](cmd/verify/release_flow/main.go)

**Files:**
- Create: [`cmd/verify/release_flow/main.go`](cmd/verify/release_flow/main.go)

- [ ] **Step 1: Write the verification program**

程序至少打印：
- Draft 创建
- Submit 到 Pending
- Approve 到 Released
- Rollback 结果
- 当前 active pointer

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/release_flow`
Expected: 输出 Draft / Pending / Released / Rollback 路径。

- [ ] **Step 3: Minimal fixes if needed**

只修复验证所需问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/release_flow/main.go internal/controlplane internal/httpserver
 git commit -m "test: add release flow verification tool"
```

---

## Chunk 4: 回归验证

### Task 7: 执行本地回归

**Files:**
- No new files

- [ ] **Step 1: Run controlplane tests**

Run: `go test ./internal/controlplane -v`
Expected: PASS

- [ ] **Step 2: Run httpserver tests**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 3: Run verify package tests**

Run: `go test ./cmd/verify/release_flow`
Expected: PASS or `[no test files]`

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize change approval and release flow"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-change-approval-and-release-flow.md`. Ready to execute?
