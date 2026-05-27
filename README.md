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

## 功能清单

### P1 — 网关内核
- **OpenAI 兼容 API**: `/v1/chat/completions`、`/v1/models`、`/v1/embeddings`
- **多模型适配**: OpenAI、Anthropic、Google、Azure、AWS、自定义渠道
- **智能路由**: 基于能力/成本/延迟/健康度加权评分，支持 fallback 链
- **统一鉴权**: API Key 管理、RBAC、租户隔离
- **配额与限流**: 租户级 RPM 限制、Redis 滑动窗口
- **结构化日志**: slog JSON 格式，完整可观测性
- **流式响应**: SSE 流式转发，支持 WebSocket 聊天

### P2 — 长期缓存体系
- **L1 精确缓存**: Redis SHA256 匹配，毫秒级响应
- **L2 语义缓存**: Qdrant 向量数据库，余弦相似度匹配，带置信度阈值
- **L3 会话记忆**: PostgreSQL 会话摘要、用户偏好、最近消息
- **L4 资产复用层**: 知识资产 CRUD、版本控制、复用审计

### P3 — 跨模型复用
- **Fallback Chain**: 主 Provider 故障时自动切换，支持多跳 fallback（最大深度 3）
- **标准化摘要资产**: 自动生成、版本管理、跨模型复用
- **审计与合规**: 完整请求/响应审计日志、数据导出、保留策略

### P4 — 企业化增强
- **BYOK (Bring Your Own Key)**: 租户级 API Key 管理，加密存储
- **审计合规**: JSON/CSV 数据导出、自动清理、保留策略
- **控制面/数据面分离**: 配置热加载、版本发布、灰度推广、回滚
- **治理平台**: 模型推荐、审批工作流、策略版本、灰度发布、漂移检测

### P5 — 用户体系
- **用户注册/登录**: 邮箱密码 + JWT 认证
- **OAuth 绑定**: GitHub OAuth 集成
- **用户 API Key**: 用户级别 API Key 管理与用量追踪
- **用量仪表盘**: 个人用量统计、成本趋势

### P6 — 计费系统
- **钱包管理**: 租户钱包、充值、余额查询
- **定价配置**: 按模型/Provider 差异化定价
- **订阅管理**: 套餐订阅、到期管理
- **邀请码**: 邀请注册、奖励机制

### P7 — 管理控制台
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

### P8 — CLI 管理工具
- **Prompt 预设管理**: 创建、更新、删除、列出提示词模板
- **掩码规则管理**: 敏感信息掩码规则的 CRUD
- **租户管理**: 列出租户、查看详情、配额管理
- **健康检查**: 简单/详细/管理员三种级别

## 快速开始

### 环境要求
- Go 1.22+
- Node.js 18+
- PostgreSQL 14+
- Redis 6+
- Qdrant (可选，L2 语义缓存)

### 方式一：Docker Compose 一键启动（推荐）

```bash
# 1. 克隆仓库
git clone https://github.com/jimwong8/llm-gateway.git
cd llm-gateway

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 文件，至少设置以下项：
#   POSTGRES_PASSWORD=your_strong_password
#   ADMIN_API_KEY=your_admin_key
#   OPENAI_API_KEY=sk-...

# 3. 一键启动所有服务
docker compose up -d

# 4. 验证
curl http://localhost:8080/healthz

# 5. 访问管理控制台
open http://localhost:8080/admin/ui
```

`docker-compose.yml` 包含三个服务：
- **postgres**: pgvector/pg16，自动执行迁移脚本
- **redis**: 7-alpine，AOF 持久化，256mb 内存限制
- **gateway**: 自动构建 Go 二进制 + 前端 SPA，端口 8080

### 方式二：手动启动

#### 1. 克隆仓库

```bash
git clone https://github.com/jimwong8/llm-gateway.git
cd llm-gateway
```

#### 2. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env 文件
```

#### 3. 启动依赖服务

```bash
# 使用 Docker 启动 PostgreSQL 和 Redis
docker compose up -d postgres redis
```

#### 4. 启动后端

```bash
go build -o server cmd/server/main.go
./server
```

#### 5. 构建前端

```bash
cd web/admin
npm install
npm run build
```

前端构建产物会自动嵌入到 Go 二进制中，通过 `/admin/ui` 路径访问。

#### 6. 验证

```bash
# 健康检查
curl http://localhost:8080/healthz

# 管理 API (需要 Admin Key)
curl -H "Authorization: Bearer ok0115ok" http://localhost:8080/admin/health

# 访问管理控制台
open http://localhost:8080/admin/ui
```

### 7. 运行验证套件

```bash
# 顶层总 smoke (推荐每次改动后运行)
go run ./cmd/verify/smoke

# 增强型 smoke (含 runtime observer，需 PostgreSQL)
go run ./cmd/verify/smoke_plus

# 全部验证
go test ./...
```

## API 文档

### OpenAPI 规范

完整的 API 规范可通过以下端点获取：

```
GET /api/openapi.json
```

在线查看：`http://localhost:8080/api/openapi.json`

详细 API 端点列表见 [docs/API.md](./docs/API.md)。

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

## CLI 使用

`cmd/cli` 提供命令行管理工具，支持 `table` 和 `json` 两种输出格式。

### 全局选项

```bash
llm-gateway-cli --api-base=http://localhost:8080 --token=<admin-key> --output=json <子命令>
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--api-base` | `http://localhost:8080` | API 服务器基础地址 |
| `--token` | (空) | 认证 Token (Bearer) |
| `--output` | `table` | 输出格式: `json` 或 `table` |

### Prompt 预设管理

```bash
# 列出所有预设
llm-gateway-cli preset list

# 创建预设
llm-gateway-cli preset create \
  --name "代码审查" \
  --template "请审查以下代码：\n{{code}}" \
  --variables "code" \
  --tags "review,code" \
  --public

# 更新预设
llm-gateway-cli preset update --id 1 --name "代码审查 v2"

# 删除预设
llm-gateway-cli preset delete --id 1
```

### 掩码规则管理

```bash
# 列出所有规则
llm-gateway-cli mask list

# 创建规则（屏蔽手机号）
llm-gateway-cli mask create \
  --name "手机号掩码" \
  --pattern "1[3-9]\\d{9}" \
  --replace "***"

# 删除规则
llm-gateway-cli mask delete --id 1
```

### 租户管理

```bash
# 列出所有租户
llm-gateway-cli tenant list

# 查看租户详情
llm-gateway-cli tenant show --tenant-id=tenant-001

# 查看配额
llm-gateway-cli tenant quota --tenant-id=tenant-001

# 设置配额
llm-gateway-cli tenant quota --tenant-id=tenant-001 --rpm-limit=120 --tpd-limit=1000000
```

### 健康检查

```bash
# 简单检查
llm-gateway-cli health

# 详细检查
llm-gateway-cli health --detailed

# 管理员检查
llm-gateway-cli health --admin
```

## 配置参考

所有配置通过环境变量设置，参考 `.env.example`：

### 基础配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `APP_ENV` | `production` | 运行环境: `development` / `production` |
| `APP_NAME` | `llm-gateway` | 服务名称 |
| `APP_PORT` | `8080` | 监听端口 |
| `MOCK_MODE` | `false` | Mock 模式（开发/测试用） |

### 数据库

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `POSTGRES_DSN` | — | PostgreSQL 连接串（必填） |
| `POSTGRES_PASSWORD` | — | PostgreSQL 密码（必填） |

### Redis

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REDIS_ADDR` | `redis:6379` | Redis 地址 |
| `REDIS_PASSWORD` | (空) | Redis 密码 |
| `REDIS_DB` | `0` | Redis 数据库编号 |
| `L1_CACHE_TTL_SECONDS` | `600` | L1 缓存 TTL（秒） |

### 语义缓存 (Qdrant)

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SEMANTIC_CACHE_ENABLED` | `true` | 是否启用 L2 语义缓存 |
| `SEMANTIC_CACHE_THRESHOLD` | `0.80` | 余弦相似度阈值 |
| `SEMANTIC_VECTOR_SIZE` | `64` | 向量维度 |
| `QDRANT_URL` | — | Qdrant 服务地址 |
| `QDRANT_API_KEY` | — | Qdrant API Key |
| `QDRANT_COLLECTION` | `semantic_cache_v1` | 集合名称 |

### 记忆

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MEMORY_ENABLED` | `true` | 是否启用 L3 会话记忆 |
| `MEMORY_MAX_ITEMS` | `3` | 最大记忆条数 |

### Provider 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DEFAULT_PROVIDER` | `openai` | 默认 Provider |
| `DEFAULT_MODEL` | `gpt-4o-mini` | 默认模型 |
| `OPENAI_API_KEY` | — | OpenAI API Key |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI 基础 URL |
| `OPENAI_TIMEOUT_SEC` | `120` | OpenAI 超时（秒） |
| `ANTHROPIC_API_KEY` | — | Anthropic API Key |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com/v1` | Anthropic 基础 URL |
| `XSTX_API_KEY` | — | XSTX API Key |
| `XSTX_BASE_URL` | `https://api.xstx.info/v1` | XSTX 基础 URL |

### 审计与计费

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `AUDIT_LOG_ENABLED` | `true` | 是否启用审计日志 |
| `AUDIT_RETENTION_DAYS` | `90` | 审计日志保留天数 |
| `BILLING_ENABLED` | `true` | 是否启用计费 |

### 配额

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TENANT_RPM` | `60` | 租户默认 RPM 限制 |
| `DEFAULT_API_KEY_RPM` | `60` | API Key 默认 RPM 限制 |

### 安全与认证

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ADMIN_API_KEY` | — | 管理 API 密钥（必填，生产环境务必修改） |
| `JWT_SECRET` | — | JWT 密钥（至少 32 字符） |

### 熔断与回退

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PROVIDER_MAX_RETRIES` | `1` | Provider 最大重试次数 |
| `PROVIDER_FAILURE_THRESHOLD` | `2` | 熔断触发失败次数 |
| `PROVIDER_OPEN_SECONDS` | `30` | 熔断器断开时间（秒） |
| `PROVIDER_HEALTH_TIMEOUT_SEC` | `5` | 健康检查超时（秒） |
| `FALLBACK_MAX_DEPTH` | `3` | Fallback 链最大深度 |
| `FALLBACK_MIN_SCORE_RATIO` | `0.5` | Fallback 最低评分比例 |

### 模型治理

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MODEL_GOVERNANCE_ENABLED` | `true` | 是否启用模型治理 |
| `MODEL_GOVERNANCE_CACHE_TTL_SECONDS` | `60` | 治理缓存 TTL |
| `MODEL_GOVERNANCE_ROLLOUT_MAX_ERROR_RATE` | `0.02` | 灰度发布最大错误率 |
| `MODEL_GOVERNANCE_ROLLOUT_MAX_P95_MS` | `1200` | 灰度发布最大 P95 延迟 |
| `MODEL_GOVERNANCE_ROLLOUT_MAX_FALLBACK_RATE` | `0.15` | 灰度发布最大 Fallback 率 |
| `MODEL_GOVERNANCE_MIN_SAMPLE_COUNT` | `200` | 最小样本数 |

### 可选集成

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `GITHUB_CLIENT_ID` | (空) | GitHub OAuth Client ID |
| `GITHUB_CLIENT_SECRET` | (空) | GitHub OAuth Client Secret |
| `SMTP_HOST` | (空) | SMTP 服务器地址 |
| `SMTP_PORT` | `587` | SMTP 端口 |
| `SMTP_USER` | (空) | SMTP 用户名 |
| `SMTP_PASSWORD` | (空) | SMTP 密码 |
| `SMTP_FROM` | (空) | 发件人地址 |

## 开发指南

### 本地开发

#### 1. 启动依赖服务

```bash
docker compose up -d postgres redis
```

#### 2. 配置开发环境

```bash
cp .env.example .env
# 设置 MOCK_MODE=true 可在无真实 Provider 的情况下开发
```

#### 3. 热重载开发

```bash
# 后端：直接运行（Go 不支持原生热重载，需借助 air 等工具）
go run cmd/server/main.go

# 前端：Vite HMR
cd web/admin
npm install
npm run dev
```

#### 4. 构建前端并嵌入

```bash
cd web/admin
npm run build
# 产物在 dist/ 目录，Go 编译时自动嵌入到 adminui/
```

### 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./internal/httpserver/... -v

# 运行特定测试
go test ./internal/httpserver/... -run TestChatCompletions -v

# 带覆盖率
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 验证工具集

项目提供多个验证工具，位于 `cmd/verify/`：

| 工具 | 说明 |
|------|------|
| `smoke` | 顶层 smoke，覆盖核心链路 |
| `smoke_plus` | 增强型 smoke，含 runtime observer |
| `routing` | 路由引擎验证 |
| `chat_policy` | 聊天策略验证 |
| `model_governance` | 模型治理验证 |
| `controlplane_runtime` | 控制面运行时验证 |
| `policy_engine` | 策略引擎验证 |
| `observability` | 可观测性验证 |
| `quota_observability` | 配额可观测性验证 |
| `compensation` | 补偿机制验证 |
| `inheritance_promotion` | 继承与晋升验证 |
| `promotion` | 晋升流程验证 |
| `project_scope` | 项目作用域验证 |
| `runtime_bus` | 运行时总线验证 |
| `runtime_observer` | 运行时观测器验证 |
| `cache_purge` | 缓存清理工具 |
| `cache_rebuild` | 缓存重建工具 |
| `reconcile` | 对账工具 |

运行方式：

```bash
go run ./cmd/verify/<tool_name>
```

### 代码结构

```
llm-gateway/
├── cmd/
│   ├── server/main.go          # 主入口
│   ├── cli/                    # CLI 管理工具
│   │   ├── main.go             # CLI 入口
│   │   ├── preset.go           # Prompt 预设管理
│   │   ├── mask.go             # 掩码规则管理
│   │   ├── tenant.go           # 租户管理
│   │   ├── health.go           # 健康检查
│   │   ├── client.go           # HTTP 客户端
│   │   └── output.go           # 输出格式化
│   └── verify/                 # 验证工具集
├── internal/
│   ├── httpserver/             # HTTP 服务器 + 所有 admin handlers
│   │   ├── server.go           # 主路由 + 内联 handlers
│   │   ├── admin_handler.go    # 控制面板管理
│   │   ├── model_governance_handler.go
│   │   ├── model_runtime_handler.go
│   │   ├── memory_admin_handler.go
│   │   ├── openapi.go          # OpenAPI 规范生成
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
│   ├── auth/                   # 认证 (API Key / JWT / OAuth)
│   ├── config/                 # 配置加载
│   ├── configstore/            # 配置版本存储
│   ├── chat/                   # 聊天会话存储
│   ├── email/                  # 邮件服务
│   ├── webhook/                # Webhook 注册
│   ├── broadcast/              # 广播消息
│   ├── abtest/                 # A/B 测试
│   ├── i18n/                   # 国际化
│   ├── health/                 # 健康检查
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
│   ├── plans/                  # 设计文档
│   ├── audit/                  # 审计报告
│   └── testing/                # 测试报告
├── Dockerfile                  # 多阶段构建
├── docker-compose.yml          # 开发/测试环境
├── docker-compose.prod.yml     # 生产环境
├── README.md
└── RUNBOOK.md                  # 运维手册
```

### 技术选型

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

## 部署指南

### Docker 部署（推荐）

#### 开发/测试环境

```bash
docker compose up -d
```

#### 生产环境

使用 `docker-compose.prod.yml`，包含 Nginx 反向代理和资源限制：

```bash
# 1. 确保 .env 已配置生产环境变量
# 2. 准备 Nginx 配置（nginx/nginx.conf, nginx/conf.d/）
# 3. 启动
docker compose -f docker-compose.prod.yml --env-file .env up -d
```

生产环境架构：
- **nginx**: 反向代理，暴露 80/443 端口，支持 Let's Encrypt SSL
- **gateway**: 仅内部网络访问，不直接暴露端口
- **postgres**: 仅内部网络访问，资源限制 1G 内存 / 2 CPU
- **redis**: 仅内部网络访问，资源限制 512M 内存 / 1 CPU
- **backend 网络**: 内部网络，无外部访问
- **frontend 网络**: nginx ↔ gateway 通信

### Kubernetes 部署

```bash
# 1. 创建 ConfigMap
kubectl create configmap llm-gateway-config --from-env-file=.env

# 2. 部署（需自行编写 K8s manifests）
kubectl apply -f k8s/
```

建议的 K8s 资源清单：
- `Deployment`: llm-gateway（replicas: 2+）
- `Service`: ClusterIP 类型
- `Ingress`: 配置 TLS 证书
- `ConfigMap`: 环境变量
- `Secret`: API Key、密码等敏感信息
- `PVC`: PostgreSQL 和 Redis 持久化存储

### 生产环境检查清单

- [ ] 修改 `ADMIN_API_KEY` 为强密码
- [ ] 修改 `JWT_SECRET` 为至少 32 字符的随机字符串
- [ ] 修改 `POSTGRES_PASSWORD` 为强密码
- [ ] 配置 `REDIS_PASSWORD`
- [ ] 设置 `APP_ENV=production`
- [ ] 设置 `MOCK_MODE=false`
- [ ] 配置至少一个 Provider API Key
- [ ] 启用 SSL/TLS（通过 Nginx 或 Ingress）
- [ ] 配置日志轮转
- [ ] 设置数据库备份策略
- [ ] 配置监控告警（Prometheus + Grafana）
- [ ] 调整资源限制（CPU/内存）
- [ ] 配置 `AUDIT_RETENTION_DAYS` 保留策略

### 运维

详见 [RUNBOOK.md](./RUNBOOK.md)，包含：
- 日常验证命令
- 各模块专项验证
- Memory Governance 操作入口
- 故障排查指南

## License

MIT
