# Tenant Templates and Capability Bundles Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为多租户控制面引入 tenant 默认模板、capability bundle、单层模板继承与 tenant tier 映射关系，形成标准化租户初始化能力底座。

**Architecture:** 在现有 tenant / environment / scope 模型上引入两类上层对象：模板与能力包。模板承载默认配置结构，bundle 表达能力组合，`tenant_tier` 通过映射关系选择默认模板与 bundle；tenant 与 project 仍可在其上覆盖，保持 `project override > tenant override > tenant template > tier-mapped default bundle` 的优先级不变。

**Tech Stack:** Go, controlplane models, versioning/release abstractions, runtime bus, net/http

---

## File Map

- Create: [`internal/controlplane/template.go`](internal/controlplane/template.go)
  - 定义 tenant template、capability bundle、template inheritance、tier mapping
- Create: [`internal/controlplane/template_test.go`](internal/controlplane/template_test.go)
  - 验证模板、bundle、继承与 tier 映射
- Modify: [`internal/controlplane/scope.go`](internal/controlplane/scope.go)
  - 使 `tenant_tier` 与模板 / bundle 模型衔接清晰
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
  - 如有必要，为模板与 bundle 版本化预留字段
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 暴露最小模板 / bundle / tier mapping 管理接口
- Create: [`internal/httpserver/template_handler_test.go`](internal/httpserver/template_handler_test.go)
  - 覆盖模板与 bundle 接口
- Create: [`cmd/verify/templates/main.go`](cmd/verify/templates/main.go)
  - 打印 tier -> template -> bundle -> effective config 的最小验证路径

---

## Chunk 1: 定义模板与能力包模型

### Task 1: 创建 [`internal/controlplane/template.go`](internal/controlplane/template.go)

**Files:**
- Create: [`internal/controlplane/template.go`](internal/controlplane/template.go)
- Test: [`internal/controlplane/template_test.go`](internal/controlplane/template_test.go)

- [ ] **Step 1: Write the failing test**

先定义目标结构：

```go
type CapabilityBundle struct {
    Name      string         `json:"name"`
    Features  map[string]bool `json:"features"`
}

type TenantTemplate struct {
    Name         string `json:"name"`
    Parent       string `json:"parent,omitempty"`
    BundleName   string `json:"bundle_name"`
    Summary      string `json:"summary,omitempty"`
}
```

新增测试：
- 能创建 bundle
- 能创建 template
- `Parent` 可为空

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestTemplateAndBundleShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/template.go`](internal/controlplane/template.go) 中实现：
- [`CapabilityBundle`](internal/controlplane/template.go)
- [`TenantTemplate`](internal/controlplane/template.go)
- 最小 in-memory 仓储

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestTemplateAndBundleShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/template.go internal/controlplane/template_test.go
 git commit -m "feat: add tenant template and capability bundle model"
```

### Task 2: 定义单层模板继承与 tier 映射

**Files:**
- Modify: [`internal/controlplane/template.go`](internal/controlplane/template.go)
- Modify: [`internal/controlplane/template_test.go`](internal/controlplane/template_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- 模板允许单层继承
- 不允许多级继承链
- tenant tier 能映射到默认模板和默认 bundle

建议测试名：
- `TestSingleLevelTemplateInheritance`
- `TestTenantTierMappingShape`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run 'Test(SingleLevelTemplateInheritance|TenantTierMappingShape)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/template.go`](internal/controlplane/template.go) 中增加：
- `TenantTierMapping`
- `ResolveTemplate(...)`
- `ResolveBundle(...)`

并在逻辑中只允许一层 `Parent`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run 'Test(SingleLevelTemplateInheritance|TenantTierMappingShape)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/template.go internal/controlplane/template_test.go
 git commit -m "feat: add template inheritance and tenant tier mapping"
```

---

## Chunk 2: 接入现有边界模型

### Task 3: 与 [`internal/controlplane/scope.go`](internal/controlplane/scope.go) 衔接

**Files:**
- Modify: [`internal/controlplane/scope.go`](internal/controlplane/scope.go)
- Test: [`internal/controlplane/scope_test.go`](internal/controlplane/scope_test.go)

- [ ] **Step 1: Write the failing test**

测试目标：
- `tenant_tier` 合法值能与 tier mapping 配合
- scope 校验不破坏现有逻辑

建议测试名：
- `TestTenantTierCompatibleWithTemplateMapping`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestTenantTierCompatibleWithTemplateMapping -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

只做必要字段与说明补充，不重构 scope 模型。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestTenantTierCompatibleWithTemplateMapping -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/scope.go internal/controlplane/scope_test.go internal/controlplane/template.go internal/controlplane/template_test.go
 git commit -m "feat: connect tenant tier with templates and bundles"
```

### Task 4: 为版本化预留模板 / bundle 入口

**Files:**
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)

- [ ] **Step 1: Write the failing test**

目标：
- 版本模型可承载 template / bundle 变更来源
- 不破坏现有 module 语义

建议测试名：
- `TestVersioningSupportsTemplateModules`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestVersioningSupportsTemplateModules -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

最小实现：
- 允许 `module=template` / `module=bundle`
- 不新增复杂分支逻辑

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestVersioningSupportsTemplateModules -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
 git commit -m "feat: allow versioning for templates and bundles"
```

---

## Chunk 3: 控制面接口与验证程序

### Task 5: 在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中增加模板 / bundle 接口

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Create: [`internal/httpserver/template_handler_test.go`](internal/httpserver/template_handler_test.go)

- [ ] **Step 1: Write the failing test**

建议接口：
- `/admin/control-plane/templates`
- `/admin/control-plane/bundles`
- `/admin/control-plane/tier-mappings`

测试目标：
- 路由已注册
- 返回结构稳定

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run 'Test(Template|Bundle|TierMapping)Routes' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 注册上述接口
- 返回最小结构化视图
- 不做复杂控制面聚合

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run 'Test(Template|Bundle|TierMapping)Routes' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/template_handler_test.go
 git commit -m "feat: add template and bundle control plane routes"
```

### Task 6: 新增 [`cmd/verify/templates/main.go`](cmd/verify/templates/main.go)

**Files:**
- Create: [`cmd/verify/templates/main.go`](cmd/verify/templates/main.go)

- [ ] **Step 1: Write the verification program**

程序至少打印：
- tenant tier
- mapped template
- mapped bundle
- effective precedence: project override > tenant override > template > bundle

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/templates`
Expected: 打印 tier -> template -> bundle -> precedence 的结果。

- [ ] **Step 3: Minimal fixes if needed**

仅修复最小验证路径问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/templates/main.go internal/controlplane internal/httpserver
 git commit -m "test: add template and bundle verification tool"
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

Run: `go test ./cmd/verify/templates`
Expected: PASS or `[no test files]`

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize tenant templates and capability bundles"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-tenant-templates-and-capability-bundles.md`. Ready to execute?
