# Multi-Instance Config Sync Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为多实例配置同步建立最小可用的消息总线抽象、配置变更事件模型与实例级 reload 状态模型，并与现有运行时热更新层衔接。

**Architecture:** 本阶段不绑定具体中间件，而是在运行时热更新之上增加一层“配置变更传播抽象”。控制面负责发布配置变更事件，实例负责订阅事件、执行本地 reload 并维护实例级状态。这样可以先稳定多实例同步协议，再在后续阶段选择 Redis Pub/Sub、NATS 或其他总线实现。

**Tech Stack:** Go, in-memory interfaces, existing runtime reload manager, existing router/policy/quota/billing modules

---

## File Map

- Create: [`internal/runtime/bus.go`](internal/runtime/bus.go)
  - 定义消息总线抽象接口与事件结构
- Create: [`internal/runtime/bus_test.go`](internal/runtime/bus_test.go)
  - 验证总线抽象和事件模型基础行为
- Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
  - 增加实例级事件接收状态字段
  - 增加基于事件的 reload 协调入口
- Modify: [`internal/runtime/reload_test.go`](internal/runtime/reload_test.go)
  - 补齐实例级状态测试
- Optional Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 若需要，可增加只读接口暴露实例级同步状态
- Create: [`cmd/verify/runtime_bus/main.go`](cmd/verify/runtime_bus/main.go)
  - 验证事件模型与实例级状态打印

---

## Chunk 1: 定义消息总线抽象与事件模型

### Task 1: 创建 [`internal/runtime/bus.go`](internal/runtime/bus.go)

**Files:**
- Create: [`internal/runtime/bus.go`](internal/runtime/bus.go)
- Test: [`internal/runtime/bus_test.go`](internal/runtime/bus_test.go)

- [ ] **Step 1: 写失败测试，定义配置变更事件结构**

在 [`internal/runtime/bus_test.go`](internal/runtime/bus_test.go) 中先定义目标结构：

```go
type ConfigChangeEvent struct {
    Module    string    `json:"module"`
    Scope     string    `json:"scope"`
    TenantID  string    `json:"tenant_id"`
    Version   string    `json:"version"`
    ChangedAt time.Time `json:"changed_at"`
    PayloadRef string   `json:"payload_ref"`
}
```

测试目标：
- 结构字段齐全
- `Module` 与 `Scope` 可用于后续路由

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestConfigChangeEventShape -v`
Expected: FAIL，提示结构不存在。

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/bus.go`](internal/runtime/bus.go) 中新增：
- [`ConfigChangeEvent`](internal/runtime/bus.go)
- `Bus` 接口：
  - `PublishConfigChange(event ConfigChangeEvent) error`
  - `SubscribeConfigChange(handler func(ConfigChangeEvent))`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestConfigChangeEventShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/bus.go internal/runtime/bus_test.go
git commit -m "feat: add runtime config bus abstraction"
```

### Task 2: 提供一个最小 in-process bus 实现

**Files:**
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
- Test: [`internal/runtime/bus_test.go`](internal/runtime/bus_test.go)

- [ ] **Step 1: 写失败测试，验证同进程发布/订阅行为**

新增测试目标：
- 注册一个 handler
- 发布一个 [`ConfigChangeEvent`](internal/runtime/bus.go)
- handler 能收到事件

建议测试名：
- `TestInProcessBusPublishSubscribe`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestInProcessBusPublishSubscribe -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/bus.go`](internal/runtime/bus.go) 中实现：
- `type InProcessBus struct { ... }`
- `func NewInProcessBus() *InProcessBus`
- `PublishConfigChange(...)`
- `SubscribeConfigChange(...)`

保持实现最小：
- 同步调用即可
- 不引入 goroutine 池或外部依赖

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestInProcessBusPublishSubscribe -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/bus.go internal/runtime/bus_test.go
git commit -m "feat: add in-process config bus implementation"
```

---

## Chunk 2: 扩展实例级 reload 状态模型

### Task 3: 在 [`internal/runtime/reload.go`](internal/runtime/reload.go) 中增加事件级状态

**Files:**
- Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
- Test: [`internal/runtime/reload_test.go`](internal/runtime/reload_test.go)

- [ ] **Step 1: 写失败测试，覆盖实例级事件状态字段**

测试目标：
- 每类配置除了现有 reload 状态外，还能记录：
  - `last_seen_event_at`
  - `last_seen_event_version`

建议测试名：
- `TestReloadStatusTracksLastSeenEvent`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestReloadStatusTracksLastSeenEvent -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/reload.go`](internal/runtime/reload.go) 中扩展状态结构，新增：
- `LastSeenEventAt`
- `LastSeenEventVersion`

并提供更新方法，例如：
- `MarkEventSeen(name, version string, at time.Time)`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestReloadStatusTracksLastSeenEvent -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/reload.go internal/runtime/reload_test.go
git commit -m "feat: track instance event sync status"
```

### Task 4: 增加基于事件触发 reload 的最小协调入口

**Files:**
- Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
- Test: [`internal/runtime/reload_test.go`](internal/runtime/reload_test.go)

- [ ] **Step 1: 写失败测试，验证事件进入后可更新状态并触发 reload 回调**

建议测试名：
- `TestHandleConfigChangeUpdatesReloadStatus`

测试目标：
- 事件到达时先标记 `last_seen_event_*`
- 再执行 reload callback
- 最终更新 `last_reload_*`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestHandleConfigChangeUpdatesReloadStatus -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

新增最小协调方法，例如：

```go
func (m *Manager) HandleConfigChange(event ConfigChangeEvent, reload func() error)
```

要求：
- 顺序固定：先 seen，再 reload，再写状态
- 不做复杂重试

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestHandleConfigChangeUpdatesReloadStatus -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/reload.go internal/runtime/reload_test.go
git commit -m "feat: handle config change events in reload manager"
```

---

## Chunk 3: 暴露最小验证入口

### Task 5: 新增 [`cmd/verify/runtime_bus/main.go`](cmd/verify/runtime_bus/main.go)

**Files:**
- Create: [`cmd/verify/runtime_bus/main.go`](cmd/verify/runtime_bus/main.go)

- [ ] **Step 1: 写一个最小验证程序**

它至少应：
- 构造一个 [`ConfigChangeEvent`](internal/runtime/bus.go)
- 用 [`NewInProcessBus()`](internal/runtime/bus.go) 发布事件
- 用 reload manager 打印：
  - `last_seen_event_at`
  - `last_seen_event_version`
  - `last_reload_status`

- [ ] **Step 2: Run program and ensure output appears**

Run: `go run ./cmd/verify/runtime_bus`
Expected: 能打印事件版本、seen 状态与 reload 状态。

- [ ] **Step 3: 如输出不完整则做最小修复**

仅修复验证所需问题，不扩 scope。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/runtime_bus/main.go internal/runtime/bus.go internal/runtime/reload.go
git commit -m "test: add runtime bus verification tool"
```

---

## Chunk 4: 回归验证

### Task 6: 执行本地回归

**Files:**
- No new files

- [ ] **Step 1: 运行 runtime 包测试**

Run: `go test ./internal/runtime -v`
Expected: PASS

- [ ] **Step 2: 运行验证程序**

Run: `go test ./cmd/verify/runtime_bus`
Expected: PASS 或 `[no test files]`

- [ ] **Step 3: 运行最小构建回归**

Run: `go test ./internal/runtime ./cmd/verify/runtime_bus`
Expected: PASS

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize multi-instance config sync abstraction"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-multi-instance-config-sync.md`. Ready to execute?
