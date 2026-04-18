# Message Bus Selection Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 [`internal/runtime/bus.go`](internal/runtime/bus.go) 抽象基础上，为 NATS 作为长期目标、Redis Pub/Sub 作为过渡实现预留稳定适配器边界与验证路径，不引入 Kafka 实现。

**Architecture:** 保持 [`Bus`](internal/runtime/bus.go) 作为唯一上层依赖接口，不让具体中间件 SDK 泄漏到上层逻辑。先在设计与代码层预留 `NATSBus` 与 `RedisPubSubBus` 的适配器边界、构造方式与测试挂点，继续保留 [`InProcessBus`](internal/runtime/bus.go) 作为默认开发与回退实现。

**Tech Stack:** Go, interface abstraction, in-process bus, optional Redis client integration, future NATS adapter placeholder

---

## File Map

- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
  - 预留总线能力标识与构造边界
- Modify: [`internal/runtime/bus_test.go`](internal/runtime/bus_test.go)
  - 增加适配器边界相关测试
- Create: [`internal/runtime/bus_factory.go`](internal/runtime/bus_factory.go)
  - 统一 bus 创建入口
- Create: [`internal/runtime/bus_factory_test.go`](internal/runtime/bus_factory_test.go)
  - 验证工厂与默认回退逻辑
- Create: [`internal/runtime/redis_pubsub_stub.go`](internal/runtime/redis_pubsub_stub.go)
  - Redis Pub/Sub 过渡实现占位
- Create: [`internal/runtime/nats_stub.go`](internal/runtime/nats_stub.go)
  - NATS 长期适配器占位
- Create: [`cmd/verify/runtime_bus_factory/main.go`](cmd/verify/runtime_bus_factory/main.go)
  - 输出 bus 类型与回退路径的验证程序

---

## Chunk 1: 固化总线抽象边界

### Task 1: 为 [`internal/runtime/bus.go`](internal/runtime/bus.go) 增加能力标识与实现类型

**Files:**
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
- Test: [`internal/runtime/bus_test.go`](internal/runtime/bus_test.go)

- [ ] **Step 1: 写失败测试，约束 bus 类型标识存在**

新增测试，先定义目标：

```go
type BusKind string

const (
    BusKindInProcess BusKind = "inprocess"
    BusKindRedisPubSub BusKind = "redis_pubsub"
    BusKindNATS BusKind = "nats"
)
```

测试目标：
- 默认 in-process
- 枚举值稳定

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestBusKindShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/bus.go`](internal/runtime/bus.go) 中增加：
- [`BusKind`](internal/runtime/bus.go)
- `Kind() BusKind` 能力（可放入新接口或扩展接口）
- 让 [`InProcessBus`](internal/runtime/bus.go) 返回 `BusKindInProcess`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestBusKindShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/bus.go internal/runtime/bus_test.go
 git commit -m "feat: add bus kind metadata"
```

### Task 2: 新增 bus 工厂

**Files:**
- Create: [`internal/runtime/bus_factory.go`](internal/runtime/bus_factory.go)
- Test: [`internal/runtime/bus_factory_test.go`](internal/runtime/bus_factory_test.go)

- [ ] **Step 1: 写失败测试，验证默认回退为 in-process**

建议接口：

```go
func NewBus(kind string) Bus
```

测试目标：
- 空值 -> in-process
- 未知值 -> in-process

建议测试名：
- `TestNewBusFallsBackToInProcess`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestNewBusFallsBackToInProcess -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/bus_factory.go`](internal/runtime/bus_factory.go) 中实现：
- `NewBus(kind string) Bus`
- 先只真正返回 [`InProcessBus`](internal/runtime/bus.go)
- 对 redis/nats 先返回占位实现或安全 fallback

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestNewBusFallsBackToInProcess -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/bus_factory.go internal/runtime/bus_factory_test.go
 git commit -m "feat: add runtime bus factory"
```

---

## Chunk 2: 预留 Redis Pub/Sub 与 NATS 适配器边界

### Task 3: 创建 Redis Pub/Sub 占位适配器

**Files:**
- Create: [`internal/runtime/redis_pubsub_stub.go`](internal/runtime/redis_pubsub_stub.go)
- Test: [`internal/runtime/bus_factory_test.go`](internal/runtime/bus_factory_test.go)

- [ ] **Step 1: 写失败测试，约束 Redis 适配器可构造**

测试目标：
- 存在 `NewRedisPubSubBus(...)`
- `Kind()` 返回 `redis_pubsub`
- 默认仍可安全 fallback

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestRedisPubSubBusShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/redis_pubsub_stub.go`](internal/runtime/redis_pubsub_stub.go) 中：
- 提供最小结构与构造函数
- 暂不接入真实 Redis publish/subscribe
- 可以返回 `not implemented` 或保留 no-op 行为

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestRedisPubSubBusShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/redis_pubsub_stub.go internal/runtime/bus_factory_test.go
 git commit -m "feat: add redis pubsub bus stub"
```

### Task 4: 创建 NATS 占位适配器

**Files:**
- Create: [`internal/runtime/nats_stub.go`](internal/runtime/nats_stub.go)
- Test: [`internal/runtime/bus_factory_test.go`](internal/runtime/bus_factory_test.go)

- [ ] **Step 1: 写失败测试，约束 NATS 适配器可构造**

测试目标：
- 存在 `NewNATSBus(...)`
- `Kind()` 返回 `nats`
- 可被工厂识别为长期目标类型

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestNATSBusShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/nats_stub.go`](internal/runtime/nats_stub.go) 中：
- 提供最小结构与构造函数
- 暂不接入真实 NATS SDK
- 保证接口层稳定

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestNATSBusShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/nats_stub.go internal/runtime/bus_factory_test.go
 git commit -m "feat: add nats bus stub"
```

---

## Chunk 3: 增加验证入口

### Task 5: 新增 [`cmd/verify/runtime_bus_factory/main.go`](cmd/verify/runtime_bus_factory/main.go)

**Files:**
- Create: [`cmd/verify/runtime_bus_factory/main.go`](cmd/verify/runtime_bus_factory/main.go)

- [ ] **Step 1: 写一个最小验证程序**

它至少应：
- 创建 `inprocess`
- 创建 `redis_pubsub`
- 创建 `nats`
- 打印每个 bus 的 `Kind()`
- 打印未知值时的 fallback 行为

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/runtime_bus_factory`
Expected: 输出 `inprocess` / `redis_pubsub` / `nats` 及 fallback 结果。

- [ ] **Step 3: 如输出不完整则最小修复**

只修复验证所需问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/runtime_bus_factory/main.go internal/runtime
 git commit -m "test: add message bus selection verification"
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

Run: `go test ./cmd/verify/runtime_bus ./cmd/verify/runtime_bus_factory`
Expected: PASS 或 `[no test files]`

- [ ] **Step 3: 运行最小构建回归**

Run: `go test ./internal/runtime ./cmd/verify/runtime_bus ./cmd/verify/runtime_bus_factory`
Expected: PASS

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize message bus selection abstraction"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-message-bus-selection.md`. Ready to execute?
