# Tenant Tiering and Environment Isolation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为控制面、版本化、审批流、热更新和多实例同步能力补齐统一的 `tenant_tier + environment + scope` 隔离模型，确保配置生效边界始终明确且不可串用。

**Architecture:** 不引入跨租户共享策略，也不在首版实现 project 级 override，而是先将 `tenant_tier`、`environment`、`scope` 明确定义为统一的边界字段，并逐步接入控制面、版本模型、审批流、热更新与多实例同步路径。目标是先稳定“生效边界”的公共语言，而不是立刻扩展复杂继承逻辑。

**Tech Stack:** Go, net/http, PostgreSQL, existing control-plane/versioning/reload abstractions

---

## File Map

- Modify: [`internal/config/config.go`](internal/config/config.go)
  - 视需要加入 `Environment`/默认 scope 配置常量或辅助解析
- Create: [`internal/controlplane/scope.go`](internal/controlplane/scope.go)
  - 定义 tenant tier / environment / scope 的统一领域模型
- Create: [`internal/controlplane/scope_test.go`](internal/controlplane/scope_test.go)
  - 验证边界模型与约束
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
  - 让版本模型显式包含 `environment` 与 `scope`
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
  - 让审批流显式携带 `environment` 与 `scope`
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
  - 让配置变更事件明确携带 `environment` 与 `scope`
- Modify: [`internal/runtime/instance_state.go`](internal/runtime/instance_state.go)
  - 如有必要，实例状态里展示本实例所属 environment / role
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 控制面接口显式要求或返回 `tenant_id` / `environment` / `scope`
- Create: [`internal/httpserver/scope_handler_test.go`](internal/httpserver/scope_handler_test.go)
  - 验证控制面和版本/发布接口的隔离边界
- Create: [`cmd/verify/tenant_scope/main.go`](cmd/verify/tenant_scope/main.go)
  - 最小验证程序，打印 tenant/environment/scope 生效边界

---

## Chunk 1: 建立统一隔离模型

### Task 1: 创建 [`internal/controlplane/scope.go`](internal/controlplane/scope.go)

**Files:**
- Create: [`internal/controlplane/scope.go`](internal/controlplane/scope.go)
- Test: [`internal/controlplane/scope_test.go`](internal/controlplane/scope_test.go)

- [ ] **Step 1: Write the failing test**

在 [`internal/controlplane/scope_test.go`](internal/controlplane/scope_test.go) 中先定义：

```go
type TenantTier string
const (
    TenantTierFree       TenantTier = "free"
    TenantTierPro        TenantTier = "pro"
    TenantTierEnterprise TenantTier = "enterprise"
)

type Environment string
const (
    EnvironmentDev     Environment = "dev"
    EnvironmentStaging Environment = "staging"
    EnvironmentProd    Environment = "prod"
)

type Scope string
const (
    ScopeTenant Scope = "tenant"
)
```

并测试这些常量值存在且稳定。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestScopeConstantsShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/scope.go`](internal/controlplane/scope.go) 中实现：
- [`TenantTier`](internal/controlplane/scope.go)
- [`Environment`](internal/controlplane/scope.go)
- [`Scope`](internal/controlplane/scope.go)
- 一个最小组合结构，例如：

```go
type ConfigScope struct {
    TenantID    string      `json:"tenant_id"`
    TenantTier  TenantTier  `json:"tenant_tier"`
    Environment Environment `json:"environment"`
    Scope       Scope       `json:"scope"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestScopeConstantsShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/scope.go internal/controlplane/scope_test.go
git commit -m "feat: add tenant tier and environment scope model"
```

### Task 2: 增加最小校验逻辑

**Files:**
- Modify: [`internal/controlplane/scope.go`](internal/controlplane/scope.go)
- Test: [`internal/controlplane/scope_test.go`](internal/controlplane/scope_test.go)

- [ ] **Step 1: Write the failing test**

验证：
- `tenant_id + environment` 是最小生效边界
- `scope` 首版只能是 `tenant`
- `tenant_tier` 不为空时必须属于允许集合

建议测试名：
- `TestConfigScopeValidation`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestConfigScopeValidation -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/scope.go`](internal/controlplane/scope.go) 中新增：
- `func (s ConfigScope) Validate() error`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestConfigScopeValidation -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/scope.go internal/controlplane/scope_test.go
git commit -m "feat: validate tenant tier environment scope"
```

---

## Chunk 2: 接入版本化 / 审批流 / 总线模型

### Task 3: 扩展 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)

**Files:**
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)

- [ ] **Step 1: Write the failing test**

目标：
- `ConfigVersion` 必须显式携带 `tenant_id`、`environment`、`scope`
- `ActivePointer` 同样要携带这些字段

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestConfigVersionCarriesEnvironmentScope -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go) 中补齐字段，并让版本仓储按 `(module, tenant_id, environment, scope, version)` 组织。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestConfigVersionCarriesEnvironmentScope -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
 git commit -m "feat: add environment scope to config versioning"
```

### Task 4: 扩展 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)

**Files:**
- Modify: [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go)
- Test: [`internal/controlplane/release_flow_test.go`](internal/controlplane/release_flow_test.go)

- [ ] **Step 1: Write the failing test**

目标：
- Draft / Pending / Released 必须显式携带 `tenant_id`、`environment`、`scope`
- 同 tenant 不同 environment 的发布流互不影响

建议测试名：
- `TestReleaseFlowIsolatedByEnvironment`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestReleaseFlowIsolatedByEnvironment -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/release_flow.go`](internal/controlplane/release_flow.go) 中加入字段并按 `(module, tenant_id, environment, scope)` 维度隔离流转。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestReleaseFlowIsolatedByEnvironment -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/release_flow.go internal/controlplane/release_flow_test.go
 git commit -m "feat: isolate release flow by environment and scope"
```

### Task 5: 扩展 [`internal/runtime/bus.go`](internal/runtime/bus.go)

**Files:**
- Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
- Test: [`internal/runtime/bus_test.go`](internal/runtime/bus_test.go)

- [ ] **Step 1: Write the failing test**

目标：
- `ConfigChangeEvent` 必须携带 `tenant_id`、`environment`、`scope`
- 事件消费方可用这些字段判断是否属于自身边界

建议测试名：
- `TestConfigChangeEventCarriesEnvironmentScope`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -run TestConfigChangeEventCarriesEnvironmentScope -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

扩展 [`ConfigChangeEvent`](internal/runtime/bus.go) 字段，但不改变现有 `PublishConfigChange` / `SubscribeConfigChange` 抽象。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -run TestConfigChangeEventCarriesEnvironmentScope -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/bus.go internal/runtime/bus_test.go
 git commit -m "feat: add environment scope to config change events"
```

---

## Chunk 3: 控制面接口与验证程序

### Task 6: 在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中统一返回隔离字段

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Create: [`internal/httpserver/scope_handler_test.go`](internal/httpserver/scope_handler_test.go)

- [ ] **Step 1: Write the failing test**

目标：
- 控制面读取与写入接口返回中显式包含：
  - `tenant_id`
  - `environment`
  - `scope`

建议测试名：
- `TestControlPlaneResponsesCarryEnvironmentScope`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run TestControlPlaneResponsesCarryEnvironmentScope -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中统一补足这些字段的读取/回显逻辑。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run TestControlPlaneResponsesCarryEnvironmentScope -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/scope_handler_test.go
 git commit -m "feat: expose environment scope in control plane responses"
```

### Task 7: 新增 [`cmd/verify/tenant_scope/main.go`](cmd/verify/tenant_scope/main.go)

**Files:**
- Create: [`cmd/verify/tenant_scope/main.go`](cmd/verify/tenant_scope/main.go)

- [ ] **Step 1: Write the verification program**

程序至少打印：
- 一个 `tenant_id`
- 一个 `environment`
- 一个 `scope`
- 当前配置或状态快照中这三者的组合

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/tenant_scope`
Expected: 能打印 tenant/environment/scope 组合。

- [ ] **Step 3: Minimal fixes if needed**

仅修复验证路径需要的问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/tenant_scope/main.go internal/controlplane internal/runtime internal/httpserver
 git commit -m "test: add tenant environment scope verification"
```

---

## Chunk 4: 回归验证

### Task 8: 执行本地回归

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

Run: `go test ./cmd/verify/tenant_scope`
Expected: PASS or `[no test files]`

- [ ] **Step 5: Final Commit**

```bash
git add .
git commit -m "chore: finalize tenant tier and environment isolation"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-tenant-tiering-and-environment-isolation.md`. Ready to execute?
