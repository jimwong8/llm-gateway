# Promotion Gates and Validation Hooks Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为跨环境 promotion 引入最小可用的 gate、同步验证钩子与显式失败回退点，使 Released 版本在被推进前具备可控检查与可审计的失败中止语义。

**Architecture:** 不引入异步工作流，只在现有 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)、[`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)、[`internal/httpserver/server.go`](internal/httpserver/server.go) 和 [`internal/runtime/bus.go`](internal/runtime/bus.go) 之上增加同步 gate / hook / failpoint 语义。promotion 仍然是显式人工动作，目标环境只在 gate 与同步钩子都通过后才生成新的 Released 版本并触发热更新 / 多实例同步。

**Tech Stack:** Go, controlplane models, runtime bus, audit flow, net/http

---

## File Map

- Create: [`internal/controlplane/promotion_gate.go`](internal/controlplane/promotion_gate.go)
  - gate、hook、failpoint 模型与最小执行逻辑
- Create: [`internal/controlplane/promotion_gate_test.go`](internal/controlplane/promotion_gate_test.go)
  - 验证 gate / hook / failpoint 语义
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
  - 在 promotion 前后挂接 gate 与 hook
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
  - 目标环境版本创建条件与来源字段
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
  - 事件字段与 promotion 结果衔接（如必要）
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 暴露最小 promotion gate / validation 相关接口
- Create: [`internal/httpserver/promotion_gate_handler_test.go`](internal/httpserver/promotion_gate_handler_test.go)
  - 覆盖接口与失败场景
- Modify: [`cmd/verify/promotion/main.go`](cmd/verify/promotion/main.go)
  - 增加 gate 失败、hook 失败、成功 promotion 输出

---

## Chunk 1: 定义 gate / hook / failpoint 模型

### Task 1: 创建 [`internal/controlplane/promotion_gate.go`](internal/controlplane/promotion_gate.go)

**Files:**
- Create: [`internal/controlplane/promotion_gate.go`](internal/controlplane/promotion_gate.go)
- Test: [`internal/controlplane/promotion_gate_test.go`](internal/controlplane/promotion_gate_test.go)

- [ ] **Step 1: Write the failing test**

在 [`internal/controlplane/promotion_gate_test.go`](internal/controlplane/promotion_gate_test.go) 中先定义目标模型：

```go
type GateResult struct {
    Name   string `json:"name"`
    Passed bool   `json:"passed"`
    Error  string `json:"error,omitempty"`
}
```

以及最小验证器接口：

```go
type ValidationHook interface {
    Name() string
    Validate() error
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestGateAndHookShape -v`
Expected: FAIL，提示结构或接口不存在。

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/promotion_gate.go`](internal/controlplane/promotion_gate.go) 中实现：
- [`GateResult`](internal/controlplane/promotion_gate.go)
- [`ValidationHook`](internal/controlplane/promotion_gate.go)
- 一个最小执行函数，例如：

```go
func RunValidationHooks(hooks []ValidationHook) []GateResult
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestGateAndHookShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/promotion_gate.go internal/controlplane/promotion_gate_test.go
git commit -m "feat: add promotion gate and validation hook model"
```

### Task 2: 定义显式失败回退点语义

**Files:**
- Modify: [`internal/controlplane/promotion_gate.go`](internal/controlplane/promotion_gate.go)
- Test: [`internal/controlplane/promotion_gate_test.go`](internal/controlplane/promotion_gate_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- gate 未通过时，中止 promotion
- hook 返回错误时，中止 promotion
- 中止后不创建目标环境 Released 版本

建议测试名：
- `TestPromotionStopsAtExplicitFailpoint`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestPromotionStopsAtExplicitFailpoint -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/promotion_gate.go`](internal/controlplane/promotion_gate.go) 中新增：
- `type Failpoint struct { ... }` 或等价结果语义
- 保证执行器在失败时返回明确中止结果，而不是继续后续流程

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestPromotionStopsAtExplicitFailpoint -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/promotion_gate.go internal/controlplane/promotion_gate_test.go
git commit -m "feat: add explicit promotion failpoint semantics"
```

---

## Chunk 2: 接入 release flow 与 versioning

### Task 3: 在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中挂接 gate / hook

**Files:**
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Test: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)

- [ ] **Step 1: Write the failing test**

新增测试目标：
- gate 失败时，不进入目标环境版本创建
- hook 失败时，不进入目标环境版本创建
- 全部通过时，仍保持原 promotion 语义

建议测试名：
- `TestPromotionBlockedByGate`
- `TestPromotionBlockedByValidationHook`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run 'TestPromotionBlockedBy(Gate|ValidationHook)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中：
- 在目标环境 Released 版本创建前，调用 gate / hook 执行器
- 失败时直接返回，不更新 active pointer

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run 'TestPromotionBlockedBy(Gate|ValidationHook)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/release_flow_test.go internal/controlplane/promotion_gate.go internal/controlplane/promotion_gate_test.go
git commit -m "feat: enforce promotion gates in release flow"
```

### Task 4: 在 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go) 中补充来源与结果字段

**Files:**
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)

- [ ] **Step 1: Write the failing test**

新增测试目标：
- promotion 产生的新版本可记录：
  - `source_environment`
  - `source_version`
  - gate / hook 结果摘要（如需要最小元数据）

建议测试名：
- `TestPromotionVersionCarriesOriginMetadata`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestPromotionVersionCarriesOriginMetadata -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在版本模型中加入必要字段，但保持最小：
- 不做复杂执行轨迹
- 只保留源环境、源版本和最小结果摘要

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestPromotionVersionCarriesOriginMetadata -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
git commit -m "feat: add promotion origin metadata to versions"
```

---

## Chunk 3: 暴露控制面接口与验证程序

### Task 5: 在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中增加最小 gate / hook 可见性

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Create: [`internal/httpserver/promotion_gate_handler_test.go`](internal/httpserver/promotion_gate_handler_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- promotion 接口返回时，能够包含 gate / hook 结果摘要
- gate 失败 / hook 失败时，返回结构稳定

建议测试名：
- `TestPromotionResponseContainsValidationResults`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestPromotionResponseContainsValidationResults -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 将 gate / hook 的最小结果拼入响应
- 失败时写统一错误与审计

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestPromotionResponseContainsValidationResults -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/promotion_gate_handler_test.go
 git commit -m "feat: expose promotion validation results"
```

### Task 6: 增强 [`cmd/verify/promotion/main.go`](cmd/verify/promotion/main.go)

**Files:**
- Modify: [`cmd/verify/promotion/main.go`](cmd/verify/promotion/main.go)

- [ ] **Step 1: Write verification output for three cases**

程序至少打印：
- gate 失败
- hook 失败
- gate 与 hook 通过后的 promotion 成功

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/promotion`
Expected: 输出三类 promotion 路径结果。

- [ ] **Step 3: Minimal fixes if needed**

只修复验证程序和对应最小响应所需问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/promotion/main.go internal/controlplane internal/httpserver
 git commit -m "test: add promotion gate verification"
```

---

## Chunk 4: 本地回归

### Task 7: 执行回归验证

**Files:**
- No new files

- [ ] **Step 1: Run controlplane tests**

Run: `go test ./internal/controlplane -v`
Expected: PASS

- [ ] **Step 2: Run httpserver tests**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 3: Run verify package tests**

Run: `go test ./cmd/verify/promotion`
Expected: PASS or `[no test files]`

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize promotion gates and validation hooks"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-promotion-gates-and-validation-hooks.md`. Ready to execute?
