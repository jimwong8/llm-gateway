# Automatic Rollback and Compensation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为发布、promotion、热更新和多实例同步失败链路建立最小可用的自动回退触发条件与结构化补偿记录模型，使失败不再静默消失，并为后续自动重试/自动回滚扩展留出边界。

**Architecture:** 首版不做自动重试编排，不直接执行自动回滚，而是在失败节点统一生成补偿记录，并明确 `failed_stage`、`error_summary` 与 `suggested_action`。补偿记录不进入 active pointer 决策链，也不当作新的配置版本，而是作为一类独立的失败工单/恢复依据，与现有版本化、promotion、热更新和多实例同步模型并行存在。

**Tech Stack:** Go, controlplane models, runtime reload abstractions, message bus abstractions, net/http

---

## File Map

- Create: [`internal/controlplane/compensation.go`](internal/controlplane/compensation.go)
  - 定义补偿记录模型、失败阶段枚举与最小内存仓储
- Create: [`internal/controlplane/compensation_test.go`](internal/controlplane/compensation_test.go)
  - 验证补偿记录结构与失败语义
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
  - 在 promotion 失败路径生成补偿记录
- Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
  - 在 reload 失败路径生成补偿记录或携带补偿入口
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
  - 在配置同步失败语义中预留补偿关联字段（如需要）
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 暴露最小补偿记录读取接口
- Create: [`internal/httpserver/compensation_handler_test.go`](internal/httpserver/compensation_handler_test.go)
  - 覆盖补偿读取接口的结构与路由
- Create: [`cmd/verify/compensation/main.go`](cmd/verify/compensation/main.go)
  - 打印失败触发与补偿记录最小验证路径

---

## Chunk 1: 定义补偿记录模型

### Task 1: 创建 [`internal/controlplane/compensation.go`](internal/controlplane/compensation.go)

**Files:**
- Create: [`internal/controlplane/compensation.go`](internal/controlplane/compensation.go)
- Test: [`internal/controlplane/compensation_test.go`](internal/controlplane/compensation_test.go)

- [ ] **Step 1: Write the failing test**

在 [`internal/controlplane/compensation_test.go`](internal/controlplane/compensation_test.go) 中先定义：

```go
type CompensationRecord struct {
    Module          string    `json:"module"`
    TenantID        string    `json:"tenant_id"`
    Environment     string    `json:"environment"`
    Version         string    `json:"version"`
    FailedStage     string    `json:"failed_stage"`
    ErrorSummary    string    `json:"error_summary"`
    SuggestedAction string    `json:"suggested_action"`
    CreatedAt       time.Time `json:"created_at"`
}
```

并测试：
- 核心字段存在
- `FailedStage` 和 `SuggestedAction` 可用于人工恢复指引

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestCompensationRecordShape -v`
Expected: FAIL，提示结构不存在。

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/compensation.go`](internal/controlplane/compensation.go) 中实现：
- [`CompensationRecord`](internal/controlplane/compensation.go)
- 最小仓储：
  - `Add(record)`
  - `List()`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestCompensationRecordShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/compensation.go internal/controlplane/compensation_test.go
git commit -m "feat: add compensation record model"
```

### Task 2: 固定失败阶段枚举与补偿建议语义

**Files:**
- Modify: [`internal/controlplane/compensation.go`](internal/controlplane/compensation.go)
- Test: [`internal/controlplane/compensation_test.go`](internal/controlplane/compensation_test.go)

- [ ] **Step 1: Write the failing test**

先定义失败阶段最小集合，例如：
- `promotion_gate_failed`
- `promotion_validation_failed`
- `release_write_failed`
- `reload_failed`
- `config_sync_failed`

并测试：
- 枚举值稳定
- 对应的 `SuggestedAction` 可生成为非空文本

建议测试名：
- `TestCompensationStageAndSuggestionShape`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestCompensationStageAndSuggestionShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/compensation.go`](internal/controlplane/compensation.go) 中：
- 定义失败阶段常量
- 增加 `SuggestedActionFor(stage string) string`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestCompensationStageAndSuggestionShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/compensation.go internal/controlplane/compensation_test.go
git commit -m "feat: add compensation stages and suggested actions"
```

---

## Chunk 2: 把补偿记录接入现有失败路径

### Task 3: 在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中接入 promotion 失败补偿

**Files:**
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Modify: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)
- Modify: [`internal/controlplane/compensation.go`](internal/controlplane/compensation.go)（若需要）

- [ ] **Step 1: Write the failing test**

测试目标：
- gate 失败时生成一条 `promotion_gate_failed` 补偿记录
- validation hook 失败时生成一条 `promotion_validation_failed` 补偿记录
- 不创建目标环境 Released 版本

建议测试名：
- `TestPromotionFailureCreatesCompensationRecord`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestPromotionFailureCreatesCompensationRecord -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中：
- 对 gate/hook 失败分支调用补偿仓储 `Add(...)`
- 记录模块、tenant、environment、version、failed_stage、error_summary、suggested_action

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestPromotionFailureCreatesCompensationRecord -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/release_flow_test.go internal/controlplane/compensation.go internal/controlplane/compensation_test.go
git commit -m "feat: add compensation records for promotion failures"
```

### Task 4: 在 [`internal/runtime/reload.go`](internal/runtime/reload.go) 中接入 reload 失败补偿

**Files:**
- Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
- Modify: [`internal/runtime/reload_test.go`](internal/runtime/reload_test.go)
- Optional Modify: [`internal/controlplane/compensation.go`](internal/controlplane/compensation.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- reload callback 失败时，状态更新为 error
- 同时生成一条 `reload_failed` 补偿记录或明确可挂接补偿入口

建议测试名：
- `TestReloadFailureCreatesCompensationRecord`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestReloadFailureCreatesCompensationRecord -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/reload.go`](internal/runtime/reload.go) 中：
- 增加最小补偿回调挂点或直接写补偿记录
- 保持现有 reload 状态逻辑不变

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestReloadFailureCreatesCompensationRecord -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/reload.go internal/runtime/reload_test.go internal/controlplane/compensation.go internal/controlplane/compensation_test.go
git commit -m "feat: add compensation records for reload failures"
```

---

## Chunk 3: 暴露补偿记录读取接口

### Task 5: 在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中暴露只读补偿接口

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Create: [`internal/httpserver/compensation_handler_test.go`](internal/httpserver/compensation_handler_test.go)

- [ ] **Step 1: Write the failing test**

建议接口：
- [`/admin/control-plane/compensations`](internal/httpserver/server.go)

测试目标：
- 路由已注册
- 返回补偿记录列表
- 受 [`requireAdmin`](internal/httpserver/server.go:73) 保护

建议测试名：
- `TestCompensationRouteRegistered`
- `TestCompensationResponseShape`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run 'TestCompensation(RouteRegistered|ResponseShape)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 注册补偿读取接口
- 返回 `object=list` + `data=[]`
- 先做最小读取视图，不支持写入

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run 'TestCompensation(RouteRegistered|ResponseShape)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/compensation_handler_test.go
git commit -m "feat: add compensation listing endpoint"
```

### Task 6: 新增 [`cmd/verify/compensation/main.go`](cmd/verify/compensation/main.go)

**Files:**
- Create: [`cmd/verify/compensation/main.go`](cmd/verify/compensation/main.go)

- [ ] **Step 1: Write the verification program**

程序至少打印：
- 一次模拟失败触发条件
- 最近补偿记录列表
- `failed_stage`
- `suggested_action`

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/compensation`
Expected: 输出补偿记录摘要。

- [ ] **Step 3: Minimal fixes if needed**

只修复最小验证所需问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/compensation/main.go internal/controlplane internal/httpserver internal/runtime
git commit -m "test: add compensation verification tool"
```

---

## Chunk 4: 回归验证

### Task 7: 执行本地回归

**Files:**
- No new files

- [ ] **Step 1: Run controlplane tests**

Run: `go test ./internal/controlplane -v`
Expected: PASS

- [ ] **Step 2: Run runtime tests**

Run: `go test ./internal/runtime -v`
Expected: PASS

- [ ] **Step 3: Run httpserver tests**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 4: Run verify package tests**

Run: `go test ./cmd/verify/compensation`
Expected: PASS or `[no test files]`

- [ ] **Step 5: Final Commit**

```bash
git add .
git commit -m "chore: finalize automatic rollback and compensation model"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-automatic-rollback-and-compensation.md`. Ready to execute?
