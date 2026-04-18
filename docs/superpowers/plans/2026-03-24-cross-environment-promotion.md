# Cross-Environment Promotion Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为配置版本建立 `dev -> staging -> prod` 的手动 promotion 边界，使 Released 版本能够跨环境生成新版本、写审计并触发后续热更新与多实例同步链路。

**Architecture:** 不做自动流水线，只围绕“已 Released 版本手动提升到下一个环境”建立最小实现。promotion 被视为一种特殊发布动作：目标环境生成新的 Released 版本记录，记录 `source_environment` / `source_version`，并沿用现有版本化、审计、热更新与多实例同步链路。

**Tech Stack:** Go, controlplane models, runtime reload abstractions, audit integration, net/http

---

## File Map

- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
  - 增加 promotion 语义与模型
- Modify: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)
  - 覆盖 dev->staging->prod 路径和环境隔离
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
  - 让新版本可记录来源环境/版本信息
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
  - 让事件显式包含源环境与目标环境（如必要）
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 暴露最小 promotion 控制面接口
- Create: [`internal/httpserver/promotion_handler_test.go`](internal/httpserver/promotion_handler_test.go)
  - 覆盖 promotion 路由与返回结构
- Create: [`cmd/verify/promotion/main.go`](cmd/verify/promotion/main.go)
  - 验证 promotion 路径输出

---

## Chunk 1: 定义 promotion 语义

### Task 1: 为 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 增加最小 promotion 模型

**Files:**
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Test: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)

- [ ] **Step 1: Write the failing test**

新增测试，先定义最小语义：
- 只有 `released` 版本能 promotion
- 只允许 `dev -> staging` 与 `staging -> prod`

建议测试名：
- `TestPromotionAllowedTransitions`
- `TestPromotionRejectsNonReleasedState`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run 'TestPromotion(AllowedTransitions|RejectsNonReleasedState)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中增加：
- `PromoteToEnvironment(...)`
- 最小环境流向校验：
  - `dev -> staging`
  - `staging -> prod`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run 'TestPromotion(AllowedTransitions|RejectsNonReleasedState)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/release_flow_test.go
git commit -m "feat: add environment promotion transitions"
```

### Task 2: 让 promotion 在目标环境生成新 Released 版本

**Files:**
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)

- [ ] **Step 1: Write the failing test**

新增测试目标：
- promotion 后目标环境生成新的 Released 版本
- 不复用源环境版本号作为事实主键
- 保留源环境与源版本来源信息

建议测试名：
- `TestPromotionCreatesNewReleasedVersionInTargetEnvironment`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestPromotionCreatesNewReleasedVersionInTargetEnvironment -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go) 中：
- 为版本记录预留 `SourceEnvironment` / `SourceVersion`

在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中：
- promotion 时创建新的目标环境版本记录
- 该版本状态为 `released`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestPromotionCreatesNewReleasedVersionInTargetEnvironment -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/versioning.go internal/controlplane/release_flow_test.go
git commit -m "feat: create target environment release on promotion"
```

---

## Chunk 2: 审计与事件衔接

### Task 3: 为 promotion 加来源审计与事件字段

**Files:**
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/promotion_handler_test.go`](internal/httpserver/promotion_handler_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- promotion 事件包含源环境 / 目标环境 / 源版本 / 目标版本
- promotion handler 至少返回这些字段

建议测试名：
- `TestPromotionResponseShape`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestPromotionResponseShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/runtime/bus.go`](internal/runtime/bus.go) 中为事件增加：
- `SourceEnvironment`
- `TargetEnvironment`
- `SourceVersion`
- `TargetVersion`

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 新增最小 promotion 接口（如 `/admin/control-plane/config/promote`）
- 返回结构中明确这些字段
- 写入审计事件

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestPromotionResponseShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/bus.go internal/httpserver/server.go internal/httpserver/promotion_handler_test.go
git commit -m "feat: add promotion metadata and audit hooks"
```

---

## Chunk 3: 控制面接口与验证程序

### Task 4: 新增 [`internal/httpserver/promotion_handler_test.go`](internal/httpserver/promotion_handler_test.go)

**Files:**
- Create: [`internal/httpserver/promotion_handler_test.go`](internal/httpserver/promotion_handler_test.go)
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)

- [ ] **Step 1: Write the failing test**

验证：
- `/admin/control-plane/config/promote` 路由存在
- 非法方向（如 prod->staging）返回错误
- 缺少必需字段时返回错误

建议测试名：
- `TestPromotionRouteRegistered`
- `TestPromotionRejectsInvalidDirection`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run 'TestPromotion(RouteRegistered|RejectsInvalidDirection)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 注册 promotion 路由
- 解析源环境、目标环境、模块、tenant_id、版本号
- 调用 release flow 的 promotion 逻辑

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run 'TestPromotion(RouteRegistered|RejectsInvalidDirection)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/promotion_handler_test.go
git commit -m "feat: add control plane promotion endpoint"
```

### Task 5: 新增 [`cmd/verify/promotion/main.go`](cmd/verify/promotion/main.go)

**Files:**
- Create: [`cmd/verify/promotion/main.go`](cmd/verify/promotion/main.go)

- [ ] **Step 1: Write the verification program**

程序至少打印：
- source environment/version
- target environment/new version
- promotion result summary

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/promotion`
Expected: 输出 promotion 路径摘要。

- [ ] **Step 3: Minimal fixes if needed**

只修复验证所需问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/promotion/main.go internal/controlplane internal/httpserver internal/runtime
 git commit -m "test: add promotion verification tool"
```

---

## Chunk 4: 回归验证

### Task 6: 执行本地回归

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

Run: `go test ./cmd/verify/promotion`
Expected: PASS or `[no test files]`

- [ ] **Step 5: Final Commit**

```bash
git add .
git commit -m "chore: finalize cross-environment promotion flow"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-cross-environment-promotion.md`. Ready to execute?
