# Instance Discovery and State Aggregation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为实例发现与多实例状态聚合建立最小可用实现，定义实例身份、心跳状态模型和当前快照聚合视图，并与现有运行时热更新/消息总线抽象衔接。

**Architecture:** 首版不绑定 Consul、etcd、Kubernetes 等具体发现组件，而是在当前代码中先引入“实例身份 + 心跳状态 + 当前快照聚合”模型。实例状态只表达最新可见状态，不承载历史时间线或调度职责，并与 [`internal/runtime/reload.go`](internal/runtime/reload.go) 中的 reload 状态自然组合。

**Tech Stack:** Go, net/http, in-memory state, existing runtime bus/reload abstractions

---

## File Map

- Create: [`internal/runtime/instance_state.go`](internal/runtime/instance_state.go)
  - 定义实例身份、心跳与状态结构
- Create: [`internal/runtime/instance_state_test.go`](internal/runtime/instance_state_test.go)
  - 验证实例状态结构与聚合行为
- Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
  - 如有必要，将 reload 状态嵌入实例状态视图
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 暴露实例状态只读接口，例如 `/admin/runtime/instances`
- Create: [`internal/httpserver/instance_state_handler_test.go`](internal/httpserver/instance_state_handler_test.go)
  - 验证实例状态接口存在与返回结构
- Create: [`cmd/verify/instance_state/main.go`](cmd/verify/instance_state/main.go)
  - 输出实例状态快照的最小验证程序

---

## Chunk 1: 定义实例身份与状态结构

### Task 1: 创建 [`internal/runtime/instance_state.go`](internal/runtime/instance_state.go)

**Files:**
- Create: [`internal/runtime/instance_state.go`](internal/runtime/instance_state.go)
- Test: [`internal/runtime/instance_state_test.go`](internal/runtime/instance_state_test.go)

- [ ] **Step 1: 写失败测试，定义实例身份结构**

测试先固定目标结构，例如：

```go
type InstanceIdentity struct {
    InstanceID string    `json:"instance_id"`
    StartedAt  time.Time `json:"started_at"`
    Version    string    `json:"version"`
    Address    string    `json:"address"`
}
```

并断言：
- `InstanceID`、`Version`、`Address` 不为空时即视为有效身份

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestInstanceIdentityShape -v`
Expected: FAIL，提示结构不存在。

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/instance_state.go`](internal/runtime/instance_state.go) 中实现：
- [`InstanceIdentity`](internal/runtime/instance_state.go)
- [`InstanceStatus`](internal/runtime/instance_state.go)

建议 `InstanceStatus` 至少包含：
- `instance_id`
- `last_seen_at`
- `health`
- `role`
- `reload_status`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestInstanceIdentityShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/instance_state.go internal/runtime/instance_state_test.go
git commit -m "feat: add instance identity and status model"
```

### Task 2: 增加实例状态管理器

**Files:**
- Modify: [`internal/runtime/instance_state.go`](internal/runtime/instance_state.go)
- Test: [`internal/runtime/instance_state_test.go`](internal/runtime/instance_state_test.go)

- [ ] **Step 1: 写失败测试，验证状态更新与去重**

测试目标：
- 相同 `instance_id` 的心跳会覆盖旧状态
- `List()` 返回当前快照
- `last_seen_at` 更新正确

建议测试名：
- `TestInstanceStateManagerUpsertsByInstanceID`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestInstanceStateManagerUpsertsByInstanceID -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/instance_state.go`](internal/runtime/instance_state.go) 中增加：
- `Manager` 或 `InstanceRegistry`
- `Upsert(status InstanceStatus)`
- `List() []InstanceStatus`

保持最小实现：
- 内存态 map
- 按 `instance_id` 去重

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestInstanceStateManagerUpsertsByInstanceID -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/instance_state.go internal/runtime/instance_state_test.go
 git commit -m "feat: add instance state registry"
```

---

## Chunk 2: 与热更新状态衔接

### Task 3: 让实例状态包含 reload 摘要

**Files:**
- Modify: [`internal/runtime/instance_state.go`](internal/runtime/instance_state.go)
- Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
- Test: [`internal/runtime/instance_state_test.go`](internal/runtime/instance_state_test.go)

- [ ] **Step 1: 写失败测试，验证实例状态可携带 reload 摘要**

测试目标：
- `InstanceStatus` 能表达：
  - `last_reload_at`
  - `last_reload_status`
  - `last_reload_error`

建议测试名：
- `TestInstanceStatusCarriesReloadState`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestInstanceStatusCarriesReloadState -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

最小方案：
- 在 [`InstanceStatus`](internal/runtime/instance_state.go) 中增加必要字段
- 增加一个把 [`ReloadStatus`](internal/runtime/reload.go) 投影成实例状态的方法

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestInstanceStatusCarriesReloadState -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/instance_state.go internal/runtime/reload.go internal/runtime/instance_state_test.go
 git commit -m "feat: attach reload state to instance snapshot"
```

---

## Chunk 3: 暴露只读接口

### Task 4: 在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中增加实例状态快照接口

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/instance_state_handler_test.go`](internal/httpserver/instance_state_handler_test.go)

- [ ] **Step 1: 写失败测试，验证接口存在**

建议接口：
- [`/admin/runtime/instances`](internal/httpserver/server.go)

测试目标：
- 路由已注册
- 返回不是 404
- 受 [`requireAdmin`](internal/httpserver/server.go:73) 保护

建议测试名：
- `TestRuntimeInstancesRouteRegistered`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestRuntimeInstancesRouteRegistered -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 注册 `/admin/runtime/instances`
- 返回实例状态快照列表

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestRuntimeInstancesRouteRegistered -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/instance_state_handler_test.go
 git commit -m "feat: add runtime instance snapshot endpoint"
```

### Task 5: 统一返回结构

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/instance_state_handler_test.go`](internal/httpserver/instance_state_handler_test.go)

- [ ] **Step 1: 写失败测试，约束返回结构**

期望至少包含：
- `instance_id`
- `last_seen_at`
- `health`
- `role`
- `reload_status`

建议测试名：
- `TestRuntimeInstancesResponseShape`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestRuntimeInstancesResponseShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

将 handler 返回规范化为 `object=list` + `data=[]` 或统一快照结构。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestRuntimeInstancesResponseShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/instance_state_handler_test.go
 git commit -m "feat: standardize runtime instance snapshot response"
```

---

## Chunk 4: 增加验证程序与回归

### Task 6: 新增 [`cmd/verify/instance_state/main.go`](cmd/verify/instance_state/main.go)

**Files:**
- Create: [`cmd/verify/instance_state/main.go`](cmd/verify/instance_state/main.go)

- [ ] **Step 1: 写一个最小验证程序**

程序至少打印：
- 构造的实例身份
- 当前实例状态快照
- reload 摘要字段

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/instance_state`
Expected: 打印实例身份与状态字段。

- [ ] **Step 3: 如输出不完整则最小修复**

只修复验证路径所需问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/instance_state/main.go internal/runtime internal/httpserver
 git commit -m "test: add instance state verification tool"
```

### Task 7: 本地回归

**Files:**
- No new files

- [ ] **Step 1: Run runtime tests**

Run: `go test ./internal/runtime -v`
Expected: PASS

- [ ] **Step 2: Run httpserver tests**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 3: Run verify package tests**

Run: `go test ./cmd/verify/instance_state`
Expected: PASS or `[no test files]`

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize instance discovery and state aggregation"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-instance-discovery-and-state-aggregation.md`. Ready to execute?
