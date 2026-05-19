# 10.100.1.13 会话记忆与会话中心系统 — 完整研究报告

> 服务地址：http://10.100.1.13:8000
> 研究时间：2026-05-17
> 数据来源：4 个并行子代理 + 直接 API 探测 + Playwright 浏览器自动化

---

## 一、系统总览

**Session Memory Dashboard v2** 是一个生产级 AI 会话记忆管理系统，为 OpenCode 等 AI 助手提供跨会话的持久化记忆能力。

### 服务拓扑

| 端口 | 服务 | 技术栈 | 状态 |
|------|------|--------|------|
| **8000** | Session Memory API | FastAPI / Python 3.11 / uvicorn | ✅ 运行中 |
| **3000** | Grafana 监控面板 | Grafana v13.0.1 | ✅ 运行中 |
| **8080** | 未知服务 | Express 风格 | ❓ 存活但无暴露路由 |

### 数据库规模（实时）

| 指标 | 数量 |
|------|------|
| 活跃会话 | **1,395** |
| 归档会话 | 8 |
| 消息总数 | **180,063** |
| 摘要数 | **462** |
| KG 实体 | **125,635** |
| KG 关系 | **318,807** |
| KG 作业成功 | 72,928 |
| KG 死信 | 1 |

---

## 二、前端页面结构

### 2.1 Dashboard（主页 `/`）

运维管理仪表盘，包含：

| 模块 | 功能 |
|------|------|
| **LLM 默认模型配置** | 配置 5 个字段：OpenAI Model、Summary Model、Backup Chat、Backup Summary、Routing Mode |
| **Status Grid（5 卡片）** | Health / Continuation / KG Extraction / Alerts / KG Jobs 实时状态 |
| **Issues 面板** | 聚合所有模块的异常项 |
| **Capacity Snapshot** | 活跃会话、总消息数、KG Pending、KG Deadletter |
| **Shared Memory 面板** | 跨会话共享统计 |
| **KG Ops Realtime** | 实时 KG 队列状态 |
| **Ops Actions** | 运维操作按钮（死信重入、队列清空、KG Worker 等） |
| **Operation Feedback** | 操作反馈中心（idle/loading/success/error 四态） |
| **Operation History** | 最近操作历史（支持本地回退） |
| **Continuation Ops** | 续跑运维面板 |
| **Homepage Admin Overview** | AI Ops Advice + OpenCode Runtime + Model Routing + Operation History |

### 2.2 会话中心（`/app`）

高融合工作台，SPA 风格，6 个子面板：

| 面板 | 功能 |
|------|------|
| **Overview** | 系统总览与事件流 |
| **Search** | 全文检索（支持 Hybrid/Keyword/Vector 三种策略） |
| **Resume** | 恢复包与上下文恢复（只读） |
| **Session Detail** | 会话详情与消息查看（只读） |
| **Sessions** | 会话列表（基于 `/api/v1/sessions/recent`） |
| **Self-Check** | 接口契约与封板状态自检 |
| **Memory Pyramid** | 三层记忆可视化（L3 Persona / L2 Scenarios / L1 Atoms + Canvas） |

---

## 三、核心数据模型 — 记忆金字塔

这是系统最核心的创新——**三层记忆金字塔架构**：

```
              ┌─────────────────────┐
              │   L3: Persona       │  ← 用户画像 + 偏好
              │   用户长期画像       │
              ├─────────────────────┤
              │   L2: Scenarios     │  ← 场景叙事摘要
              │   会话场景叙述       │     (关联多个 Atom)
              ├─────────────────────┤
              │   L1: Atoms         │  ← 细粒度记忆原子
              │   原子级记忆单元     │     goal/fact/decision/constraint/...
              └─────────────────────┘
                        ↓
              ┌─────────────────────┐
              │   L1: Canvas        │  ← Mermaid 可视化
              │   会话画布           │
              └─────────────────────┘
```

### L1 — MemoryAtom（记忆原子）

9 种类型：`goal`、`decision`、`constraint`、`fact`、`task_state`、`blocker`、`error_pattern`、`preference`、`project`

```json
{
  "id": "uuid",
  "session_id": "uuid",
  "user_id": "jimwong",
  "kind": "goal",
  "title": "询问WebProcess高CPU原因",
  "content": "...",
  "tags": ["WebProcess", "高CPU原因"],
  "source_message_ids": ["uuid"],
  "confidence_score": 0.8,
  "sensitivity_level": "normal",
  "dedup_signature": "ff2e0ca0e10c",
  "superseded_by": null,
  "created_at": "2026-05-16T20:02:19Z"
}
```

### L2 — Scenario（场景）

```json
{
  "id": "uuid",
  "user_id": "jimwong",
  "title": "用户报告CPU高负载，助理诊断并修复",
  "narrative_md": "- **用户目标**：...\n- **结果**：...",
  "atom_ids": ["uuid", "uuid"],
  "session_ids": ["uuid"],
  "period_start": "2026-05-16T20:02:19Z",
  "period_end": "2026-05-16T20:02:19Z"
}
```

### L3 — Persona（用户画像）

```json
{
  "user_id": "jimwong",
  "profile_md": "该用户是一名技术专家，专注于系统运维...",
  "preferences_json": {
    "language": "zh",
    "work_focus": "全栈/运维/后端",
    "common_patterns": ["并行调查多路线索", "基于证据链的根因分析"],
    "tool_preferences": ["OpenCode", "Linux CLI", "Docker", "FastAPI"],
    "communication_style": "技术化、文档化、详细"
  },
  "scenario_ids": ["uuid", "uuid"],
  "version": 36
}
```

### Session（会话）

```json
{
  "id": "uuid",
  "user_id": "jimwong",
  "title": "opencode:ses_1c9cd9e54ffel6xVAj7pFmLzME",
  "visibility": "private",
  "archived": false,
  "total_tokens": 128,
  "message_count": 9,
  "metadata_json": {
    "host": "jimwong-standardpc",
    "mode": "tool-audit",
    "source": "session-memory-auto",
    "opencode_session_id": "ses_1c9cd9e54ffel6xVAj7pFmLzME"
  }
}
```

### Message（消息）

```json
{
  "id": "uuid",
  "session_id": "uuid",
  "role": "assistant",
  "content": "...",
  "tokens": 101,
  "metadata_json": {
    "kg_extract_status": "pending",
    "kg_extract_attempts": 0,
    "content_hash": "fef7b4e0..."
  }
}
```

### Context Stats（上下文统计）

```json
{
  "context_window_tokens": 140,
  "context_utilization": 0.0175,
  "budget_state": "normal",
  "budget_ratio": 0.0175,
  "summary_enabled": true,
  "summary_available": false,
  "graph_hits": 0,
  "graph_context_tokens": 0,
  "retrieved_tokens": 0,
  "recent_tokens": 134,
  "reply_reserve_tokens": 2000
}
```

---

## 四、完整 API 端点清单（55 个）

### 4.1 基础设施

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/metrics` | Prometheus 指标 |
| GET | `/openapi.json` | OpenAPI 3.1 规范 |
| GET | `/docs` | Swagger UI |

### 4.2 会话管理（20 个端点）

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/sessions?user_id={id}&limit=20&offset=0` | 列出会话 |
| POST | `/api/v1/sessions` | 创建会话 |
| GET | `/api/v1/sessions/recent` | 最近会话 |
| GET | `/api/v1/sessions/{id}` | 会话详情 |
| DELETE | `/api/v1/sessions/{id}` | 删除会话 |
| POST | `/api/v1/sessions/{id}/archive` | 归档 |
| POST | `/api/v1/sessions/{id}/unarchive` | 取消归档 |
| GET | `/api/v1/sessions/{id}/export?format=json\|markdown` | 导出 |
| GET/POST | `/api/v1/sessions/{id}/messages?limit=50&before=` | 消息列表/添加 |
| POST | `/api/v1/sessions/{id}/chat` | LLM 对话 |
| POST | `/api/v1/sessions/{id}/search` | 语义搜索 |
| POST | `/api/v1/sessions/{id}/summarize` | 触发摘要 |
| GET | `/api/v1/sessions/{id}/summary-status` | 摘要状态 |
| GET | `/api/v1/sessions/{id}/context-stats` | 上下文统计 |
| GET/PUT | `/api/v1/sessions/{id}/canvas` | 画布 |
| POST | `/api/v1/sessions/{id}/canvas/rebuild` | 重建画布 |
| GET | `/api/v1/sessions/{id}/atoms` | 会话原子 |
| GET | `/api/v1/sessions/{id}/kg/entities` | 会话 KG 实体 |
| GET | `/api/v1/sessions/{id}/kg/relations` | 会话 KG 关系 |
| POST | `/api/v1/sessions/{id}/kg/search` | 会话 KG 搜索 |

### 4.3 知识图谱（9 个端点）

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/kg/entities?limit=&offset=&entity_type=&sort=` | 实体列表 |
| GET | `/api/v1/kg/entities/by-visibility` | 按可见性 |
| GET | `/api/v1/kg/relations?limit=&offset=` | 关系列表 |
| GET | `/api/v1/kg/relations/by-visibility` | 按可见性 |
| GET | `/api/v1/kg/graph?limit=` | 完整图谱 |
| GET | `/api/v1/kg/graph/by-visibility` | 按可见性 |
| POST | `/api/v1/kg/search` | KG 搜索 |
| GET | `/api/v1/kg/entity/{entity_name}/sessions` | 实体关联会话 |
| GET | `/api/v1/kg/relation/{relation_type}/sessions` | 关系关联会话 |

### 4.4 用户/记忆（5 个端点）

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/users/{uid}/persona` | L3 用户画像 |
| GET | `/api/v1/users/{uid}/scenarios` | L2 场景列表 |
| GET | `/api/v1/users/{uid}/atoms?kind=&limit=` | L1 记忆原子 |
| GET | `/api/v1/users/{uid}/memory/export` | 导出记忆 |
| POST | `/api/v1/users/{uid}/memory/import` | 导入记忆 |

### 4.5 插件状态（8 个端点）

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/plugin-state/continuation` | 保存续传状态 |
| GET | `/api/v1/plugin-state/continuation/latest` | 最新续传 |
| GET | `/api/v1/plugin-state/continuation/{sid}` | 指定续传 |
| POST | `/api/v1/plugin-state/continuation/{sid}/consume` | 消费续传 |
| POST | `/api/v1/plugin-state/message-state` | 保存消息状态 |
| GET | `/api/v1/plugin-state/message-state/{sid}/{mid}` | 查询消息状态 |
| POST | `/api/v1/plugin-state/session-map` | 保存会话映射 |
| GET | `/api/v1/plugin-state/session-map/{oc_sid}` | 查询会话映射 |

### 4.6 管理控制台（10 个端点）

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/admin/dashboard` | 聚合仪表盘 |
| GET/POST | `/api/v1/admin/config/model` | 模型配置 |
| POST | `/api/v1/admin/metrics/continuation-event` | 续传指标事件 |
| POST | `/api/v1/admin/ops/reconcile-continuation-metrics` | 对账续传指标 |
| POST | `/api/v1/admin/ops/repair-stuck-session` | 修复卡住会话 |
| POST | `/api/v1/admin/ops/requeue-kg-deadletters` | 重入 KG 死信 |
| POST | `/api/v1/admin/ops/run-kg-worker-once` | 运行 KG Worker |
| GET | `/api/v1/admin/raw-events/stats` | 原始事件统计 |
| POST | `/api/v1/admin/runtime-events/opencode` | OpenCode 运行时事件 |

### 4.7 其他（3 个端点）

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/query` | 统一查询 |
| GET | `/api/v1/projects/{pid}/search/kg` | 项目 KG 搜索 |
| GET | `/api/v1/projects/{pid}/search/summaries` | 项目摘要搜索 |

---

## 五、搜索系统

支持三种搜索策略：

| 策略 | 算法 | 说明 |
|------|------|------|
| `hybrid` | BM25 + 向量 + RRF | 混合搜索（默认） |
| `keyword` | BM25 | 关键词搜索 |
| `vector` | 向量 | 语义向量搜索 |

向量模型：`paraphrase-multilingual-MiniLM-L12-v2`（384 维，本地部署）

搜索范围：会话内搜索 + 跨会话搜索 + KG 搜索 + 统一查询

---

## 六、上下文预算系统

```
normal → warning → critical → stall/truncate
```

关键指标：
- `context_utilization`：0~1 浮点数，当前窗口利用率
- `budget_state`：4 级状态机
- `budget_ratio`：预算比率
- 融合 `summary + graph + recent + retrieved + reply_reserve` 五类 token

---

## 七、可复用的设计模式

### 7.1 记忆金字塔（L1→L2→L3）

```
L3 Persona ← 用户长期画像（profile_md + preferences_json）
    ↑
L2 Scenarios ← 跨会话场景叙事（narrative_md + atom_ids）
    ↑
L1 Atoms ← 细粒度记忆原子（9 种 kind，置信度评分，去重签名）
    ↑
Canvas ← Mermaid 可视化（从 atoms 动态重建）
```

### 7.2 上下文预算状态机

```
normal → stretched → critical → stall
```

双阈值控制：`context_utilization` + `budget_state`

### 7.3 续会话（Continuation）机制

```
OpenCode 中断 → ContinuationState（失败码 + 最近消息）
    → session-memory 存储 → 恢复时 consume
```

### 7.4 双源历史回退

```
if (API 返回操作历史) → 使用 API 数据
if (API 没有) → 使用本地内存
if (执行新操作且 source=api) → 清空 API 数据切回本地
```

### 7.5 网络抖动弹性

```
transient 错误 → 显示 "recovering" + 自动重试
fatal 错误 → 显示 "error" + 红色提示
```

### 7.6 阶段驱动状态机（Ops Feedback）

```
idle → loading → success/error → idle
```

每阶段有完整 theme 配置（badge / statusText / messageBox）

### 7.7 响应标准化管道

```javascript
normalizeOperationHistoryEntry(entry) {
  action = action_label || action || operation || name
  endpoint = endpoint || url || path || api_endpoint
  phase = normalizeHistoryPhase(phase || status || state)
  summary = summary || result_summary || message || detail
  timestamp = normalizeTimestamp(timestamp || created_at)
}
```

### 7.8 会话选择联动（pub/sub）

```
setSelectedSession(session) →
  更新 appState → 重新渲染 6 个面板
  → Search / Resume / Session Detail / Memory 全部刷新
```

---

## 八、LLM 路由系统

当前配置：

| 模型 | 角色 | 窗口 Hits | 历史总 Hits |
|------|------|-----------|-------------|
| DeepSeek-V3.2-EXP | 主聊天 + 摘要 | 96 | 6,735 |
| DeepSeek-R1-0528-Qwen3-8B | 备援聊天 + 摘要 | 90 | 5,574 |

路由模式：`load_balance`（请求级轮询）

---

## 九、对 llm-gateway 的借鉴建议

### 可直接复用的设计

| 设计 | 说明 | 优先级 |
|------|------|--------|
| **记忆金字塔** | L1 Atoms → L2 Scenarios → L3 Persona | 🔴 P0 |
| **上下文预算系统** | budget_state + context_utilization | 🔴 P0 |
| **语义搜索** | Hybrid BM25 + Vector + RRF | 🔴 P0 |
| **续会话机制** | ContinuationState + consume | 🟡 P1 |
| **双源历史回退** | API + 本地内存 | 🟡 P1 |
| **阶段驱动状态机** | idle → loading → success/error | 🟡 P1 |
| **网络抖动弹性** | transient vs fatal 区分 | 🟡 P1 |
| **响应标准化管道** | normalizeXxxEntry | 🟢 P2 |
| **会话选择联动** | pub/sub 风格 | 🟢 P2 |
| **操作反馈面板** | 4 阶段状态 + 结果预览 | 🟢 P2 |
| **自检系统** | Deep Health + 接口契约 | 🟢 P2 |

### 可直接复用的 API 设计

| API 模式 | 示例 | 说明 |
|----------|------|------|
| **聚合端点** | `/api/v1/admin/dashboard` | 一次返回所有仪表盘数据 |
| **状态摘要** | `/sessions/{id}/summary-status` | 状态检查端点 |
| **上下文统计** | `/sessions/{id}/context-stats` | 细粒度预算管理 |
| **语义搜索** | `/sessions/{id}/search` | 3 种策略 |
| **操作端点** | `/api/v1/admin/ops/*` | POST-only，幂等设计 |
| **插件状态** | `/api/v1/plugin-state/*` | 批量上报 + 单条查询分离 |
| **记忆导出** | `/users/{uid}/memory/export` | tar.gz 全量导出 |

---

## 十、总结

**10.100.1.13 的会话记忆系统** 是一个生产级 AI 记忆管理平台，核心创新在于：

1. **三层记忆金字塔**：从用户画像到场景叙事再到知识原子，形成完整的记忆层级
2. **上下文预算引擎**：4 级状态机控制上下文窗口
3. **混合搜索流水线**：BM25 + 向量 + RRF 融合
4. **续会话恢复**：完整的 Continuation 机制
5. **弹性设计**：网络抖动恢复、双源历史回退、阶段驱动状态机

当前运行规模：**1,395 活跃会话、180K 消息、125K KG 实体、318K KG 关系**。

所有设计模式和 API 均可直接借鉴到 llm-gateway 项目中，无架构冲突。
