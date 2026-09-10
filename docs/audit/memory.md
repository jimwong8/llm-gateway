# Memory 模块审计

- 路径：`internal/memory/`
- 规模：4 文件 / 3841 行（postgres.go 占 2552）
- 表：自创建（`ensureSchema` 在 `NewStore` 中调），无 migration 文件
- ⚠️ 这里有 2 个 known bug（详见 00-overview.md）

## 数据模型

| Item | 表 | 用途 |
|---|---|---|
| `Conversation` + `Message` | `conversations` + `messages` | 会话 + 消息流（带 seq） |
| `SessionSummary` | `session_summaries` | 单会话摘要（goals/done/open/decisions/blockers） |
| `UserPreference` | `user_preferences` | 用户偏好（tenant + user 维度的 key-value） |
| `ProjectFact` | `project_facts` | 项目级事实（promoted from candidate） |
| `CandidateFact` | `candidate_facts` | 候选事实（待治理） |
| `business_audit_logs` 等 | 自动生成 | 操作审计 |

## Store 公开方法（按域分组）

### 写入消息流
- `AppendMessage(ctx, tenantID, userID, sessionID, role, content, tokenCount)`
- `AppendFromRequest(ctx, providers.ChatCompletionRequest)`
- 内部：`appendMessagesTx` 事务 + `updateConversationCacheAsync` 异步刷 redis

### 读取消息流
- `Recent(ctx, tenantID, userID, sessionID, limit)`
- `GetMessages(ctx, sessionID, cursorSeq, limit, direction)` - cursor 分页
- `GetMessagesAroundAnchor(ctx, sessionID, anchorSeq, limit)` - anchor 上下文
- `GetMessagesAfterSeq(ctx, sessionID, afterSeq, limit)`
- `GetConversation(ctx, tenantID, userID, sessionID)`

### Session summary
- `GetSessionSummary(ctx, tenantID, userID, sessionID)`
- `UpsertSessionSummary(ctx, summary)`
- `RefreshSessionSummary(ctx, tenantID, userID, sessionID)` - 从 messages 重算
- 内部：`applySessionSummaryRules`、`extractStrongGoal/CompletedItem/OpenItem/KeyDecision/Blocker/ResolvedBlocker`、`pruneSupersededProjectFactMentions`、`pruneRejectedCandidateFactMentions`

### User preferences
- `GetUserPreferences(ctx, tenantID, userID)`
- `UpsertUserPreference(ctx, pref)`
- `FormatUserPreferences(prefs)` 给 LLM 注入

### Project facts（已促晋为正式事实）
- `ListProjectFacts(ctx, tenantID, userID, status)`
- `GetProjectFacts(ctx, tenantID, userID)`
- `UpsertProjectFact(ctx, fact)`
- `getProjectFacts(ctx, tenantID, userID, includeSuperseded)` 内部支持 superseded 过滤

### Candidate facts（待治理事实）
- `GetCandidateFacts(ctx, tenantID, userID)`
- `ListCandidateFacts(ctx, tenantID, userID, status)`
- `UpsertCandidateFact(ctx, fact)` ← **bug 现场**
- `ConfirmCandidateFact(ctx, tenantID, userID, factKey)` → 状态: pending → confirmed
- `RejectCandidateFact(ctx, tenantID, userID, factKey)` → pending → rejected
- `PromoteCandidateFact(ctx, tenantID, userID, factKey)` → confirmed → promoted (并写到 project_facts)
- `PromoteCandidateFacts(ctx, tenantID, userID)` 批量
- 内部：`transitionCandidateFactStatus`、`getCandidateFactByKeyInTx`、`isCandidateFactTransitionAllowed`、`shouldPromoteCandidateFact`

### Search / hybrid recall
- `SearchMessages(ctx, tenantID, query, limit, offset)` - FTS
- `HybridSemanticRecall(ctx, tenantID, userID, sessionID, query, ftsTopK, semanticTopK, finalTopK)`
- 内部：`searchMessagesFTSForHybrid`、`searchMessagesSemanticLiteForHybrid`、`tokenizeSemanticRecall`、`semanticTokenOverlapScore`、`compactSemanticRecallSnippet`

### 删除/导出
- `DeleteConversation(ctx, tenantID, sessionID, actorID)`
- `DeleteMessage(ctx, tenantID, sessionID, seq, actorID)`
- `ExportConversation(ctx, tenantID, sessionID, actorID)`

## State machine：CandidateFact

```
pending  ──confirm──▶ confirmed ──promote──▶ promoted (写 project_facts + superseded 旧的)
   │                       │
   ├──reject──▶ rejected   ├──reject──▶ rejected
   │
   └──auto-promote (shouldPromoteCandidateFact 判断 confirmation_count ≥ 阈值)
```

`isCandidateFactTransitionAllowed(current, target)` 是闸门。

## 缓存协作

`updateConversationCacheAsync(conversationID, lastSeq, normalized, startSeq)`：
1. 计算 conversationKey + recent 数组
2. **fork goroutine**：
   - `s.cache.CacheConversationMeta(metaKey, {LastSeq, UpdatedAt})`
   - `s.cache.CacheRecentMessages(metaKey, recent, 50)`

`refillConversationMetaCacheAsync` / `refillRecentMessagesCacheAsync` / `invalidateConversationCacheAsync` 类似结构。

⚠️ **Bug 2**：上述 goroutine 直接访问 `s.cache.XXX(...)`，但单元测试用 `&Store{db: ...}` 构造时 `cache` 为 nil → panic。

### 修复 patch（建议）

```go
// internal/memory/postgres.go around line 431
func (s *Store) updateConversationCacheAsync(ctx context.Context, conversationID, lastSeq int64, normalized []messageToAppend, startSeq int64) {
    if s.cache == nil {  // ← 加这一行
        return
    }
    // ... rest unchanged
}
```

或更彻底——把 `s.cache` 类型改成 `conversationCache`（已有 interface 在 line 126），然后在测试里注入 NoopCache。

## 测试覆盖

- `session_summary_test.go` (132 行) — summary 规则单元测试
- `user_project_memory_test.go` (1157 行) — 集成测试，依赖真 PG（覆盖 user prefs / project facts / candidate facts / promotion / search / recall）
- `MemoryGovernancePage` 后端集成（`internal/httpserver/memory_admin_handler_test.go` 425 行）

## 性能 / 风险

| 严重度 | 问题 | 文件 | 修复 |
|---|---|---|---|
| P0 | `updateConversationCacheAsync` nil panic | postgres.go:431 | 1 行 nil-check |
| P0 | `UpsertCandidateFact` upsert 行为 | postgres.go:2002 | 看测试期望 → 复审 ON CONFLICT 子句 |
| P1 | `ensureSchema` 自创建表 | 无 migration 文件 | 把 schema 抽到 migrations 目录（与 governance 一致） |
| P1 | `appendMessagesTx` 长事务 + 同步 audit 写入 | postgres.go:343 | audit 改异步 |
| P2 | `HybridSemanticRecall` 多次 SQL + 内存合并 | postgres.go:1324 | 单条 SQL with CTE，或物化视图 |
| P2 | `extractByLeadingMarkers` / `extractByContainingMarkers` 大量字符串扫描 | summary 规则 | 编译为 regex；或 token 化预处理 |
