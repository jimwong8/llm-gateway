# Runtime Hot Reload Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为路由、策略、配额与观测配置增加进程内热更新、同进程通知与实例级 reload 状态可观测能力。

**Architecture:** 不做跨实例一致性与外部消息总线，先在当前进程内引入统一的运行时 reload 管理层，协调 [`internal/router/router.go`](internal/router/router.go)、[`internal/policy`](internal/policy)、[`internal/quota`](internal/quota) 与 [`internal/httpserver/server.go`](internal/httpserver/server.go) 的运行时视图刷新。控制面写入后同步触发 reload，并把 `last_reload_at / last_reload_status / last_reload_error` 写入内存状态视图供管理端读取。

**Tech Stack:** Go, net/http, in-memory state, existing config/policy/quota/router modules

---

## File Map

- Create: [`internal/runtime/reload.go`](internal/runtime/reload.go)
  - 统一的 reload 状态结构、通知与协调逻辑
- Create: [`internal/runtime/reload_test.go`](internal/runtime/reload_test.go)
  - reload 状态与通知行为测试
- Modify: [`internal/router/router.go`](internal/router/router.go)
  - 继续复用 [`SetGlobalPolicy()`](internal/router/router.go:62)
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 控制面写入后触发热更新
  - 暴露 reload 状态读取接口
- Optional Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
  - 若需要最小化辅助读取方法，做轻量补充
- Optional Modify: [`internal/quota/redis.go`](internal/quota/redis.go)
  - 若需要支持运行时参数刷新，增加最小 setter 或 refresh 方法
- Create: [`internal/httpserver/reload_handler_test.go`](internal/httpserver/reload_handler_test.go)
  - handler 级测试
- Create: [`cmd/verify/runtime_reload/main.go`](cmd/verify/runtime_reload/main.go)
  - 本地/远程联调脚本

---

## Chunk 1: 建立统一 reload 协调层

### Task 1: 创建 [`internal/runtime/reload.go`](internal/runtime/reload.go)

**Files:**
- Create: [`internal/runtime/reload.go`](internal/runtime/reload.go)
- Test: [`internal/runtime/reload_test.go`](internal/runtime/reload_test.go)

- [ ] **Step 1: 写失败测试，定义 reload 状态结构**

在 [`internal/runtime/reload_test.go`](internal/runtime/reload_test.go) 中先定义目标结构：

```go
type ReloadStatus struct {
    Name            string    `json:"name"`
    LastReloadAt    time.Time `json:"last_reload_at"`
    LastReloadStatus string   `json:"last_reload_status"`
    LastReloadError string    `json:"last_reload_error"`
}
```

新增测试目标：
- 默认状态为空但可读
- 更新后能正确记录时间、状态和错误

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestReloadStatusShape -v`
Expected: FAIL，提示包或结构不存在。

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/reload.go`](internal/runtime/reload.go) 中实现：
- [`ReloadStatus`](internal/runtime/reload.go)
- `Manager`（维护多个模块的 reload 状态）
- `SetStatus(name, status, err)`
- `GetStatus(name)`
- `AllStatuses()`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestReloadStatusShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/reload.go internal/runtime/reload_test.go
git commit -m "feat: add runtime reload manager"
```

### Task 2: 增加同进程内通知能力

**Files:**
- Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
- Test: [`internal/runtime/reload_test.go`](internal/runtime/reload_test.go)

- [ ] **Step 1: 写失败测试，验证通知会触发订阅者**

新增测试目标：
- 订阅某个模块 reload 事件后，触发时能收到通知
- 同进程内订阅不阻塞主流程

建议测试名：
- `TestReloadManagerBroadcastsInProcess`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestReloadManagerBroadcastsInProcess -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/reload.go`](internal/runtime/reload.go) 中增加：
- 轻量订阅机制（如 channel map）
- `Notify(name)` 或 `Publish(name)`
- 不引入外部消息队列

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestReloadManagerBroadcastsInProcess -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/reload.go internal/runtime/reload_test.go
git commit -m "feat: add in-process reload notifications"
```

---

## Chunk 2: 把 reload 接入现有模块

### Task 3: 为路由配置接入热更新

**Files:**
- Modify: [`internal/router/router.go`](internal/router/router.go)
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/reload_handler_test.go`](internal/httpserver/reload_handler_test.go)

- [ ] **Step 1: 写失败测试，验证路由配置写入后状态更新**

测试目标：
- 控制面触发路由配置更新后，reload manager 中对应模块状态更新为 success

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestRouteReloadUpdatesStatus -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 在控制面配置写入后触发 [`SetGlobalPolicy()`](internal/router/router.go:62)
- 调用 reload manager 记录路由模块状态

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestRouteReloadUpdatesStatus -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/router/router.go internal/httpserver/server.go internal/httpserver/reload_handler_test.go
 git commit -m "feat: wire route hot reload status"
```

### Task 4: 为 policy / quota / observability 接入热更新状态

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Optional Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
- Optional Modify: [`internal/quota/redis.go`](internal/quota/redis.go)
- Test: [`internal/httpserver/reload_handler_test.go`](internal/httpserver/reload_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 policy/quota/observability reload 状态**

新增测试目标：
- 每类配置写入后都能更新各自 `last_reload_at / last_reload_status / last_reload_error`
- reload 失败时错误能暴露

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run 'Test(Policy|Quota|Observability)ReloadStatus' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- policy 写入后同步刷新运行时视图，并更新状态
- quota 写入后同步刷新 limiter 参数（若现有 limiter 不支持，先做最小 setter）
- observability 写入后同步更新 server 侧相关参数或状态视图

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run 'Test(Policy|Quota|Observability)ReloadStatus' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/policy/postgres.go internal/quota/redis.go internal/httpserver/reload_handler_test.go
 git commit -m "feat: add policy quota observability reload status"
```

---

## Chunk 3: 暴露 reload 状态给控制面

### Task 5: 增加 reload 状态读取接口

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/reload_handler_test.go`](internal/httpserver/reload_handler_test.go)

- [ ] **Step 1: 写失败测试，验证 reload 状态接口存在**

建议接口可以是：
- `/admin/observability/reload`
- 或并入现有 control-plane / observability 返回结构

测试目标：
- 接口已注册
- 返回每类配置的 reload 状态

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestReloadStatusEndpoint -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 注册接口
- 返回路由、策略、配额、观测配置的最新 reload 状态

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestReloadStatusEndpoint -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/reload_handler_test.go
 git commit -m "feat: expose runtime reload status endpoint"
```

---

## Chunk 4: 本地与远程联调

### Task 6: 新增 [`cmd/verify/runtime_reload/main.go`](cmd/verify/runtime_reload/main.go)

**Files:**
- Create: [`cmd/verify/runtime_reload/main.go`](cmd/verify/runtime_reload/main.go)

- [ ] **Step 1: 写联调脚本**

脚本至少验证：
- 触发一次路由配置更新
- 触发一次 policy 配置更新
- 触发一次 quota 配置更新
- 读取 reload 状态接口
- 打印 `last_reload_status / last_reload_error`

- [ ] **Step 2: Run script and confirm incomplete output**

Run: `go run ./cmd/verify/runtime_reload`
Expected: 初始输出可能不完整或缺接口。

- [ ] **Step 3: Write minimal fixes**

只修复与热更新联调直接相关的问题。

- [ ] **Step 4: Run script and confirm it passes**

Run: `go run ./cmd/verify/runtime_reload`
Expected: 能看到热更新动作与 reload 状态输出。

- [ ] **Step 5: 本地回归**

Run:
- `go test ./internal/runtime -v`
- `go test ./internal/httpserver -v`
- `go test ./cmd/verify/runtime_reload`

Expected: PASS

- [ ] **Step 6: 远程回归**

Run（远程）:
- `go test ./internal/runtime ./internal/httpserver ./cmd/verify/runtime_reload`

Expected: PASS

- [ ] **Step 7: Final Commit**

```bash
git add internal/runtime internal/httpserver cmd/verify/runtime_reload
 git commit -m "feat: add runtime hot reload observability"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-runtime-hot-reload.md`. Ready to execute?
