# LLM Gateway 架构说明

## 系统概述

LLM Gateway 是一个企业级 AI 大模型中转站，对外提供与 OpenAI 兼容的 API 网关能力，对内屏蔽不同模型供应商之间的接口差异、鉴权差异、限流差异和计费差异。系统采用 Go 后端 + React 前端的经典架构，支持单二进制部署。

## 整体架构

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

## 分层架构

### 1. 接入层 (HTTP Server)

**位置：** `internal/httpserver/`

接入层负责所有 HTTP 请求的路由、认证、限流和响应。核心文件：

| 文件 | 职责 |
|------|------|
| `server.go` | 主路由注册、中间件链、内联 handlers |
| `admin_handler.go` | 渠道、资产、租户密钥管理 |
| `model_governance_handler.go` | 模型治理平台 |
| `model_runtime_handler.go` | 运行时决策与观测 |
| `memory_admin_handler.go` | Memory Governance |
| `openapi.go` | OpenAPI 3.0 规范生成 |
| `chat_handler.go` | 聊天补全（生产 API） |
| `auth_handler.go` | 用户认证 |
| `billing_handler.go` | 计费 API |
| `quota_handler.go` | 配额管理 |
| `ws_handler.go` | WebSocket 聊天 |
| `email_handler.go` | 邮件验证 |
| `oauth_handler.go` | OAuth 认证 |
| `parser_handler.go` | 文件解析 |
| `preset_handler.go` | Prompt 预设 |
| `usage_handler.go` | 用量统计 |

**中间件链（按执行顺序）：**
```
panicRecoveryMiddleware → requestIDMiddleware → loggingMiddleware → i18n.Middleware → handler
```

**认证方式：**
- **管理端点** (`/admin/*`): Admin API Key（`X-Admin-Key` 或 `Authorization: Bearer`）
- **用户端点** (`/api/*`): JWT Token（`Authorization: Bearer`）
- **生产端点** (`/v1/*`): API Key + 可选 JWT
- **公开端点** (`/healthz`, `/api/openapi.json`): 无需认证

**RBAC 角色：**
| 角色 | 权限 |
|------|------|
| `admin` | 全部权限 |
| `operator` | 读写（除治理审批外） |
| `approver` | 治理审批相关 |
| `readonly` | 只读 |

### 2. 路由引擎 (Router Engine)

**位置：** `internal/router/`

路由引擎是系统的核心决策组件，负责将请求路由到最优的 Provider。

**评分维度：**
- **Capability**: 模型能力匹配度（是否支持请求的功能）
- **Cost**: 调用成本（按 token 定价）
- **Latency**: 历史延迟表现
- **Health**: 当前健康状态（熔断器状态）

**路由流程：**
```
请求 → 能力匹配 → 成本评分 → 延迟评分 → 健康度评分 → 加权排序 → 选择最优 Provider
                                                                    ↓
                                                            失败 → Fallback 链
```

**Fallback 链：**
- 最大深度：3（可配置 `FALLBACK_MAX_DEPTH`）
- 最低评分比例：0.5（可配置 `FALLBACK_MIN_SCORE_RATIO`）
- 支持跨模型复用

### 3. Provider 适配器层

**位置：** `internal/providers/`

Provider 适配器屏蔽不同模型供应商的接口差异。

**支持的 Provider：**
| Provider | 说明 |
|----------|------|
| OpenAI | GPT-4、GPT-4o、GPT-4o-mini 等 |
| Anthropic | Claude-3 系列 |
| Google | Gemini 系列 |
| Azure | Azure OpenAI 服务 |
| AWS | Amazon Bedrock |
| XSTX | 自定义渠道 |
| Custom | 任意 OpenAI 兼容接口 |

**熔断器配置：**
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `PROVIDER_MAX_RETRIES` | 1 | 最大重试次数 |
| `PROVIDER_FAILURE_THRESHOLD` | 2 | 熔断触发失败次数 |
| `PROVIDER_OPEN_SECONDS` | 30 | 熔断器断开时间 |
| `PROVIDER_HEALTH_TIMEOUT_SEC` | 5 | 健康检查超时 |

### 4. 策略引擎 (Policy Engine)

**位置：** `internal/policy/`

策略引擎管理租户级别的模型访问策略。

**功能：**
- 租户-模型白名单管理
- 敏感信息检测与掩码
- 请求/响应策略执行

### 5. 治理平台 (Model Governance Platform)

**位置：** `internal/governance/`

治理平台提供完整的模型生命周期管理。

**核心功能：**
- **模型推荐**: 基于成本/性能自动生成推荐
- **审批工作流**: 多级审批（创建 → 审批 → 激活）
- **策略版本**: 策略的版本管理与差异对比
- **灰度发布**: 按比例灰度，自动监控错误率/延迟/Fallback 率
- **漂移检测**: 检测模型性能漂移
- **回滚**: 一键回滚到任意版本

**灰度发布阈值：**
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `MODEL_GOVERNANCE_ROLLOUT_MAX_ERROR_RATE` | 0.02 | 最大错误率 2% |
| `MODEL_GOVERNANCE_ROLLOUT_MAX_P95_MS` | 1200 | 最大 P95 延迟 1200ms |
| `MODEL_GOVERNANCE_ROLLOUT_MAX_FALLBACK_RATE` | 0.15 | 最大 Fallback 率 15% |
| `MODEL_GOVERNANCE_MIN_SAMPLE_COUNT` | 200 | 最小样本数 |

### 6. 控制面 (Control Plane)

**位置：** `internal/controlplane/`

控制面实现配置的版本管理、继承、晋升和发布。

**核心概念：**
- **配置版本**: 每次配置变更生成新版本
- **环境继承**: dev → staging → prod 的继承链
- **晋升 (Promotion)**: 配置从低环境晋升到高环境
- **发布 (Release)**: 将配置推送到运行时
- **补偿 (Compensation)**: 失败操作的自动补偿

### 7. 运行时 (Runtime)

**位置：** `internal/runtime/`

运行时负责配置的热加载和事件分发。

**功能：**
- 配置热重载（无需重启服务）
- 运行时事件发布/订阅
- 模块状态追踪
- 决策记录

### 8. 四层缓存体系

#### L1 — 精确缓存 (Redis)

**位置：** `internal/cache/`

- **匹配方式**: SHA256 哈希精确匹配
- **TTL**: 600 秒（可配置）
- **用途**: 完全相同的请求直接返回缓存
- **响应时间**: < 1ms

#### L2 — 语义缓存 (Qdrant)

**位置：** `internal/semantic/`

- **匹配方式**: 向量余弦相似度
- **阈值**: 0.80（可配置）
- **向量维度**: 64（可配置）
- **用途**: 语义相似的请求复用缓存
- **集合名**: `semantic_cache_v1`

#### L3 — 会话记忆 (PostgreSQL)

**位置：** `internal/memory/`

- **存储**: 会话摘要、用户偏好、最近消息
- **最大条数**: 3（可配置 `MEMORY_MAX_ITEMS`）
- **用途**: 跨请求的上下文记忆

#### L4 — 资产复用层 (PostgreSQL)

**位置：** `internal/admin/`

- **存储**: 知识资产、版本历史、复用审计
- **功能**: 知识资产的 CRUD、版本控制、跨模型复用

### 9. 审计系统

**位置：** `internal/audit/`

- **记录内容**: 所有请求/响应的完整审计
- **导出格式**: JSON / CSV
- **保留策略**: 默认 90 天（可配置 `AUDIT_RETENTION_DAYS`）
- **自动清理**: 支持定时清理过期数据

### 10. 计费系统

**位置：** `internal/billing/`

- **钱包管理**: 租户钱包、余额、充值
- **定价**: 按模型/Provider 差异化定价
- **订阅**: 套餐订阅管理
- **用量统计**: Token 用量、成本计算
- **邀请码**: 邀请注册奖励

### 11. 配额与限流

**位置：** `internal/quota/`

- **限流算法**: Redis 滑动窗口
- **租户级 RPM**: 默认 60（可配置 `TENANT_RPM`）
- **API Key 级 RPM**: 默认 60（可配置 `DEFAULT_API_KEY_RPM`）
- **每日 Token 限制**: 支持 TPD 限制

### 12. 用户认证体系

**位置：** `internal/auth/`

- **邮箱密码**: 注册、登录、密码重置
- **JWT**: Token 认证（HS256）
- **API Key**: 用户级别 API Key 管理
- **OAuth**: GitHub OAuth 2.0
- **邮箱验证**: 注册验证邮件

### 13. 前端管理控制台

**位置：** `web/admin/`

**技术栈：**
- React 18 + TypeScript
- Vite（构建工具）
- TanStack Query（服务器状态管理）
- React Router v6（路由）
- Recharts（图表）
- 纯 CSS + BEM（样式）

**页面结构（21 个页面）：**

| 分组 | 页面 | 组件文件 |
|------|------|----------|
| 概览 | Dashboard | `pages/Dashboard.tsx` |
| 管理 | Channels | `pages/Channels.tsx` |
| | Assets | `pages/Assets.tsx` |
| | Config Center | `pages/ConfigCenter.tsx` |
| | Releases | `pages/Releases.tsx` |
| 监控 | Audit & Runtime | `pages/AuditRuntime.tsx` |
| | Audit Export | `pages/AuditExport.tsx` |
| | Observability | `pages/Observability.tsx` |
| | Drift Dashboard | `pages/DriftDashboard.tsx` |
| | Runtime Observer | `pages/RuntimeObserver.tsx` |
| 策略 | Policies | `pages/Policies.tsx` |
| | Policy Versions | `pages/PolicyVersions.tsx` |
| | Approvals | `pages/Approvals.tsx` |
| | Rollouts | `pages/Rollouts.tsx` |
| 系统 | Quota | `pages/Quota.tsx` |
| | Memory Governance | `pages/MemoryGovernance.tsx` |
| | Tenant Keys | `pages/TenantKeys.tsx` |
| | Playground | `pages/Playground.tsx` |
| | System | `pages/System.tsx` |

**构建产物嵌入：**
前端构建后，产物被复制到 `internal/httpserver/adminui/`，Go 编译时通过 `embed.FS` 嵌入到二进制中，通过 `/admin/ui` 路径提供静态文件服务。

## 数据流

### 聊天请求流程

```
Client Request
    │
    ▼
┌──────────────────┐
│  API Key 认证     │
│  + 限流检查       │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  L1 精确缓存查询  │──── 命中 → 直接返回
└────────┬─────────┘
         │ 未命中
         ▼
┌──────────────────┐
│  L2 语义缓存查询  │──── 命中 → 返回（带置信度）
└────────┬─────────┘
         │ 未命中
         ▼
┌──────────────────┐
│  路由引擎决策      │
│  (评分 + Fallback)│
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Provider 适配器   │
│  (协议转换)        │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  外部 LLM API     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  响应处理          │
│  + 缓存写入        │
│  + 审计记录        │
│  + 计费记录        │
│  + L3/L4 更新     │
└────────┬─────────┘
         │
         ▼
    Client Response
```

### 配置发布流程

```
管理员操作
    │
    ▼
┌──────────────────┐
│  控制面           │
│  (创建配置版本)    │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  审批流程          │
│  (如需要)          │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  发布              │
│  (推送到运行时)     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  运行时热加载       │
│  (事件分发)         │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  补偿机制           │
│  (失败自动补偿)     │
└──────────────────┘
```

## 数据库设计

**位置：** `internal/db/migrations/`

共 27 个迁移文件（004-027），核心表：

| 表名 | 说明 |
|------|------|
| `channels` | 渠道配置 |
| `users` | 用户信息 |
| `user_api_keys` | 用户 API Key |
| `api_key_usage` | API Key 用量 |
| `billing_wallets` | 计费钱包 |
| `chat_sessions` | 聊天会话 |
| `chat_messages` | 聊天消息 |
| `site_config` | 站点配置 |
| `config_versions` | 配置版本 |
| `oauth_bindings` | OAuth 绑定 |
| `broadcasts` | 广播消息 |
| `subscriptions` | 订阅 |
| `memory_pyramid` | 记忆金字塔 |
| `usage_logs` | 用量日志 |
| `model_governance_*` | 治理平台（推荐、审批、版本、评估、漂移、回滚等） |

## 部署架构

### Docker Compose（开发/测试）

```
docker-compose.yml
├── postgres (pgvector/pg16)
├── redis (7-alpine)
└── gateway (Go binary + SPA)
```

### Docker Compose（生产）

```
docker-compose.prod.yml
├── postgres (内部网络, 资源限制)
├── redis (内部网络, 密码保护, 资源限制)
├── gateway (内部网络, 资源限制)
└── nginx (反向代理, SSL, 暴露 80/443)
```

### 网络隔离

- **backend 网络**: 内部网络，postgres/redis/gateway 之间通信，无外部访问
- **frontend 网络**: nginx ↔ gateway 通信

## 安全设计

1. **认证**: 多层认证（Admin Key / JWT / API Key）
2. **RBAC**: 基于角色的访问控制（admin/operator/approver/readonly）
3. **加密**: API Key 加密存储，密码 bcrypt 哈希
4. **熔断**: Provider 级熔断器，防止级联故障
5. **限流**: Redis 滑动窗口，租户级 + API Key 级双重限流
6. **审计**: 完整操作审计，支持导出和清理
7. **非 root 运行**: Docker 容器使用非 root 用户（uid 1001）
8. **最小镜像**: Alpine 基础镜像，多阶段构建，静态链接

## 可观测性

1. **结构化日志**: slog JSON 格式
2. **健康检查**: `/healthz`（简单）、`/healthz/detailed`（详细）、`/admin/health`（管理员）
3. **可观测性 API**: 请求量、Token、成本、延迟、缓存命中率、错误率
4. **运行时观测**: 活跃策略、缓存状态、运行时决策、分发事件
5. **Docker 健康检查**: 每 30 秒自动检查

## 扩展性

- **水平扩展**: 无状态设计，支持多实例部署
- **Provider 扩展**: 新增 Provider 只需实现适配器接口
- **缓存扩展**: L1/L2 缓存可独立扩展
- **配置热加载**: 无需重启即可更新配置
