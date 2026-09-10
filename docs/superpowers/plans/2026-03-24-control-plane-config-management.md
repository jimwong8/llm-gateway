# Control Plane Config Management Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为控制面补齐统一配置管理入口，使策略、配额与观测配置可以通过一致的 `/admin/control-plane/*` 接口统一读取与写入。

**Architecture:** 不新增统一配置事实表，而是在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 上加一层控制面聚合接口，分别协调 [`internal/policy`](internal/policy)、[`internal/quota`](internal/quota)、[`internal/billing`](internal/billing) 与现有 observability 读取逻辑。返回结构统一，底层仍由各模块保持自己的存储与逻辑。

**Tech Stack:** Go, net/http, PostgreSQL, Redis, existing admin auth middleware, existing policy/quota/billing modules

---

## File Map

- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
  - 注册 `/admin/control-plane/config`
  - 注册 `/admin/control-plane/policy`
  - 注册 `/admin/control-plane/quota`
  - 注册 `/admin/control-plane/observability`
  - 新增统一配置读取与写入 handler
- Create: [`internal/httpserver/control_plane_handler_test.go`](internal/httpserver/control_plane_handler_test.go)
  - 覆盖路由注册、统一返回结构与基础写入行为
- Optional Modify: [`internal/policy/postgres.go`](internal/policy/postgres.go)
  - 如需要，补充面向控制面接口的查询辅助方法
- Optional Modify: [`internal/quota/redis.go`](internal/quota/redis.go)
  - 如需要，补充 quota 配置读取辅助方法
- Optional Modify: [`internal/billing/postgres.go`](internal/billing/postgres.go)
  - 如需要，补充 observability 配置读取辅助方法
- Create: [`cmd/verify/control_plane/main.go`](cmd/verify/control_plane/main.go)
  - 远程联调统一配置入口

---

## Chunk 1: 建立统一控制面读取入口

### Task 1: 为 `/admin/control-plane/*` 写失败测试

**Files:**
- Create: [`internal/httpserver/control_plane_handler_test.go`](internal/httpserver/control_plane_handler_test.go)
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)

- [ ] **Step 1: 写失败测试，覆盖路由注册**

新增测试，验证以下路由不返回 404：
- `/admin/control-plane/config`
- `/admin/control-plane/policy`
- `/admin/control-plane/quota`
- `/admin/control-plane/observability`

建议测试名：
- `TestControlPlaneRoutesRegistered`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run TestControlPlaneRoutesRegistered -v`
Expected: FAIL

- [ ] **Step 3: 写最小实现**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中注册上述四个路由，并创建空壳 handler：
- `adminControlPlaneConfig`
- `adminControlPlanePolicy`
- `adminControlPlaneQuota`
- `adminControlPlaneObservability`

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run TestControlPlaneRoutesRegistered -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/control_plane_handler_test.go
git commit -m "feat: add control plane config routes"
```

### Task 2: 统一读取返回结构

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/control_plane_handler_test.go`](internal/httpserver/control_plane_handler_test.go)

- [ ] **Step 1: 写失败测试，约束统一响应格式**

测试目标：
- GET `/admin/control-plane/config?tenant_id=t1` 返回统一结构：
  - `scope`
  - `tenant_id`
  - `module`
  - `config`
  - `updated_at`

建议测试名：
- `TestControlPlaneConfigResponseShape`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run TestControlPlaneConfigResponseShape -v`
Expected: FAIL

- [ ] **Step 3: 写最小实现**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- 先实现 GET 聚合视图
- `config` 中可以先聚合最小字段：
  - policy 配置摘要
  - quota 配置摘要
  - observability 配置摘要

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run TestControlPlaneConfigResponseShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/control_plane_handler_test.go
 git commit -m "feat: add unified control plane config read view"
```

---

## Chunk 2: 建立最小写入入口

### Task 3: 为 policy / quota / observability 分别补统一写入接口

**Files:**
- Modify: [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Test: [`internal/httpserver/control_plane_handler_test.go`](internal/httpserver/control_plane_handler_test.go)

- [ ] **Step 1: 写失败测试，覆盖 POST/PUT 更新 policy 配置**

建议测试目标：
- `POST /admin/control-plane/policy`
- `PUT /admin/control-plane/policy`
- body 至少包含 `tenant_id` 和 `config`

建议测试名：
- `TestControlPlanePolicyWriteShape`

- [ ] **Step 2: 写失败测试，覆盖 POST/PUT 更新 quota 配置**

建议测试名：
- `TestControlPlaneQuotaWriteShape`

- [ ] **Step 3: 写失败测试，覆盖 POST/PUT 更新 observability 配置**

建议测试名：
- `TestControlPlaneObservabilityWriteShape`

- [ ] **Step 4: 运行测试并确认失败**

Run: `go test ./internal/httpserver -run 'TestControlPlane(Policy|Quota|Observability)WriteShape' -v`
Expected: FAIL

- [ ] **Step 5: 写最小实现**

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中：
- policy 写接口调用 [`internal/policy`](internal/policy) 现有 upsert 能力
- quota 写接口可先只支持最小更新（若底层暂缺持久化配置能力，则返回已接收结构并标记 `updated_at`）
- observability 写接口可先支持最小配置回显与占位更新语义

严格遵守：
- 不新增统一配置表
- 不做跨模块事务

- [ ] **Step 6: 运行测试并确认通过**

Run: `go test ./internal/httpserver -run 'TestControlPlane(Policy|Quota|Observability)WriteShape' -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/control_plane_handler_test.go
 git commit -m "feat: add control plane config write endpoints"
```

---

## Chunk 3: 远程联调与回归

### Task 4: 新增 [`cmd/verify/control_plane/main.go`](cmd/verify/control_plane/main.go)

**Files:**
- Create: [`cmd/verify/control_plane/main.go`](cmd/verify/control_plane/main.go)

- [ ] **Step 1: 写远程联调脚本**

脚本至少验证：
- GET `/admin/control-plane/config`
- GET `/admin/control-plane/policy`
- GET `/admin/control-plane/quota`
- GET `/admin/control-plane/observability`
- 至少一个写接口 POST/PUT 成功返回

- [ ] **Step 2: 运行脚本并确认输出不完整**

Run: `go run ./cmd/verify/control_plane`
Expected: 初次可能缺少部分字段或接口未返回一致结构。

- [ ] **Step 3: 最小修复实现**

仅修复与统一控制面入口直接相关的问题。

- [ ] **Step 4: 运行脚本确认通过**

Run: `go run ./cmd/verify/control_plane`
Expected: 输出统一配置结构和更新结果。

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/control_plane_handler_test.go cmd/verify/control_plane/main.go
 git commit -m "test: add control plane config verification"
```

---

## Chunk 4: 回归验证

### Task 5: 执行本地与远程回归

**Files:**
- No new files

- [ ] **Step 1: 本地回归**

Run:
- `go test ./internal/httpserver -v`
- `go test ./cmd/verify/control_plane`

Expected: PASS

- [ ] **Step 2: 远程回归**

Run（远程）：
- `go test ./internal/httpserver ./cmd/verify/control_plane`

Expected: PASS

- [ ] **Step 3: Final Commit**

```bash
git add .
git commit -m "chore: finalize control plane config management"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-control-plane-config-management.md`. Ready to execute?
