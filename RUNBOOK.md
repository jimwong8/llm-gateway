# LLM Gateway Runbook

## 1. 日常验证入口

### 1.1 顶层总 smoke

```bash
go run ./cmd/verify/smoke
```

用途：一键验证当前仓库最关键的三条闭环：

- control-plane -> runtime apply
- control-plane replay -> request-path policy enforcement
- model governance admin/runtime + memory admin 管理入口

期望输出：

- `[PASS] controlplane_runtime`
- `[PASS] chat_policy`
- `[PASS] model_governance`
- `verify result: PASS smoke(controlplane_runtime,chat_policy,model_governance)`

适用时机：

- 跨 control-plane / runtime / policy / httpserver 的改动后
- 交付前总体验证
- 出现“功能看起来都在，但不确定闭环是否还活着”时

---

### 1.2 control-plane / runtime smoke

```bash
go run ./cmd/verify/controlplane_runtime
```

用途：验证 control-plane 到 runtime 的核心链路：

- replay
- compensation replay
- rollback
- policy overlays runtime replay

当前覆盖的 policy overlays：

- allowed_models
- role_bindings
- provider_policies
- sensitive_rules

当它失败时，优先排查：

- `internal/controlplane/*`
- `internal/runtime/*`
- `cmd/server/main.go`
- `cmd/server_main.go`

---

### 1.3 chat policy request-path smoke

```bash
go run ./cmd/verify/chat_policy
```

用途：验证真实 `/v1/chat/completions` 请求路径上的策略 enforcement。

当前覆盖：

- sensitive content block
- role deny
- provider deny all candidates
- preferred model deny
- baseline allow

当它失败时，优先排查：

- `internal/httpserver/server.go`
- `internal/httpserver/chat_completions_policy_live_apply_test.go`
- `internal/policy/postgres.go`

---

## 2. 推荐验证顺序

### 2.1 只改 control-plane / runtime

```bash
go run ./cmd/verify/controlplane_runtime
go test ./internal/runtime ./internal/controlplane ./internal/httpserver ./cmd/server ./cmd
```

### 2.2 只改 policy enforcement / chat 请求路径

```bash
go run ./cmd/verify/chat_policy
go test ./internal/httpserver -run 'TestChatCompletionsPolicy' -count=1
```

### 2.3 跨层改动或准备交付

```bash
go run ./cmd/verify/model_governance
go run ./cmd/verify/smoke
go test ./...
```

---

## 3. Model Governance 平台运维操作（Admin API）

> 目标：给值班/运维提供一套可直接执行的治理流程（推荐 -> 审批 -> 激活 -> rollout 推进 -> 回滚）。

### 3.1 统一环境变量

先设置网关地址与管理密钥（所有治理 Admin API 都要求 `X-Admin-Key`）：

```bash
export GATEWAY_BASE_URL="http://127.0.0.1:8080"
export ADMIN_API_KEY="admin-dev-key"
```

可选：先快速验证鉴权与服务可达性：

```bash
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/health"
```

### 3.2 手动触发推荐（generate recommendation）

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/recommendations" \
  -d '{
    "tenant_id":"tenant-a",
    "agent_id":"agent-1",
    "task_type":"chat",
    "environment":"prod",
    "requested_by":"ops-bot",
    "summary":"manual recommendation trigger"
  }'
```

期望：HTTP 201，返回体中包含推荐 `id`（后续审批要用）。

### 3.3 审批推荐（approve recommendation）

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/approvals" \
  -d '{
    "recommendation_id":"rec_替换为上一步id",
    "decision":"approved",
    "approved_by":"ops-bot",
    "approval_reason":"manual approval after evaluation",
    "effective_scope":{
      "scope":"agent",
      "environment":"prod"
    }
  }'
```

期望：HTTP 201，返回审批对象 `id`。

### 3.4 创建并审批/激活策略版本（approve policy version + activate）

1) 先从审批单创建策略版本：

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/policy-versions" \
  -d '{
    "approval_id":"ap_替换为上一步id",
    "created_by":"ops-bot"
  }'
```

2) 审批该策略版本（approve）：

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/policy-versions/pv_替换为版本id/approve" \
  -d '{"approved_by":"ops-bot"}'
```

3) 激活该策略版本（activate）：

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/policy-versions/pv_替换为版本id/activate" \
  -d '{}'
```

期望：approve/activate 都返回 HTTP 200，且版本状态进入 `active`。

### 3.5 启动并推进 rollout（start rollout + promote rollout）

1) 启动 rollout：

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/rollouts" \
  -d '{
    "policy_version_id":"pv_替换为已激活版本id",
    "rollout_mode":"progressive",
    "rollout_percent":10,
    "trigger_reason":"start progressive rollout",
    "triggered_by":"ops-bot"
  }'
```

2) 推进 rollout（promote）：

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/rollouts/ro_替换为rollout_id/promote" \
  -d '{
    "rollout_percent":50,
    "guard_summary":"error_rate and latency within threshold"
  }'
```

期望：start 返回 HTTP 201；promote 返回 HTTP 200，`rollout_percent` 更新。

### 3.6 治理回滚（rollback）

当前治理域已暴露两种回滚 admin API：

- `POST /admin/governance/rollbacks`：直接按 rollout id 创建回滚记录并执行回滚
- `POST /admin/governance/rollouts/{rolloutID}/rollback`：从 rollout 子资源入口触发回滚

1) 直接执行治理回滚：

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/rollbacks" \
  -d '{
    "rollout_id":"ro_替换为目标rollout_id",
    "actor":"ops-bot",
    "reason":"rollback to known good policy version"
  }'
```

2) 从 rollout 子资源入口执行回滚：

```bash
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/governance/rollouts/ro_替换为目标rollout_id/rollback" \
  -d '{
    "actor":"ops-bot",
    "reason":"guard threshold violated"
  }'
```

期望：HTTP 201，返回 rollback 结果，包含：

- `rollout.status = rolled_back`
- `restored_policy_version_id`
- `reverted_policy_version_id`
- `distribution_event`

可选只读查询：

```bash
# 查看最近 rollback 记录
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/governance/rollbacks?limit=20"

# 查看指定 rollback 记录明细
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/governance/rollbacks/rb_替换为rollback_id"
```

### 3.7 Memory Governance（候选事实 / 项目事实）

当前 memory admin API 已可用于审阅候选事实、查看项目事实，以及执行 confirm/reject/promote，或通过 bulk action 一次处理多条候选事实：

```bash
# 列出候选事实
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/memory/candidate-facts?tenant_id=tenant-a&user_id=user-a&status=pending"

# 列出项目事实
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/memory/project-facts?tenant_id=tenant-a&user_id=user-a&status=active"

# 确认候选事实
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/memory/candidate-facts/repo/confirm" \
  -d '{"tenant_id":"tenant-a","user_id":"user-a"}'

# 拒绝候选事实
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/memory/candidate-facts/repo/reject" \
  -d '{"tenant_id":"tenant-a","user_id":"user-a"}'

# 提升候选事实为已采纳事实
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/memory/candidate-facts/repo/promote" \
  -d '{"tenant_id":"tenant-a","user_id":"user-a"}'

# 批量确认多条候选事实
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/memory/candidate-facts/actions/confirm" \
  -d '{
    "items": [
      {"tenant_id":"tenant-a","user_id":"user-a","fact_key":"repo"},
      {"tenant_id":"tenant-a","user_id":"user-a","fact_key":"stack"}
    ]
  }'

# 批量拒绝多条候选事实
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/memory/candidate-facts/actions/reject" \
  -d '{
    "items": [
      {"tenant_id":"tenant-a","user_id":"user-a","fact_key":"repo"},
      {"tenant_id":"tenant-a","user_id":"user-a","fact_key":"stack"}
    ]
  }'

# 批量提升多条候选事实
curl -sS -X POST \
  -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  "${GATEWAY_BASE_URL}/admin/memory/candidate-facts/actions/promote" \
  -d '{
    "items": [
      {"tenant_id":"tenant-a","user_id":"user-a","fact_key":"repo"},
      {"tenant_id":"tenant-a","user_id":"user-a","fact_key":"stack"}
    ]
  }'
```

期望：列表接口返回 HTTP 200；单条动作接口返回 HTTP 200，且返回候选事实最新状态；bulk action 返回 HTTP 200，并包含 `success_count`、`failure_count` 与逐项 `results`。

### 3.8 验证命令（MUST RUN）

治理流程操作后，至少执行以下验证：

```bash
# 治理 admin/runtime 路由闭环
go run ./cmd/verify/model_governance

# 顶层总 smoke（覆盖 control-plane + chat_policy + governance）
go run ./cmd/verify/smoke

# 全量回归（交付前建议）
go test ./...
```

可选线上核验（只读查询）：

```bash
# 查看最近推荐
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/governance/recommendations?limit=20"

# 查看最近策略版本
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/governance/policy-versions?limit=20"

# 查看 rollout
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/governance/rollouts?limit=20"

# 查看 runtime 决策快照
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/governance/runtime-decisions?limit=20"

# 查看策略分发事件（含激活/回滚事件）
curl -sS -H "X-Admin-Key: ${ADMIN_API_KEY}" \
  "${GATEWAY_BASE_URL}/admin/governance/distribution-events?limit=20"
```

---

## 4. 常见失败定位

### 4.1 `controlplane_runtime` 失败

先看失败停在：

- replay
- compensation replay
- rollback
- policy replay / role / provider / sensitive

对应检查：

- `internal/runtime/publisher.go`
- `internal/runtime/policy_apply.go`
- `internal/runtime/startup_replay.go`
- `internal/httpserver/admin_handler.go`
- `internal/controlplane/service.go`

### 4.2 `chat_policy` 失败

先看失败停在：

- sensitive block
- role deny
- provider deny-all
- preferred model deny
- baseline allow

对应检查：

- `internal/httpserver/server.go`
- `internal/policy/postgres.go`
- `internal/httpserver/chat_completions_policy_live_apply_test.go`
- `cmd/verify/chat_policy/main.go`

### 4.3 `model_governance` 失败

先看失败停在：

- recommendations / approvals / policy versions
- rollouts / rollbacks / dashboard
- runtime-decisions / distribution-events / runtime-observer
- memory candidate facts / project facts / confirm|reject|promote / bulk actions

对应检查：

- `internal/httpserver/server.go`
- `internal/httpserver/model_governance_handler.go`
- `internal/httpserver/model_runtime_handler.go`
- `internal/httpserver/memory_admin_handler.go`
- `cmd/verify/model_governance/main.go`

### 4.4 顶层 smoke 失败
