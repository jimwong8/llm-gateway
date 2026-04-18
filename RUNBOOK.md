# LLM Gateway Runbook

## 1. 日常验证入口

### 1.1 顶层总 smoke

```bash
go run ./cmd/verify/smoke
```

用途：一键验证当前仓库最关键的两条闭环：

- control-plane -> runtime apply
- control-plane replay -> request-path policy enforcement

期望输出：

- `[PASS] controlplane_runtime`
- `[PASS] chat_policy`
- `verify result: PASS smoke(controlplane_runtime,chat_policy)`

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
go run ./cmd/verify/smoke
go test ./...
```

---

## 3. 常见失败定位

### 3.1 `controlplane_runtime` 失败

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

### 3.2 `chat_policy` 失败

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

### 3.3 顶层 smoke 失败

顶层命令只是编排器。优先直接重跑失败的子命令：

```bash
go run ./cmd/verify/controlplane_runtime
# 或
go run ./cmd/verify/chat_policy
```

不要先怀疑 `cmd/verify/smoke` 本身，除非两个子命令单跑都正常但总入口失败。
