# Project Scope and Config Override Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为控制面配置引入 project 级 scope 与 tenant 默认配置 / project override 的两层优先级模型，并与版本化、发布、热更新、多实例同步能力保持一致。

**Architecture:** 在现有 `tenant` 级 scope 基础上增加 `project` 级 scope，但保持优先级规则最小化：`project override > tenant default`。不做跨 project 继承、不生成 tenant 默认的影子 project 版本，并要求所有版本、发布、热更新与同步事件都显式携带 `scope` 与可选 `project_id`。

**Tech Stack:** Go, control plane models, versioning/release abstractions, runtime bus/reload abstractions

---

## File Map

- Modify: [`internal/controlplane/scope.go`](internal/controlplane/scope.go)
  - 增加 `ScopeProject`、`project_id` 相关模型与校验
- Modify: [`internal/controlplane/scope_test.go`](internal/controlplane/scope_test.go)
  - 覆盖 project scope 与优先级边界
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
  - 让版本模型携带 `project_id`
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
  - 让发布与回滚按 `scope + project_id` 隔离
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
  - 配置事件携带 `project_id`
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 控制面接口显式收发 `scope` 与 `project_id`
- Create: [`internal/httpserver/project_scope_handler_test.go`](internal/httpserver/project_scope_handler_test.go)
  - 验证 project scope 相关接口语义
- Create: [`cmd/verify/project_scope/main.go`](cmd/verify/project_scope/main.go)
  - 最小验证程序，打印 tenant 默认与 project override 的解析结果

---

## Chunk 1: 扩展 scope 模型

### Task 1: 在 [`internal/controlplane/scope.go`](internal/controlplane/scope.go) 中增加 `project` 级 scope

**Files:**
- Modify: [`internal/controlplane/scope.go`](internal/controlplane/scope.go)
- Modify: [`internal/controlplane/scope_test.go`](internal/controlplane/scope_test.go)

- [ ] **Step 1: Write the failing test**

新增测试，先定义：

```go
const ScopeProject Scope = "project"
```

并要求 `ConfigScope` 在 `scope=project` 时必须包含 `project_id`。

建议测试名：
- `TestProjectScopeRequiresProjectID`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestProjectScopeRequiresProjectID -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/scope.go`](internal/controlplane/scope.go) 中：
- 增加 `ScopeProject`
- 为 `ConfigScope` 增加 `ProjectID`
- 更新 `Validate()` 规则：
  - `scope=tenant` 时 `project_id` 为空
  - `scope=project` 时 `project_id` 必填

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestProjectScopeRequiresProjectID -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/scope.go internal/controlplane/scope_test.go
 git commit -m "feat: add project scope boundary model"
```

### Task 2: 固定 tenant default 与 project override 优先级语义

**Files:**
- Modify: [`internal/controlplane/scope.go`](internal/controlplane/scope.go)
- Modify: [`internal/controlplane/scope_test.go`](internal/controlplane/scope_test.go)

- [ ] **Step 1: Write the failing test**

新增测试目标：
- project scope 优先于 tenant scope
- project 缺失时回退 tenant default
- 不自动复制 tenant 配置生成 project 版本

建议测试名：
- `TestProjectOverridePrecedence`
- `TestMissingProjectOverrideFallsBackToTenantDefault`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run 'Test(ProjectOverridePrecedence|MissingProjectOverrideFallsBackToTenantDefault)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

增加最小解析辅助函数，例如：

```go
func ResolveEffectiveScope(projectVersion, tenantVersion *ConfigScope) *ConfigScope
```

只表达优先级，不做复杂继承链。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run 'Test(ProjectOverridePrecedence|MissingProjectOverrideFallsBackToTenantDefault)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/scope.go internal/controlplane/scope_test.go
 git commit -m "feat: define tenant default and project override precedence"
```

---

## Chunk 2: 接入版本化 / 发布 / 总线模型

### Task 3: 扩展 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)

**Files:**
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)

- [ ] **Step 1: Write the failing test**

新增测试目标：
- `scope=project` 的版本必须带 `project_id`
- 同 tenant 同 environment 下，不同 `project_id` 的版本历史互不覆盖

建议测试名：
- `TestProjectVersionsAreIndependent`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestProjectVersionsAreIndependent -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在版本模型中加入 `ProjectID`，并让版本唯一性键显式包含它（tenant scope 为空即可）。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestProjectVersionsAreIndependent -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
 git commit -m "feat: isolate config versions by project scope"
```

### Task 4: 扩展 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 与 [`internal/runtime/bus.go`](internal/runtime/bus.go)

**Files:**
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
- Test: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)
- Test: [`internal/runtime/bus_test.go`](internal/runtime/bus_test.go)

- [ ] **Step 1: Write the failing test**

覆盖：
- project Released 只影响对应 project
- bus 事件携带 `scope + project_id`
- tenant Released 不覆盖已有 project override

建议测试名：
- `TestProjectReleaseDoesNotOverrideTenantDefault`
- `TestConfigChangeEventCarriesProjectScope`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane ./internal/runtime -run 'Test(ProjectReleaseDoesNotOverrideTenantDefault|ConfigChangeEventCarriesProjectScope)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

- 在 [`release_flow.go`](internal/controlplane/release_flow.go) 中增加 `ProjectID`
- 在 [`bus.go`](internal/runtime/bus.go) 中增加 `ProjectID`
- 保证 project 发布只影响 project 自身边界

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane ./internal/runtime -run 'Test(ProjectReleaseDoesNotOverrideTenantDefault|ConfigChangeEventCarriesProjectScope)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/release_flow_test.go internal/runtime/bus.go internal/runtime/bus_test.go
 git commit -m "feat: add project scope to release flow and bus events"
```

---

## Chunk 3: 接入控制面与验证入口

### Task 5: 在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 与新测试中暴露 scope 字段

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Create: [`internal/httpserver/project_scope_handler_test.go`](internal/httpserver/project_scope_handler_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- 控制面接口支持/返回：
  - `tenant_id`
  - `environment`
  - `scope`
  - `project_id`（当 `scope=project` 时）

建议测试名：
- `TestControlPlaneSupportsProjectScope`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestControlPlaneSupportsProjectScope -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在控制面接口中统一读取/回显这些字段，不做复杂解析引擎。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestControlPlaneSupportsProjectScope -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/project_scope_handler_test.go
 git commit -m "feat: expose project scope in control plane endpoints"
```

### Task 6: 新增 [`cmd/verify/project_scope/main.go`](cmd/verify/project_scope/main.go)

**Files:**
- Create: [`cmd/verify/project_scope/main.go`](cmd/verify/project_scope/main.go)

- [ ] **Step 1: Write the verification program**

程序至少打印：
- tenant default 配置标识
- project override 配置标识
- 最终生效的 `scope + project_id`

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/project_scope`
Expected: 输出 tenant 与 project 两层优先级结果。

- [ ] **Step 3: Minimal fixes if needed**

仅修复验证路径需要的问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/project_scope/main.go internal/controlplane internal/runtime internal/httpserver
 git commit -m "test: add project scope verification tool"
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

Run: `go test ./cmd/verify/project_scope`
Expected: PASS or `[no test files]`

- [ ] **Step 5: Final Commit**

```bash
git add .
git commit -m "chore: finalize project scope and config override"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-project-scope-and-config-override.md`. Ready to execute?
