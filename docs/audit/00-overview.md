# LLM Gateway 模块审计总览

- 日期：2026-05-15
- 审计 head：`5a2d6a0` on `feat/model-governance-platform`
- 验证环境：10.100.1.13 PostgreSQL pgvector/pg16 + Redis 7
- 状态：smoke_plus 4/4 PASS / go test 13/15 PASS（2 known bug 已根因定位）

## 模块清单

| 模块 | 行数 | 文件数 | 审计文件 | 关键产物 |
|---|---:|---:|---|---|
| httpserver | 9991 | ~30 | `httpserver.md`（待补） | 60+ 路由汇聚、admin 鉴权、chat completions 全流程 |
| governance | 6878 | 42 | [`governance.md`](./governance.md) | 治理服务全集（recommendation→approval→version→rollout→rollback→drift→evaluation→distribution→runtime_resolver→scope_priority→policy_diff） |
| runtime | 4002 | 17 | [`runtime.md`](./runtime.md) | bus + publisher + reload + bridge + 三类 apply（policy/router/quota）+ startup_replay |
| memory | 3841 | 4 | [`memory.md`](./memory.md) | session_summary / user_prefs / candidate_facts / project_facts / 自创建 schema |
| controlplane | 1374 | 12 | [`controlplane.md`](./controlplane.md) | 配置版本生命周期 draft→released→promoted、scope precedence、compensation/rollback |
| web/admin | TS+TSX | 17 page | [`frontend.md`](./frontend.md) | React 18 + RQ 5 + react-router 6，已与后端 1:1 对齐（少量缺口见报告） |

## 两个 known bug 根因（已定位）

### Bug 1：`governance.TestRolloutDashboardServiceListRows`

- 错误：`expected latest rollout first, got rollout_rollback_seam`
- 根因：**测试间数据污染**。`model_rollouts` 表被多个 `_test.go` 共用，`rollback_service_test.go` 插入的 `rollout_rollback_seam` 在 `Limit:2 ORDER BY created_at DESC` 时混入。
- 不是 sort 逻辑问题（`ORDER BY created_at DESC` 正确）。
- 修复方案（择一）：
  1. 测试开头 `DELETE FROM model_rollouts WHERE rollout_id LIKE 'rollout_dashboard_%' OR rollout_id LIKE 'rollout_rollback_%'` 显式清理
  2. 给 `ListRecentRollouts` 加可选 prefix/env 过滤参数，测试用 prefix 隔离
  3. 改用 `governance_test.SetupCleanRollouts(t)` helper 包裹

### Bug 2：`memory.TestStoreCandidateFactsUpsertAndGetIntegration` + nil panic

- 错误 1：`expected 2 candidate facts, got 1` —— 上游 upsert 行为差异（待具体 trace）
- 错误 2：`runtime error: invalid memory address or nil pointer dereference`
  - 位置：`internal/cache/redis.go:92` `(*RedisCache).CacheConversationMeta` 被 `nil` 接收者调用
  - 调用链：`internal/memory/postgres.go:431` `updateConversationCacheAsync` 启 goroutine → `s.cache.CacheConversationMeta(...)`
  - 根因：测试构造 `*memory.Store` 时 cache 为 nil，但 goroutine 入口未 nil-check
- 修复（一行）：在 `updateConversationCacheAsync` 的 goroutine 起始处加：
  ```go
  if s.cache == nil {
      return
  }
  ```

## P0 / P1 修复建议

### P0（阻塞 CI）
- [ ] memory nil panic：1 行修复
- [ ] rollout dashboard 测试隔离

### P1（治理域闭环）
- [ ] candidate-fact UPSERT 行为对齐预期
- [ ] 统一所有 governance `_test.go` 的清理 helper
- [ ] httpserver 路由治理：见 frontend.md 中"后端有路由前端无页面"那 11 条

### P1（工程化）
- [ ] CI（GitHub Actions）：smoke_plus + go test ./... + vitest
- [ ] Docker 多阶段镜像
- [ ] 集中日志（slog 替换 `log.Printf`）+ Prometheus metrics

## 与 docs/plans/ 的对照

- `policy-engine-hardening-design` → policy_apply.go 实现完整
- `runtime-hot-reload-design` → runtime/{bus,publisher,reload,bridge,*_apply}.go 实现完整
- `config-versioning-and-rollback-design` → controlplane/* 实现完整
- `multi-instance-config-sync-design` → 当前只有 `InProcessBus`，**多实例 bus 未实现** ← P2
- `message-bus-selection-design` → 选型文档存在，未引入消息中间件 ← P2

## 下一步

1. 把 PR #1 中的 2 known bug 修了（开新 PR）
2. 修完合 master
3. 启动 CI
4. 排期 P2（消息总线、多实例同步、Qdrant 接入）
