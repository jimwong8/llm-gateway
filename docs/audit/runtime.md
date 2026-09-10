# Runtime 模块审计

- 路径：`internal/runtime/`
- 规模：17 文件 / 4002 行
- 角色：control-plane → 各运行时模块（policy / router / quota）的事件总线 + 应用层

## 文件清单

| 文件 | 角色 |
|---|---|
| `bus.go` | 事件总线接口 + `InProcessBus` 内存实现 |
| `publisher.go` | controlplane → runtime 事件发布 + apply payload 构造 |
| `reload.go` | `Manager`：状态记录 + 失败补偿 |
| `bridge.go` | bus 订阅 → manager → 模块 apply 桥接 |
| `policy_apply.go` | 应用 4 类 policy overlay |
| `router_apply.go` | 应用 router bootstrap 配置 |
| `quota_apply.go` | 应用 tenant RPM |
| `startup_replay.go` | 进程启动时重放 current released |

## 核心契约

### `ConfigChangeEvent`
```go
type ConfigChangeEvent struct {
    Module      string    // "policy" | "router" | "quota"
    Scope       string
    TenantID    string
    Environment string
    ProjectID   string
    Version     string    // cfg_xxx
    ChangedAt   time.Time
    PayloadRef  string    // 用于 publisher 反查 RuntimeApplyPayload
}
```

### `RuntimeApplyPayload`
- `BuildRuntimeApplyPayload(version)` 从 `controlplane.ConfigVersion` 构造
- 拆分：`buildRouterModulePayload` / `buildQuotaModulePayload` / `buildPolicyModulePayload`
- `payload_ref` 编码：`<module>:<tenant>:<env>:<project>:<version>`
- 通过 `Publisher.FindApplyPayloadByRef(ref)` 反查

## 事件流

### Happy path

```
controlplane.Service
  → Publisher.PublishIfReleased(version)
    → Bus.PublishConfigChange(event)
      → bridge handler
        → Manager.HandleConfigChange(event, reload)
          → MarkEventSeen
          → reload()
            → Apply.FindApplyPayloadByRef
            → 应用到 policy.Store / router.Router / quota.Limiter
          → SetStatus("ok")
```

### 失败路径

```
Manager.HandleConfigChange
  ↓ reload() returns error
  → SetStatus(module, "error", err)
  → CompensationStore.Add(CompensationRecord{
      Module, TenantID, Environment, Version,
      FailedStage: FailedStageReload,
      ErrorSummary: err.Error(),
      SuggestedAction: SuggestedActionFor(FailedStageReload),
      CreatedAt: now
    })
```

UI 入口：
- 查询：`GET /admin/control-plane/compensations`
- 重放：`POST /admin/control-plane/compensations/replay`

特殊情况：`reload == nil` 也走补偿路径（status="skipped", error="reload func is nil"）。

## Manager（`reload.go`）细节

- `statuses map[string]ReloadStatus` 进程内单例，**无锁** → 依赖 InProcessBus 串行
- `compensations *CompensationStore` 内嵌
- `AllStatuses()` 排序：name asc → LastSeenEventAt desc → LastReloadAt desc
- ⚠️ **无持久化**：进程重启 → statuses 全丢 → 靠 `startup_replay` 重新拉一次

## 三类 Apply

### `policy_apply.go`（4 类 overlay）
1. `allowed_models` → `SetAllowedModels`
2. `role_bindings` → `SetRoleBindings`（先 dedupe by tenant+user+role）
3. `provider_policies` → `SetProviderPolicies`（dedupe by tenant+provider+effect）
4. `sensitive_rules` → `SetSensitiveRules`（dedupe by tenant+pattern）

每类有 3 路径：
- `extractXxxFromPayload(map)` — 从 RuntimeApplyPayload 解析
- `parseXxxList(raw any)` — 严格类型校验
- `parseXxxFromConfig(rawString)` — fallback 从 legacy config map 解析

幂等：同 payload 重复 apply 结果一致（覆盖语义）。

### `router_apply.go`
- 直接调 `applier.BootstrapFromFile(bootstrapPath)` 重新载入 router rules
- 用 `parseReleasedPayloadRef` / `buildRuntimeApplyPayloadFromReleasedVersion` 通过 resolver 反查

### `quota_apply.go`
- 应用 `tenant_rpm` 单字段到 `quota.Limiter.SetTenantRPM`
- 最简单

## startup_replay.go

```go
ReplayCurrentReleasedModuleConfig(ctx, lister, bus, module) error
ReplayCurrentReleasedRouterConfig(ctx, lister, bus) error
```

- 拉 `lister.ListCurrentReleasedByModule(module)`
- 对每个 released version 构造 event → 推到 bus → 与正常发布同管线
- ⚠️ **风险**：单条 record 损坏（payload 不可解析）会让整模块 replay 中断 → 建议 collect errors + continue

## 关键风险

| 严重度 | 问题 | 影响 | 修复 |
|---|---|---|---|
| P0 | `Manager.statuses` 无并发保护 | 多 goroutine bus 时数据竞争 | 加 `sync.RWMutex` |
| P1 | `startup_replay` fail-fast | 启动可能因脏数据卡住 | collect errors continue |
| P1 | InProcessBus handler 同步阻塞 | 一个慢 handler 阻塞所有 | goroutine + per-handler timeout |
| P1 | Manager 无持久化 | 崩溃后 last_reload 状态丢 | 新增 `runtime_module_status` 表 |
| P2 | `payload_ref` 无版本 | 跨版本兼容性差 | 加 `v=1:` 前缀 |
| P2 | InProcessBus 不跨实例 | 多实例不同步 | 见 `2026-03-24-message-bus-selection-design.md`，落地 NATS/Kafka |

## 对 controlplane 的依赖

- 入：`Publisher.PublishIfReleased(controlplane.ConfigVersion)` — 读 .Status / .Config / .Module
- 出：失败时 `CompensationStore.Add(CompensationRecord)` — controlplane 后续可持久化
- 反查：`releasedVersionResolver`（router_apply / policy_apply / quota_apply 共用）从 controlplane.Service 拉历史 released

runtime 不直接读 controlplane DB；只走内存对象。
