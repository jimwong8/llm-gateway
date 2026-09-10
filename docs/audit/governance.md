# Governance 模块审计

- 路径：`internal/governance/`
- 规模：42 文件 / 6878 行
- 数据库：13 张治理表（migrations 004-009）
- 测试：每个 service 都有 `_test.go`（多数依赖真 PG）

## 服务全集（11 大服务 + 公共组件）

### 1. RecommendationService（候选生成）
- 文件：`recommendation_repo.go` + `recommendation_service.go`
- 表：`model_recommendations`
- 入口：`Generate(GenerateRecommendationInput) (Recommendation, error)`
- 数据：tenant + agent + reason + 候选模型链

### 2. ApprovalService（审批）
- 文件：`approval_repo.go` + `approval_service.go`
- 表：`model_approvals` + `governance_audit_logs`
- 入口：`Decide(ApprovalInput) (Approval, error)`
- 决策枚举：approve / reject / override
- 副作用：通过 `governanceAuditEmitter` 写 audit 日志（可注入）

### 3. VersionService（策略版本）
- 文件：`version_repo.go` + `version_service.go` + `policy_diff.go`
- 表：`model_policy_versions`
- 入口：
  - `CreateFromApproval(approvalID, createdBy)` — 审批通过 → 创建草稿
  - `Approve(versionID, approvedBy)` — 二次确认
  - `Activate(versionID)` — 激活（驱动 rollout）
  - `GetDiff(versionID)` — 与 base 的 path-level 差异
- 状态机：`draft → approved → active`

### 4. RolloutService（灰度发布）
- 文件：`rollout_repo.go` + `rollout_service.go`
- 表：`model_rollouts` + `model_distribution_events`
- 入口：
  - `Start(StartRolloutInput) → (Rollout, DistributionEvent, error)`
  - `Promote(PromoteRolloutInput) → (Rollout, error)`
- 依赖注入：`rolloutVersionActivator`、`rolloutAuditEmitter`、`rolloutCacheInvalidator`

### 5. RolloutDashboardService（聚合视图）
- 文件：`rollout_dashboard_repo.go` + `rollout_dashboard_service.go`
- 行为：repo `ORDER BY created_at DESC LIMIT N` + 逐行 `RolloutMetricsService.Aggregate`
- 指标：error_rate / p95_latency / fallback_rate / sample_count
- ⚠️ **bug**：测试隔离不足（见 00-overview.md）

### 6. RolloutMetricsService（指标聚合）
- 文件：`rollout_metrics_repo.go` + `rollout_metrics_service.go`
- 表：`runtime_decision_snapshots`
- 入口：`Aggregate(RolloutMetricsQuery) → RolloutMetricsSnapshot`
- 计算口径：
  - error_rate = `count(success=false) / count(*)`
  - fallback_rate = `count(policy_fallback_used=true) / count(*)`
  - p95_latency = latency_ms 百分位

### 7. RollbackService（回滚）
- 文件：`rollback_repo.go` + `rollback_service.go` + `rollback_record_repo.go`
- 表：`model_rollbacks` + `model_distribution_events`
- 入口：`Execute(ExecuteRollbackInput) → ExecuteRollbackResult`
- 内部：通过 `rollbackVersionSwitcher` 切回旧版本、`cacheInvalidator` 清缓存、写 rollback distribution event

### 8. EvaluationService（评估流水线）
- 文件：`evaluation_repo.go` + `evaluation_service.go`
- 表：`evaluation_datasets` / `evaluation_scoring_formulas` / `evaluation_runs` / `evaluation_results`
- 入口：`CreateDataset` / `CreateFormula` / `StartRun` / `UpdateRunStatus`
- 状态机：`running → completed | failed | canceled`

### 9. DriftService（策略漂移检测）
- 文件：`drift_repo.go` + `drift_service.go`
- 表：`policy_drifts`
- 入口：
  - `DetectModelMismatch(input) → (PolicyDrift, bool, error)`
  - `Acknowledge(driftID, reason)`
  - `Resolve(driftID, reason)`
- 状态机：`open → acknowledged | resolved`

### 10. DistributionService（分发事件）
- 文件：`distribution_repo.go` + `distribution_service.go`
- 表：`model_distribution_events`
- 入口：
  - `CreateActivationEvent(rollout)`
  - `CreateRollbackEvent(rollout, actor, reason, restoredVer, revertedVer)`

### 11. RuntimeResolver（运行时入口）
- 文件：`runtime_resolver.go` + `scope_priority.go`
- 表：读 `model_policy_versions`（active），写 `runtime_decision_snapshots`
- 入口：`Resolve(ResolveInput) → ResolveDecision`
- 内部：
  - `loadPolicyCached(env)` 本地缓存 + TTL
  - `buildScopeCandidates(policy, input)` 枚举 scope 匹配项
  - `chooseHighestPriorityScope(matches)` 用 `ScopePriorityOrder() = [project, tenant, template, default]` 排序
  - `resolveModelFromCandidate` 给出最终 model + chain
  - 同步写 snapshot 供 metrics
- `InvalidateCache(env)` 由 RolloutService/RollbackService 在写入后调用

### 12. SnapshotRepo
- 文件：`snapshot_repo.go`
- 表：`runtime_decision_snapshots`
- 入口：`Save(RuntimeDecisionSnapshotWrite)` 由 RuntimeResolver 写入

## 公共组件

### `policy_diff.go`
- `buildPolicyDiff(current, base) []PolicyDiffEntry`
- 算法：`flattenRuntimePolicy` + `walkAny` 递归打平 → `normalizeValue` 类型归一 → `jsonValueEqual` 深比较

### `scope_priority.go`
- 写死优先级 `[project, tenant, template, default]`，与 controlplane 一致

## DB 表 → 服务映射

| 表 | 主写入 | 主读取 |
|---|---|---|
| `model_recommendations` | RecommendationService | RecommendationService.List / ApprovalService |
| `model_approvals` | ApprovalService.Decide | VersionService.CreateFromApproval |
| `model_policy_versions` | VersionService | RolloutService / RuntimeResolver |
| `model_rollouts` | RolloutService | RolloutDashboardService / DistributionService |
| `model_distribution_events` | RolloutService + RollbackService + DistributionService | UI distribution-events |
| `model_rollbacks` | RollbackService | UI rollbacks |
| `policy_drifts` | DriftService | UI drifts |
| `evaluation_*` (4 张) | EvaluationService | UI evaluations |
| `runtime_decision_snapshots` | RuntimeResolver | RolloutMetricsService |
| `governance_audit_logs` | 多 service via emitter | UI audit-events |

## 真实 SQL vs Mock

**全部 service 走真实 SQL** — 没有 mock 路径。测试通过 `GOVERNANCE_TEST_POSTGRES_DSN` 接真库；共享 schema 导致 Bug 1。

## 风险表

| 严重度 | 问题 | 文件 | 一句话修复 |
|---|---|---|---|
| P0 | 测试间数据污染 | `rollout_dashboard_service_test.go` | 加 prefix DELETE 或独立清理 helper |
| P1 | rollout Start 无事务 | `rollout_service.Start` 写 rollouts + distribution_events 分开 | 用 `sql.Tx` 包裹 |
| P2 | metrics 全表扫描 latency | `rollout_metrics_service.Aggregate` | 加 `(rollout_id, created_at)` 复合索引 + 物化视图 |
| P2 | `policy_diff` 无 depth 限制 | `walkAny` 无递归保护 | 加 `maxDepth=32` 防御 |
