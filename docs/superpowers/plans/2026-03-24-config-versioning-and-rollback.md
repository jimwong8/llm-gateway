# Config Versioning and Rollback Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为控制面配置引入统一版本模型、当前生效版本指针、差异视图、批量回滚与变更来源审计的最小实现路径，并与热更新、多实例同步能力衔接。

**Architecture:** 在控制面之上新增一层统一配置版本域模型，不改变现有 policy/quota/observability 的底层存储职责。所有配置写入都会形成新版本，并更新当前生效指针；热更新、多实例同步和实例状态聚合都围绕“当前生效版本”工作。回滚不直接改写历史，而是通过新的版本事件或指针切换实现。

**Tech Stack:** Go, PostgreSQL, net/http, existing control-plane design, runtime reload abstraction, multi-instance config sync abstraction

---

## File Map

- Create: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
  - 统一配置版本模型与指针模型
- Create: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)
  - 版本结构、指针切换、差异视图与回滚语义测试
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 增加控制面版本读取、diff、rollback 接口
- Create: [`internal/httpserver/config_versioning_handler_test.go`](internal/httpserver/config_versioning_handler_test.go)
  - 覆盖控制面版本接口行为
- Optional Modify: [`internal/runtime/reload.go`](internal/runtime/reload.go)
  - 让 reload 状态能够关联版本号
- Optional Modify: [`internal/runtime/bus.go`](internal/runtime/bus.go)
  - 配置变更事件携带版本字段的使用场景更清晰
- Create: [`cmd/verify/config_versioning/main.go`](cmd/verify/config_versioning/main.go)
  - 打印版本创建、diff、rollback 的最小验证路径

---

## Chunk 1: 统一配置版本模型与生效指针

### Task 1: 创建 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)

**Files:**
- Create: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)

- [ ] **Step 1: 写失败测试，定义版本模型**

先在测试中固定结构：

```go
type ConfigVersion struct {
    Module    string    `json:"module"`
    Scope     string    `json:"scope"`
    TenantID  string    `json:"tenant_id"`
    Version   string    `json:"version"`
    Summary   string    `json:"summary"`
    CreatedAt time.Time `json:"created_at"`
}
```

以及：

```go
type ActivePointer struct {
    Module   string `json:"module"`
    Scope    string `json:"scope"`
    TenantID string `json:"tenant_id"`
    Version  string `json:"version"`
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestConfigVersionShape -v`
Expected: FAIL，提示包或结构不存在。

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go) 中实现：
- [`ConfigVersion`](internal/controlplane/versioning.go)
- [`ActivePointer`](internal/controlplane/versioning.go)
- 最小内存仓储：
  - `AddVersion(...)`
  - `SetActive(...)`
  - `GetActive(...)`
  - `ListVersions(...)`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestConfigVersionShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
git commit -m "feat: add control plane config version model"
```

### Task 2: 为指针切换与版本追加写失败测试

**Files:**
- Modify: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)

- [ ] **Step 1: 写失败测试，验证写入必然产生新版本**

测试目标：
- 两次写入相同模块/tenant 时，生成两个不同 `version`
- `SetActive(...)` 后 `GetActive(...)` 能返回最新版本

建议测试名：
- `TestVersionStoreAppendsVersions`
- `TestVersionStoreTracksActivePointer`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run 'TestVersionStore(AppendsVersions|TracksActivePointer)' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

让内存仓储支持：
- 同一 `(module, scope, tenant)` 下多个版本并存
- 单独维护当前指针

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run 'TestVersionStore(AppendsVersions|TracksActivePointer)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
git commit -m "feat: track active config version pointers"
```

---

## Chunk 2: 差异视图与批量回滚语义

### Task 3: 实现最小 diff 视图

**Files:**
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)

- [ ] **Step 1: 写失败测试，定义 diff 结构**

建议结构：

```go
type ConfigDiff struct {
    FromVersion string         `json:"from_version"`
    ToVersion   string         `json:"to_version"`
    Changes     map[string]any `json:"changes"`
}
```

测试目标：
- 支持“相邻版本 diff”
- 支持“当前生效版本 vs 指定版本 diff”

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestConfigDiffShape -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

实现：
- `DiffVersions(from, to ConfigVersion) ConfigDiff`
- 首版只做结构级字段 diff
- 不做三方合并

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestConfigDiffShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
 git commit -m "feat: add config version diff view"
```

### Task 4: 为批量回滚写失败测试

**Files:**
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)

- [ ] **Step 1: 写失败测试，定义批量回滚语义**

测试目标：
- 一次可回滚多个模块到指定版本
- 回滚不会删除旧版本
- 回滚后当前生效指针更新
- 回滚动作本身生成新的变更事件/记录

建议测试名：
- `TestBatchRollbackUpdatesActivePointers`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestBatchRollbackUpdatesActivePointers -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go) 中增加：
- `RollbackSet(...)`
- 支持 tenant 级多个模块一次更新 active pointer
- 首版不做事务编排模拟，只保证语义正确

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestBatchRollbackUpdatesActivePointers -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
 git commit -m "feat: add config rollback semantics"
```

---

## Chunk 3: 变更来源审计与接口暴露

### Task 5: 扩展版本记录以包含变更来源

**Files:**
- Modify: [`internal/controlplane/versioning.go`](internal/controlplane/versioning.go)
- Test: [`internal/controlplane/versioning_test.go`](internal/controlplane/versioning_test.go)

- [ ] **Step 1: 写失败测试，覆盖来源字段**

建议最小来源字段：
- `Actor`
- `Source`
- `Summary`

测试目标：
- 新版本记录中可保留来源审计元数据

建议测试名：
- `TestVersionRecordCarriesChangeOrigin`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controlplane -run TestVersionRecordCarriesChangeOrigin -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

扩展 [`ConfigVersion`](internal/controlplane/versioning.go) 增加：
- `Actor`
- `Source`
- `Summary`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controlplane -run TestVersionRecordCarriesChangeOrigin -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controlplane/versioning.go internal/controlplane/versioning_test.go
 git commit -m "feat: add change origin metadata to config versions"
```

### Task 6: 在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中暴露最小版本接口

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Create: [`internal/httpserver/config_versioning_handler_test.go`](internal/httpserver/config_versioning_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖版本读取与 diff 接口**

建议最小接口：
- `/admin/control-plane/config/versions`
- `/admin/control-plane/config/diff`
- `/admin/control-plane/config/rollback`

测试目标：
- 路由已注册
- 返回结构稳定

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpserver -run 'TestControlPlaneConfig(Versions|Diff|Rollback)Routes' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 注册上述接口
- 提供最小 handler 占位，消费 versioning 模型
- 首版只要求接口结构和最小成功路径可用

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpserver -run 'TestControlPlaneConfig(Versions|Diff|Rollback)Routes' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/config_versioning_handler_test.go internal/controlplane/versioning.go internal/controlplane/versioning_test.go
 git commit -m "feat: expose config versioning control plane endpoints"
```

---

## Chunk 4: 验证程序与回归

### Task 7: 新增 [`cmd/verify/config_versioning/main.go`](cmd/verify/config_versioning/main.go)

**Files:**
- Create: [`cmd/verify/config_versioning/main.go`](cmd/verify/config_versioning/main.go)

- [ ] **Step 1: 写最小验证程序**

程序至少打印：
- 创建新版本
- 当前 active pointer
- 一个 diff 结果
- 一次 rollback 结果

- [ ] **Step 2: Run program and confirm output**

Run: `go run ./cmd/verify/config_versioning`
Expected: 输出版本创建、diff 与 rollback 信息。

- [ ] **Step 3: 如输出不完整则最小修复**

只修复验证路径所需问题。

- [ ] **Step 4: Commit**

```bash
git add cmd/verify/config_versioning/main.go internal/controlplane internal/httpserver
 git commit -m "test: add config versioning verification tool"
```

### Task 8: 回归验证

**Files:**
- No new files

- [ ] **Step 1: Run control plane tests**

Run: `go test ./internal/controlplane -v`
Expected: PASS

- [ ] **Step 2: Run httpserver tests**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 3: Run verify package tests**

Run: `go test ./cmd/verify/config_versioning`
Expected: PASS or `[no test files]`

- [ ] **Step 4: Final Commit**

```bash
git add .
git commit -m "chore: finalize config versioning and rollback"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-config-versioning-and-rollback.md`. Ready to execute?
