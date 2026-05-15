# Controlplane 模块审计

- 路径：`internal/controlplane/`
- 规模：12 文件 / 1374 行
- 角色：配置版本生命周期 + scope precedence + 失败补偿

## 文件清单

| 文件 | 角色 |
|---|---|
| `service.go` | `Service` 主入口 — 创建/发布/推广/回滚/解析 |
| `versioning.go` | `ConfigVersion` 数据 + `VersionStore` 内存存储 + diff |
| `release_flow.go` | `ReleaseStore` + `PromotionEngine` + `ValidationHook` 钩子链 |
| `compensation.go` | `CompensationRecord` + `CompensationStore` + `SuggestedActionFor` |
| `promotion_gate.go` | `GateResult` + `Failpoint` + `RunValidationHooks` |
| `scope.go` | `ConfigScope` + `Validate` + `ResolveEffectiveScope` |

## 核心数据

### `ConfigVersion`（versioning.go）

```go
type ConfigVersion struct {
    ID, Module, TenantID, Environment, Scope, ProjectID, Status string
    Source            ConfigSource
    Config            map[string]string  // 实际配置（serialized 子树）
    SourceEnvironment string             // 继承草稿留痕
    SourceVersion     string
    CreatedBy, ApprovedBy, Reason string
    CreatedAt, ReleasedAt time.Time
}
```

状态：
- `ConfigStatusDraft = "draft"`
- `ConfigStatusReleased = "released"`

### `Service` 公开方法（service.go）

| 方法 | 作用 |
|---|---|
| `CreateVersion(input)` | 直接创建一个 draft 版本 |
| `CreateInheritanceDraft(input)` | 从源 environment 当前 released 复制为目标 environment 草稿 |
| `ReleaseDraft(...)` | 把 draft 标记 released → 触发 publisher.PublishIfReleased |
| `PromoteReleased(...)` | 跨环境推广 released（生成新 released id） |
| `RollbackReleased(input)` | 回滚到指定 version → 重新 released |
| `CurrentReleased(...)` | 查当前 released |
| `GetVersion(...)` / `ListVersions(...)` | 查询 |
| `ResolveConfig(...)` | 4 层合并 → 终态配置 |

## 生命周期

```
CreateVersion / CreateInheritanceDraft  →  ConfigVersion(status=draft)
                                       ↓
                              ReleaseDraft (audit + publisher.PublishIfReleased)
                                       ↓
                              ConfigVersion(status=released, ReleasedAt)
                                       ↓  ↓  ↓
              PromoteReleased         RollbackReleased   (终态)
              生成新 released id      切回旧版本
```

## scope precedence（scope.go + service.go）

```
ResolveConfig 入参：projectOverride, tenantOverride, tenantTemplate, tierDefault

最终 = mergeConfig(
    mergeConfig(
      mergeConfig(
        cloneConfig(tierDefault),
        tenantTemplate),
      tenantOverride),
    projectOverride)

优先级（高到低）：project > tenant > template > default
```

`scope.ResolveEffectiveScope(projectScope, tenantScope)` 选两个非空 scope 中优先级更高的（project 优于 tenant）。

`ConfigScope.Validate()`：project scope 必须有 `project_id`，tenant scope 必须有 `tenant_id`，否则报错（这是 `verify/project_scope` 验证的关键路径）。

### Worked Example

```
入参：module=policy, tenant=tenant-a, env=prod, scope=project, project_id=proj-x

projectOverride = {"allowed_models": "[\"gpt-4o\"]"}
tenantOverride  = {"allowed_models": "[\"claude-sonnet\"]", "tenant_rpm": "100"}
tenantTemplate  = {"tenant_rpm": "60"}
tierDefault     = {"tenant_rpm": "30", "log_level": "info"}

合并步骤：
1. clone(tierDefault)           = {tenant_rpm: 30, log_level: info}
2. merge(<-, tenantTemplate)    = {tenant_rpm: 60, log_level: info}
3. merge(<-, tenantOverride)    = {tenant_rpm: 100, log_level: info, allowed_models: [claude-sonnet]}
4. merge(<-, projectOverride)   = {tenant_rpm: 100, log_level: info, allowed_models: [gpt-4o]}  ← 终态
```

## release_flow.go 内部

- `ReleaseRecord` 状态：`draft → submitted → approved → released → rolled_back`
- `PromoteToEnvironment(record, targetEnv, hooks)` 内部：
  1. `RunValidationHooks(hooks)` → `[]GateResult`
  2. 任何 hook fail → 返回 record 不变 + 失败 gate
  3. 全 pass → 复制 record，env=targetEnv，重新 submitted
- `RollbackToVersion(...)` 直接把 ActivePointer 切回旧 version

## compensation.go

```go
type CompensationRecord struct {
    Module, TenantID, Environment, Version string
    FailedStage     string  // "reload" | "publish" | ...
    ErrorSummary    string
    SuggestedAction string  // SuggestedActionFor(stage)
    CreatedAt       time.Time
}
```

`CompensationStore` 内存版（List + Add）。runtime.Manager 在 reload 失败时往这里写。

UI：
- `GET /admin/control-plane/compensations` → List
- `POST /admin/control-plane/compensations/replay` → 重放（具体走 publisher）

## promotion_gate.go

- `GateResult{Name, Pass, Reason}`
- `Failpoint`（保留扩展，目前最简）
- `RunValidationHooks(hooks []ValidationHook) []GateResult`：依次跑每个 hook，收集结果

## 与 docs/plans/ 的对照

- `2026-03-24-config-versioning-and-rollback-design.md` → 全部落地
- `2026-03-24-control-plane-config-management-design.md` → 全部落地
- `2026-03-24-cross-environment-promotion-design.md` → 落地（PromoteReleased + PromotionEngine）
- `2026-03-24-promotion-gates-and-validation-hooks-design.md` → 落地（promotion_gate.go）
- `2026-03-24-automatic-rollback-and-compensation-design.md` → 部分（compensation 是内存版，未自动触发 rollback）

## 风险

| 严重度 | 问题 | 修复 |
|---|---|---|
| P0 | `VersionStore` / `ReleaseStore` / `CompensationStore` 全部内存 | 进程崩溃 → 配置版本全丢 → 必须落 PG（已有 `internal/controlplane/postgres.go`？没有，需要补） |
| P0 | `Service` 各方法接受 `_ context.Context` 但不传给底层 | 实际不可中断 → 加 ctx 透传到所有 store |
| P1 | `recordReleaseSideEffects` 同步 publish + audit | 任一失败影响主流程 → 改异步或最终一致 |
| P1 | `mergeConfig` 仅 string→string 浅合并 | JSON 子结构无法路径级合并 → 需要 deep-merge（policy_diff.flattenRuntimePolicy 已有思路可复用） |
