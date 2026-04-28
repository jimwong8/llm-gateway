# LLM Gateway

- 运维/验证 runbook：见 [RUNBOOK.md](./RUNBOOK.md)

## 验证命令

当前仓库已经提供四条可直接运行的 smoke / verify 命令，分别覆盖不同层级：

```bash
# 1) control-plane -> runtime apply 闭环
go run ./cmd/verify/controlplane_runtime

# 2) control-plane replay -> /v1/chat/completions policy enforcement 闭环
go run ./cmd/verify/chat_policy

# 3) model governance admin/runtime 路由闭环
go run ./cmd/verify/model_governance

# 4) 顶层总 smoke（顺序执行上面三条）
go run ./cmd/verify/smoke
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

- `cmd/verify/model_governance`
  - 验证治理域 admin/runtime 基础路由是否可用
  - 当前覆盖：
    - recommendation / approval / policy version / rollout / rollback / evaluation / drift 关键写路径
    - rollout dashboard、rollback records、runtime-decisions、distribution-events、runtime-observer
    - memory governance（candidate facts / project facts / confirm|reject|promote / bulk actions）
    - runtime resolve 公共入口
    - admin 鉴权保护

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
