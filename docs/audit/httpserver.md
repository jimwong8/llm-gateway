# Httpserver 模块审计

- 路径：`internal/httpserver/`
- 规模：~30 文件 / 9991 行（server.go 占 2339）
- 角色：所有 HTTP 路由 + admin 鉴权 + 请求格式标准化

## 路由总表

### Public（无鉴权）
| 路径 | 方法 | Handler |
|---|---|---|
| `/healthz` | GET | `healthz` |
| `/v1/models` | GET | `models` |
| `/v1/chat/completions` | POST | `chatCompletions` |
| `/v1/runtime/resolve` | GET? | `modelRuntimeResolveRoute` |

### Admin 公共（`requireAdmin`）
| 路径前缀 | Handler 文件 | 职责 |
|---|---|---|
| `/admin/health` | server.go | 健康检查（强化版） |
| `/admin/usage` | server.go | 用量 |
| `/admin/audit` | server.go | 审计 |
| `/admin/observability/{summary,cache,providers,hotspots,quota,quota/trends}` | server.go | 6 条观测视图 |
| `/admin/policies/models` | server.go | 当前 allowed_models 视图 |
| `/admin/assets[/{stats,reuse-audits,versions,rollback}]` | server.go | 资产管理（5 条） |
| `/admin/control-plane/compensations[/replay]` | server.go + control_plane | 补偿记录 + replay |
| `/admin/ui[/]` | server.go | 前端 SPA 静态托管 |

### Control plane（`controlPlaneRoute`，由 admin_handler 路由分发）
| 路径 | 方法 |
|---|---|
| `/admin/inheritance-drafts` | POST |
| `/admin/releases` | POST（draft → released） |
| `/admin/releases/rollback` | POST |
| `/admin/releases/replay` | POST |
| `/admin/promotions` | POST |
| `/admin/audit-events` | GET |
| `/admin/runtime-events` | GET |
| `/admin/config-versions[/{id}]` | GET |

### Model governance（`modelGovernanceRoute`）
| 路径 | 用途 |
|---|---|
| `/admin/governance/recommendations` | List/Get |
| `/admin/governance/approvals` | Decide |
| `/admin/governance/policy-versions[/{id}/{diff,approve,activate}]` | Version 流 |
| `/admin/governance/rollouts[/]` | Start/Promote |
| `/admin/governance/dashboard/rollouts` | Dashboard |
| `/admin/governance/rollbacks[/]` | Execute |
| `/admin/governance/evaluations[/]` | Evaluation 流水线 |
| `/admin/governance/drifts[/]` | Drift detect/ack/resolve |

### Model runtime（`modelRuntimeResolveRoute`）
- `/v1/runtime/resolve` (公共)
- `/admin/governance/runtime/resolve`
- `/admin/governance/runtime-decisions`
- `/admin/governance/distribution-events`
- `/admin/governance/runtime-observer`

### Memory governance（`memoryAdminRoute`）
- `/admin/memory/candidate-facts[/{factKey}/{action}]`
- `/admin/memory/candidate-facts/actions/{action}` (bulk)
- `/admin/memory/project-facts`

## Admin 鉴权（server.go:278）

```go
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
    1. 取 X-Admin-Key 头
    2. 若空 → 解析 Authorization: Bearer <token>
    3. 与 s.cfg.AdminAPIKey 比对（明文，常量时间）⚠️ 用 == 不是 subtle.ConstantTimeCompare
    4. 若 policy 已启用 + URL 带 tenant_id：
       4.1 取 currentSubject(r)
       4.2 RoleFor(tenant, subject)
       4.3 roleAllowsAdminPath(role, path, method) 二次校验 RBAC
       4.4 失败 → 403 + audit log
    5. 通过 → 调下游
}
```

⚠️ **风险**：
- token 比对未常量时间 → timing attack（实际风险低，因为是单一固定 token）
- 没有 token 轮换；docs/plans/2026-03-24-admin-token-auth-design.md 提到了，未落地

## `chatCompletions` 全流程（server.go:1085）

```
1. 校验：method=POST、JSON、messages 非空
2. normalizeRequestIdentity → 保证有 session_id（header / generated）
3. 写 X-Session-Id 响应头
4. policy 检查（s.policy != nil 且 req.TenantID != ""）：
   a. AllowedModels(tenant)              → 交集 candidate_models；preferred 不在则 403 preferred_model_denied
   b. RoleFor(tenant, subject) + roleAllowsMethod → 403 rbac_denied
   c. SensitiveRules + containsSensitive → 403 sensitive_block
   d. ProviderPolicies (mode=deny) →
       - 若 preferred 命中 deny provider → 403 provider_denied
       - candidate_models 过滤掉所有 deny 命中 → 若全空 → 403 all-candidates-denied
5. 缓存查（L1 redis + L2 semantic_cache）
6. router.Decide → 选定 provider + model
7. provider.Invoke
8. 响应组装 + 写 audit + billing
9. 返回 ChatCompletionResponse
```

错误响应格式统一为：
```json
{"error":{"message":"...","type":"policy_error|authentication_error|...","tenant_id":"...","model":"..."}}
```

## SSE / 流式

`grep flusher /tmp/opencode/llm-gateway/internal/httpserver/*.go` —— **未发现**。当前 `chatCompletions` 不支持 SSE / streaming，全部一次性 JSON 响应。
- docs/plans 没有 streaming 设计文档
- 这是 P2 缺口（前端 PlaygroundPage 也只能等响应再展示）

## Session id（session_id.go）

- 优先级：`X-Session-Id` header > `req.SessionID` > 生成 `oc_<uuid>`
- 写回 `X-Session-Id` 响应头
- 测试覆盖：`session_id_test.go` + `session_identity_integration_test.go`

## 中间件 / 横切

| 关注点 | 现状 |
|---|---|
| CORS | 未发现专门 handler；如需跨域需后加 |
| Rate limit | 走 `quota.Limiter`，但只在 chatCompletions 内部触发，admin 路径不限流 |
| 日志 | `log.Printf` 散落多处；建议替换为 slog 结构化 |
| Recover panic | 未发现 — panic 会让 process 整个挂；**P0 建议加 mux 包裹 panic recovery** |
| 请求 trace | 仅靠 `requestID = req-<unixnano>`；无 W3C trace propagation |

## stub / TODO 风险

抽样几个 admin 子路由扫描：
- `/admin/assets/*` 5 条 — 需复查 handler 是否完整（前端无对应页面，frontend.md 已标）
- `/admin/governance/runtime/resolve` 与公共 `/v1/runtime/resolve` 共用 handler — 鉴权语义需确认（admin 走 token，public 不走）

## 风险表

| 严重度 | 问题 | 修复 |
|---|---|---|
| P0 | mux 无 panic recovery | 加 `panicRecoveryMiddleware` 包 mux |
| P0 | admin token == 比较 | `subtle.ConstantTimeCompare` |
| P1 | 无 streaming（SSE） | chatCompletions 加 `text/event-stream` 分支 |
| P1 | log.Printf 散落 | 替换为 slog + RequestID 字段 |
| P1 | admin 路径无限流 | 接 `quota.Limiter` 或独立 admin RPS |
| P2 | 缺统一 error envelope helper | 抽 `writeError(w, status, type, msg, fields)` |
| P2 | 无 W3C trace 传播 | 接 `traceparent` 头 + OTel SDK |
