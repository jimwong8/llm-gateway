# LLM Gateway — 企业级 AI 大模型中转站

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript)](https://www.typescriptlang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> 带长期知识资产层的企业级 LLM Gateway。对外提供与 OpenAI 兼容的 API 网关能力，对内屏蔽不同模型供应商之间的接口差异、鉴权差异、限流差异和计费差异。

## 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                      Admin Console                          │
│  React 18 + TypeScript + Vite + Recharts + TanStack Query    │
│  /admin/ui                                                  │
└──────────────────────────┬──────────────────────────────────┘
                           │ REST API
┌──────────────────────────▼──────────────────────────────────┐
│                    LLM Gateway Server                        │
│  Go 1.22+ · slog JSON logging · single binary               │
│                                                             │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │  Control     │  │  Policy      │  │  Model Governance │  │
│  │  Plane       │  │  Engine      │  │  Platform         │  │
│  └──────┬──────┘  └──────┬───────┘  └────────┬──────────┘  │
│         │                │                    │             │
│  ┌──────▼────────────────▼────────────────────▼──────────┐  │
│  │                   Router Engine                        │  │
│  │  Capability/Cost/Latency/Health weighted scoring       │  │
│  │  Fallback chain · Cross-model reuse · Semantic cache   │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          │                                  │
│  ┌───────────────────────▼───────────────────────────────┐  │
│  │                 Provider Adapters                      │  │
│  │  OpenAI · Anthropic · Google · Azure · AWS · Custom   │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ L1 Exact │  │ L2 Sem.  │  │ L3 Session│  │ L4 Asset │   │
│  │ Cache    │  │ Cache    │  │ Memory   │  │ Reuse    │   │
│  │ (Redis)  │  │ (Qdrant) │  │ (PG)     │  │ (PG)     │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
          │                │                │
    ┌─────▼─────┐   ┌─────▼─────┐   ┌─────▼─────┐
    │  Redis    │   │  Qdrant   │   │ PostgreSQL │
    │  (L1缓存) │   │ (语义缓存)│   │ (L3/L4/治理)│
    └───────────┘   └───────────┘   └───────────┘
```

## 核心特性

### 网关内核 (P1)
- **OpenAI 兼容 API**: `/v1/chat/completions`、`/v1/models`、`/v1/embeddings`
- **多模型适配**: OpenAI、Anthropic、Google、Azure、AWS、自定义渠道
- **智能路由**: 基于能力/成本/延迟/健康度加权评分，支持 fallback 链
- **统一鉴权**: API Key 管理、RBAC、租户隔离
- **配额与限流**: 租户级 RPM 限制、Redis 滑动窗口
- **结构化日志**: slog JSON 格式，完整可观测性

### 长期缓存体系 (P2)
- **L1 精确缓存**: Redis SHA256 匹配，毫秒级响应
- **L2 语义缓存**: Qdrant 向量数据库，余弦相似度匹配，带置信度阈值
- **L3 会话记忆**: PostgreSQL 会话摘要、用户偏好、最近消息
- **L4 资产复用层**: 知识资产 CRUD、版本控制、复用审计

### 跨模型复用 (P3)
- **Fallback Chain**: 主 Provider 故障时自动切换，支持多跳 fallback
- **标准化摘要资产**: 自动生成、版本管理、跨模型复用
- **审计与合规**: 完整请求/响应审计日志、数据导出、保留策略

### 企业化增强 (P4)
- **BYOK (Bring Your Own Key)**: 租户级 API Key 管理，加密存储
- **审计合规**: JSON/CSV 数据导出、自动清理、保留策略
- **控制面/数据面分离**: 配置热加载、版本发布、灰度推广、回滚
- **治理平台**: 模型推荐、审批工作流、策略版本、灰度发布、漂移检测

## 管理控制台

基于 React 18 + TypeScript 构建，21 个功能页面：

| 分组 | 页面 | 功能 |
|------|------|------|
| **概览** | Dashboard | 服务健康、Token 趋势、模型分布、缓存命中、渠道状态 |
| **管理** | Channels | 渠道 CRUD、批量操作、状态筛选、测试 |
| | Assets | 知识资产浏览、搜索/筛选/分页、删除 |
| | Config Center | 配置版本管理、继承/发布/回滚 |
| | Releases | 发布草稿、配置提升 |
| **监控** | Audit & Runtime | 审计事件、运行时事件、摘要统计 |
| | Audit Export | 数据导出 (JSON/CSV)、保留策略、清理 |
| | Observability | 请求量/Token/成本/延迟/缓存命中/错误率 |
| | Drift Dashboard | 模型漂移检测 |
| | Runtime Observer | 活跃策略、缓存状态、运行时决策、分发事件 |
| **策略** | Policies | 租户模型策略管理 |
| | Policy Versions | 策略版本、审批、激活、差异对比 |
| | Approvals | 审批队列、模型推荐审批 |
| | Rollouts | 灰度发布仪表盘、回滚 |
| **系统** | Quota | 租户配额管理、使用量趋势 |
| | Memory Governance | 候选事实/项目事实管理、确认/驳回/提升 |
| | Tenant Keys | BYOK 租户 API Key 管理 |
| | Playground | 在线 LLM 聊天测试 |
| | System | 系统状态、健康检查 |

## 快速开始

### 环境要求
- Go 1.22+
- Node.js 18+
- PostgreSQL 14+
- Redis 6+
- Qdrant (可选，L2 语义缓存)

### 1. 克隆仓库

```bash
git clone https://github.com/jimwong8/llm-gateway.git
cd llm-gateway
```

### 2. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env 文件
```

关键配置项：

```bash
# 基础配置
APP_ENV=development
APP_PORT=8080
ADMIN_API_KEY=ok0115ok

# 数据库
POSTGRES_DSN=postgres://llmadmin:CHANGE_ME@127.0.0.1:5432/llmgateway?sslmode=disable

# Redis
REDIS_ADDR=127.0.0.1:6379

# Provider (至少配置一个)
OPENAI_API_KEY=sk-...
OPENAI_BASE_URL=https://api.openai.com/v1

# 语义缓存 (可选)
QDRANT_URL=http://127.0.0.1:6333
QDRANT_API_KEY=CHANGE_ME
SEMANTIC_CACHE_ENABLED=true
SEMANTIC_CACHE_THRESHOLD=0.80

# 治理平台
MODEL_GOVERNANCE_ENABLED=true
AUDIT_LOG_ENABLED=true
BILLING_ENABLED=true
```

### 3. 启动后端

```bash
go build -o server cmd/server/main.go
./server
```

### 4. 构建前端

```bash
cd web/admin
npm install
npm run build
```

前端构建产物会自动嵌入到 Go 二进制中，通过 `/admin/ui` 路径访问。

### 5. 验证

```bash
# 健康检查
curl http://localhost:8080/healthz

# 管理 API (需要 Admin Key)
curl -H "Authorization: Bearer ok0115ok" http://localhost:8080/admin/health

# 访问管理控制台
open http://localhost:8080/admin/ui
```

### 6. 运行验证套件

```bash
# 顶层总 smoke (推荐每次改动后运行)
go run ./cmd/verify/smoke

# 增强型 smoke (含 runtime observer，需 PostgreSQL)
go run ./cmd/verify/smoke_plus

# 全部验证
go test ./...
```

## API 参考

### 生产端点

```bash
# 聊天补全 (OpenAI 兼容)
POST /v1/chat/completions
Authorization: Bearer <api-key>
Content-Type: application/json

{
  "model": "gpt-4",
  "messages": [{"role": "user", "content": "Hello"}]
}

# 模型列表
GET /v1/models
```

### 管理端点

所有管理端点需要 `Authorization: Bearer <admin-key>` 或 `X-Admin-Key: <admin-key>` 头。

```bash
# 渠道管理
GET    /admin/channels           # 列表
POST   /admin/channels           # 创建
GET    /admin/channels/{id}      # 详情
PUT    /admin/channels/{id}      # 更新
DELETE /admin/channels/{id}      # 删除
POST   /admin/channels/{id}/test # 测试连接
POST   /admin/channels/batch-delete
POST   /admin/channels/batch-status

# 资产管理
GET    /admin/assets             # 列表 (支持 keyword/task_type/source_model 筛选)
GET    /admin/assets/{id}        # 详情
DELETE /admin/assets/{id}        # 删除
GET    /admin/assets/stats       # 统计

# 租户密钥 (BYOK)
GET    /admin/tenant-keys        # 列表
POST   /admin/tenant-keys        # 创建/更新
DELETE /admin/tenant-keys/{tenant_id}/{provider}

# 审计与合规
GET    /admin/audit/export?tenant_id=xxx&format=json
POST   /admin/audit/cleanup?retention_days=90
GET    /admin/audit/retention

# 运行时观测
GET    /admin/governance/runtime-observer?environment=prod&limit=20

# 仪表盘
GET    /admin/dashboard
GET    /admin/dashboard/charts/token-usage
GET    /admin/dashboard/charts/model-distribution
GET    /admin/dashboard/charts/cache-hit-rate
GET    /admin/dashboard/charts/channel-status
```

## 项目结构

```
llm-gateway/
├── cmd/
│   ├── server/main.go          # 主入口
│   └── verify/                 # 验证工具集
│       ├── smoke/              # 顶层 smoke
│       ├── controlplane_runtime/
│       ├── chat_policy/
│       ├── model_governance/
│       └── ...
├── internal/
│   ├── httpserver/             # HTTP 服务器 + 所有 admin handlers
│   │   ├── server.go           # 主路由 + 内联 handlers
│   │   ├── admin_handler.go    # 控制面板管理
│   │   ├── model_governance_handler.go
│   │   ├── model_runtime_handler.go
│   │   ├── memory_admin_handler.go
│   │   └── adminui/            # 嵌入的前端构建产物
│   ├── router/                 # 路由引擎
│   ├── providers/              # Provider 适配器
│   ├── policy/                 # 策略引擎
│   ├── governance/             # 治理平台
│   ├── admin/                  # 管理后台存储
│   ├── audit/                  # 审计日志
│   ├── billing/                # 计费
│   ├── cache/                  # L1 缓存 (Redis)
│   ├── semantic/               # L2 语义缓存 (Qdrant)
│   ├── memory/                 # L3 会话记忆
│   ├── tenant/                 # 租户密钥管理
│   ├── quota/                  # 配额与限流
│   ├── controlplane/           # 控制面
│   ├── runtime/                # 运行时
│   └── db/migrations/          # 数据库迁移
├── web/admin/                  # 前端源码
│   ├── src/
│   │   ├── pages/              # 21 个页面组件
│   │   ├── components/         # 通用组件 (UI/Charts/Dashboard)
│   │   ├── lib/                # API 客户端 + 工具函数
│   │   ├── types/              # TypeScript 类型定义
│   │   └── styles/             # 纯 CSS 样式
│   └── package.json
├── docs/
│   └── plans/                  # 设计文档
├── README.md
└── RUNBOOK.md                  # 运维手册
```

## 技术选型

| 领域 | 技术 | 说明 |
|------|------|------|
| 后端 | Go 1.22+ | 高并发、流式转发、单二进制部署 |
| 前端 | React 18 + TypeScript | 类型安全、组件化 |
| 构建 | Vite | 快速 HMR、优化构建 |
| 状态管理 | TanStack Query | 服务器状态同步、缓存 |
| 路由 | React Router v6 | 声明式路由、嵌套布局 |
| 图表 | Recharts | 响应式、可定制 |
| 样式 | 纯 CSS (BEM) | 零依赖、轻量级 |
| 数据库 | PostgreSQL 14+ | 租户、配置、审计、记忆 |
| 缓存 | Redis 6+ | L1 精确缓存、限流、会话 |
| 向量数据库 | Qdrant | L2 语义缓存、资产检索 |
| 日志 | slog (JSON) | 结构化日志、可观测性 |

## 运维

详见 [RUNBOOK.md](./RUNBOOK.md)，包含：
- 日常验证命令
- 各模块专项验证
- Memory Governance 操作入口
- 故障排查指南

## License

MIT
