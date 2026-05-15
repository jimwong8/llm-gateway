# LLM Gateway

- 运维/验证 runbook：见 [RUNBOOK.md](./RUNBOOK.md)

## 验证命令

当前仓库已经提供十一条可直接运行的 smoke / verify 命令，分别覆盖不同层级：

```bash
# 1) control-plane -> runtime apply 闭环
go run ./cmd/verify/controlplane_runtime

# 2) control-plane replay -> /v1/chat/completions policy enforcement 闭环
go run ./cmd/verify/chat_policy

# 3) model governance admin/runtime 路由闭环
go run ./cmd/verify/model_governance

# 5) runtime observer / runtime decisions / distribution events 查询闭环（需 PostgreSQL DSN）
go run ./cmd/verify/runtime_observer

# 6) promotion 发布闭环
go run ./cmd/verify/promotion

# 7) compensation replay 闭环
go run ./cmd/verify/compensation

# 8) policy engine request-path + audit visibility 闭环
go run ./cmd/verify/policy_engine

# 9) project scope / precedence 闭环
go run ./cmd/verify/project_scope

# 10) runtime bus / manager 状态 / compensation 事件传播闭环
go run ./cmd/verify/runtime_bus

# 11) 顶层总 smoke（顺序执行核心三条）
go run ./cmd/verify/smoke

# 6) 增强型 smoke（在核心三条之外附加 runtime observer 查询闭环）
go run ./cmd/verify/smoke_plus
```

### 分层说明

- `cmd/verify/controlplane_runtime`
  - 验证 control-plane 版本发布/重放/补偿恢复/回滚 是否能正确驱动 runtime apply
  - 同时覆盖 policy overlays 的 runtime 回放：
    - allowed_models
    - role_bindings
    - provider_policies
    - sensitive_rules

- `cmd/verify/chat_policy`
  - 验证真实请求路径 `/v1/chat/completions` 上的 policy enforcement
  - 当前覆盖：
    - sensitive content block
    - role deny
    - provider deny all candidates
    - preferred model deny
    - baseline allow

- `cmd/verify/smoke`
  - 顶层总入口
  - 顺序执行 `controlplane_runtime`、`chat_policy` 与 `model_governance`
  - 适合作为改动后的默认一键 smoke

- `cmd/verify/smoke_plus`
  - 增强型总入口
  - 在默认 `smoke` 的核心三条之外，再执行 `runtime_observer`
  - 适合接了真实 PostgreSQL 的环境，用于交付前增强验证治理运行时观测链路

- `cmd/verify/model_governance`
  - 验证治理域 admin/runtime 基础路由是否可用
  - 当前覆盖：
    - recommendation / approval / policy version / rollout / rollback / evaluation / drift 关键写路径
    - rollout dashboard、rollback records、runtime-decisions、distribution-events、runtime-observer
    - memory governance（candidate facts / project facts / confirm|reject|promote / bulk actions）
    - runtime resolve 公共入口
    - admin 鉴权保护

- `cmd/verify/runtime_observer`
  - 专门验证 runtime observer / runtime decisions / distribution events 的真实查询闭环
  - 需要设置 `POSTGRES_DSN` 或 `GOVERNANCE_TEST_POSTGRES_DSN`
  - 若未提供 DSN，则命令会输出 skip 提示并退出 0
  - 适合在接了真实 PostgreSQL 的环境下确认治理运行时观测链路可查可验

- `cmd/verify/promotion`
  - 专门验证 `/admin/promotions` 发布闭环
  - 重点确认 source released version 可被推广到目标环境，并生成新的 released version id

- `cmd/verify/compensation`
  - 专门验证 `/admin/control-plane/compensations/replay` 补偿重放闭环
  - 重点确认 released version 可被 compensation replay，且 runtime manager 收到并应用事件

- `cmd/verify/policy_engine`
  - 专门验证策略引擎的 request-path 闭环与审计可见性
  - 当前覆盖：readonly 角色拒绝、provider deny-all、sensitive rule block、audit summary 可见

- `cmd/verify/project_scope`
  - 专门验证 tenant scope / project scope / precedence 闭环
  - 当前覆盖：project scope 缺少 project_id 时被拒绝、project override > tenant default、project > tenant > template > default 的配置合并优先级

- `cmd/verify/runtime_bus`
  - 专门验证 runtime bus / publisher / manager / compensation 的事件传播闭环
  - 当前覆盖：released version 发布后 manager 同步成功、reload 失败后状态落为 error 且 compensation 记录可见

建议在做完 control-plane、runtime、policy、router、quota、governance 相关改动后至少运行：

```bash
go run ./cmd/verify/smoke
go test ./...
```

### Memory Governance / Admin 操作入口

当前仓库已提供 memory governance admin API，可用于查看候选事实、项目事实，并执行 confirm / reject / promote：

- `GET /admin/memory/candidate-facts`
- `GET /admin/memory/project-facts`
- `POST /admin/memory/candidate-facts/{factKey}/confirm`
- `POST /admin/memory/candidate-facts/{factKey}/reject`
- `POST /admin/memory/candidate-facts/{factKey}/promote`
- `POST /admin/memory/candidate-facts/actions/{action}`（bulk confirm / reject / promote）

其中 bulk action 请求体格式为：

```json
{
  "items": [
    { "tenant_id": "tenant-a", "user_id": "user-a", "fact_key": "repo" },
    { "tenant_id": "tenant-a", "user_id": "user-a", "fact_key": "stack" }
  ]
}
```

具体 curl 示例与运维流程见 [RUNBOOK.md](./RUNBOOK.md)。

### 推荐执行时机

- 只改 control-plane / runtime wiring / replay / rollback：

```bash
go run ./cmd/verify/controlplane_runtime
```

- 只改 policy enforcement / chat 请求路径：

```bash
go run ./cmd/verify/chat_policy
```

- 做了跨层改动，或准备交付前做一次总体验证：

```bash
go run ./cmd/verify/model_governance
go run ./cmd/verify/smoke
go test ./...
```

- 做了跨层改动，且环境已接真实 PostgreSQL，希望把 runtime observer 查询闭环也一起纳入一键验证：

```bash
go run ./cmd/verify/smoke_plus
```

- 做了 promotion / compensation 链路相关改动：

```bash
go run ./cmd/verify/promotion
go run ./cmd/verify/compensation
```

- 做了 policy engine / request-path enforcement / audit summary 相关改动：

```bash
go run ./cmd/verify/policy_engine
```

- 做了 tenant/project scope、project override、effective precedence 相关改动：

```bash
go run ./cmd/verify/project_scope
```

- 做了 runtime bus / publisher / manager / compensation 事件传播相关改动：

```bash
go run ./cmd/verify/runtime_bus
```

- 做了运行时观测、runtime observer、distribution events 或 runtime decisions 相关改动，且环境已接真实 PostgreSQL：

```bash
go run ./cmd/verify/runtime_observer
```
