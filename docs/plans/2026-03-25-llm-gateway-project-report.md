# LLM Gateway 项目说明报告

- 日期：2026-03-25
- 项目路径：[`/home/jimwong/zhongzhuan`](docs/plans/2026-03-25-llm-gateway-project-report.md)
- Go 模块：[`llm-gateway/gateway`](go.mod:1)

## 1. 项目概述

### 1.1 项目名称、定位、核心价值

该项目是一个企业级 Go 语言实现的 LLM Gateway，目标是提供统一的 OpenAI 兼容接入层，并在此基础上叠加长期缓存、路由治理、控制面发布、审计与可观测能力。

根据设计文档 [`docs/plans/2026-03-09-llm-gateway-design.md`](docs/plans/2026-03-09-llm-gateway-design.md)，项目定位包括：

- 作为企业/团队面向多模型供应商的统一入口。
- 屏蔽不同 Provider 的接口、鉴权、错误语义与计费差异。
- 将长期缓存作为核心卖点，沉淀可跨请求、跨会话、跨模型复用的知识资产。
- 采用控制面 / 数据面分离思路，适配 SaaS、专有实例与私有化部署。

核心价值可概括为：

1. **统一接入**：通过 [`cmd/server/main.go`](cmd/server/main.go) 启动网关，对外暴露 OpenAI 风格接口，如 [`/v1/chat/completions`](internal/httpserver/server.go:51) 与 [`/v1/models`](internal/httpserver/server.go:50)。
2. **治理能力**：通过 [`internal/controlplane/service.go`](internal/controlplane/service.go) 实现配置版本、发布与推广生命周期，通过 [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go) 暴露控制面管理 API。
3. **路由与容灾**：通过 [`router.Router.Decide()`](internal/router/router.go:67) 执行自动、混合与手动路由决策，并支持 fallback。
4. **缓存降本**：通过 [`cache.L1Cache`](internal/cache/l1.go:16) 与语义缓存接口形成多级缓存基础。
5. **可审计与可观测**：结合 [`audit.Recorder`](internal/audit/recorder.go:23)、运行时发布器 [`runtime.Publisher`](internal/runtime/publisher.go:9) 以及 billing/quota/admin 接口提供观测数据。

### 1.2 技术栈

#### Go 版本

- Go 版本：[`go 1.22`](go.mod:3)

#### 主要依赖

[`go.mod`](go.mod) 中直接依赖如下：

- [`github.com/lib/pq`](go.mod:6)：PostgreSQL 驱动，用于审计、计费、管理、策略、记忆等持久化存储。
- [`github.com/redis/go-redis/v9`](go.mod:7)：Redis 客户端，用于 L1 缓存与配额限流等能力。

间接依赖包括：

- [`github.com/cespare/xxhash/v2`](go.mod:11)
- [`github.com/dgryski/go-rendezvous`](go.mod:12)
- [`go.uber.org/atomic`](go.mod:13)

#### 主要内部技术组件

从 [`cmd/server/main.go`](cmd/server/main.go) 与各内部包可以看出技术栈包含：

- 标准库 HTTP 服务：[`http.ListenAndServe()`](cmd/server/main.go:90)
- 配置加载：[`config.Load()`](internal/config/config.go:42)
- Provider 抽象：[`providers.Provider`](internal/providers/provider.go:42)
- 路由引擎：[`router.Router`](internal/router/router.go:39)
- 精确缓存：[`cache.L1Cache`](internal/cache/l1.go:16)
- 语义缓存：[`semantic.L2Cache`](cmd/server/main.go:68)
- 记忆存储：[`memory.Store`](cmd/server/main.go:79)
- 审计：[`audit.Store`](cmd/server/main.go:43)
- 计费：[`billing.Store`](cmd/server/main.go:51)
- 配额：[`quota.Limiter`](cmd/server/main.go:41)
- 策略控制：[`policy.Store`](cmd/server/main.go:63)
- 管理数据层：[`admin.Store`](cmd/server/main.go:59)

### 1.3 项目目录结构说明

根据当前目录结构，核心代码组织如下：

- [`cmd/server`](cmd/server)：主服务入口。
- [`cmd/verify`](cmd/verify)：验证脚本目录，包含 7 个已说明的验证子命令类别与若干专项验证目录。
- [`docs/plans`](docs/plans)：设计文档与计划/报告文档。
- [`internal/admin`](internal/admin)：Admin 数据访问与资产管理。
- [`internal/audit`](internal/audit)：审计事件记录与存储。
- [`internal/billing`](internal/billing)：计费、使用量与观测汇总。
- [`internal/cache`](internal/cache)：L1 精确缓存。
- [`internal/config`](internal/config)：环境配置加载。
- [`internal/controlplane`](internal/controlplane)：控制面配置版本生命周期服务。
- [`internal/health`](internal/health)：健康检查辅助能力。
- [`internal/httpserver`](internal/httpserver)：HTTP Server、网关接口与 Admin API。
- [`internal/memory`](internal/memory)：长期记忆存储。
- [`internal/policy`](internal/policy)：模型许可、RBAC 与策略检查。
- [`internal/providers`](internal/providers)：Provider 协议抽象与具体 Provider 注册。
- [`internal/quota`](internal/quota)：租户配额与趋势观测。
- [`internal/router`](internal/router)：路由引擎与策略。
- [`internal/runtime`](internal/runtime)：运行时发布事件。
- [`internal/semantic`](internal/semantic)：语义缓存接口与实现。

## 2. 架构说明

### 2.1 控制面（Control Plane）

控制面能力主要由 [`controlplane.Service`](internal/controlplane/service.go:59) 与 [`AdminHandler`](internal/httpserver/admin_handler.go:24) 共同构成。

#### 2.1.1 配置版本管理

控制面围绕 [`ConfigVersion`](internal/runtime/publisher.go:6) 所引用的配置版本实体运行，核心状态常量定义在 [`internal/controlplane/service.go`](internal/controlplane/service.go) 中：

- [`ConfigStatusDraft`](internal/controlplane/service.go:14)
- [`ConfigStatusReleased`](internal/controlplane/service.go:15)
- [`SourceTypeInheritance`](internal/controlplane/service.go:17)

服务核心职责：

- 通过 [`Service.CreateVersion()`](internal/controlplane/service.go:81) 创建配置版本。
- 通过 [`Service.ListVersions()`](internal/controlplane/service.go:212) 与 [`Service.GetVersion()`](internal/controlplane/service.go:196) 查询版本。
- 通过 [`Service.CurrentReleased()`](internal/controlplane/service.go:188) 查找某租户/环境当前生效版本。
- 通过 [`Service.ResolveConfig()`](internal/controlplane/service.go:238) 将 tier default、tenant template、tenant override、project override 合并为最终配置。

#### 2.1.2 draft / release / promote 生命周期

控制面生命周期非常清晰：

1. **创建草稿**：[`Service.CreateVersion()`](internal/controlplane/service.go:81) 或 [`Service.CreateInheritanceDraft()`](internal/controlplane/service.go:109)
2. **发布草稿**：[`Service.ReleaseDraft()`](internal/controlplane/service.go:135)
3. **跨环境推广**：[`Service.PromoteReleased()`](internal/controlplane/service.go:162)
4. **查找当前发布版**：[`Service.findCurrentReleased()`](internal/controlplane/service.go:251)

其中继承草稿支持从源环境复制当前 released 版本，形成 target environment 的 draft，保留：

- 源环境 [`SourceEnvironment`](internal/controlplane/service.go:124)
- 源版本 [`SourceVersion`](internal/controlplane/service.go:125)

发布与推广都会触发 [`Service.recordReleaseSideEffects()`](internal/controlplane/service.go:274)，串联：

- 审计记录 [`auditor.RecordRelease()`](internal/controlplane/service.go:276)
- 运行时发布 [`publisher.PublishIfReleased()`](internal/controlplane/service.go:279)

这说明控制面不仅管理配置版本，也负责将配置变更传播到观测与运行时层。

### 2.2 数据面（Data Plane）

数据面主要由 [`httpserver.Server`](internal/httpserver/server.go:29) 承载。

#### 2.2.1 请求入口与路由

[`Server.Handler()`](internal/httpserver/server.go:47) 注册了：

- 健康检查 [`/healthz`](internal/httpserver/server.go:49)
- 模型列表 [`/v1/models`](internal/httpserver/server.go:50)
- 聊天补全 [`/v1/chat/completions`](internal/httpserver/server.go:51)

服务对象持有数据面运行所需的关键依赖：

- Provider 注册表 [`providers.Registry`](internal/httpserver/server.go:31)
- L1 缓存 [`cache.L1Cache`](internal/httpserver/server.go:32)
- 路由引擎 [`router.Router`](internal/httpserver/server.go:33)
- 审计、语义缓存、记忆、billing、quota、admin、policy 等治理组件

#### 2.2.2 Provider 适配

Provider 协议由 [`providers.Provider`](internal/providers/provider.go:42) 定义，要求实现：

- [`Provider.ChatCompletion()`](internal/providers/provider.go:43)
- [`Provider.Name()`](internal/providers/provider.go:44)

请求与响应统一抽象为：

- [`ChatCompletionRequest`](internal/providers/provider.go:10)
- [`ChatCompletionResponse`](internal/providers/provider.go:22)

在服务启动时，[`main()`](cmd/server/main.go:24) 初始化了：

- OpenAI Provider [`providers.NewOpenAIProvider()`](cmd/server/main.go:27)
- 默认 Mock Provider [`providers.NewMockProvider()`](cmd/server/main.go:28)
- code / analysis / fail 三类 mock provider，用于路由测试与故障注入 [`cmd/server/main.go`](cmd/server/main.go:29)

这表明数据面对 Provider 的设计目标是协议统一、实现可插拔、支持测试替身和故障演练。

#### 2.2.3 缓存层级

项目当前明确实现了 L1，并为 L2/L3 预留了接入点：

- L1：[`cache.L1Cache`](internal/cache/l1.go:16)
- L2：[`semantic.L2Cache`](internal/httpserver/server.go:35)
- L3：[`memory.Store`](internal/httpserver/server.go:36)

L1 的关键点：

- 使用 [`NormalizedRequest`](internal/cache/l1.go:23) 进行请求规范化。
- 通过 [`buildL1Key()`](internal/cache/l1.go:74) 对 model、tenant_id、messages 做归一化后计算 SHA-256 key。
- [`MemoryL1Cache.Get()`](internal/cache/l1.go:45) 与 [`MemoryL1Cache.Set()`](internal/cache/l1.go:59) 提供内存缓存兜底实现。

设计文档中提出的 L1/L2/L3/L4 长期缓存体系，在当前实现中已至少落地为：

- 精确缓存
- 语义缓存接口初始化
- 记忆存储初始化

### 2.3 Admin API 层

Admin API 由两部分组成：

1. 数据面服务内的管理接口，定义于 [`Server.Handler()`](internal/httpserver/server.go:47)
2. 控制面专用 Admin Handler，定义于 [`AdminHandler.registerRoutes()`](internal/httpserver/admin_handler.go:112)

#### 2.3.1 鉴权机制

数据面 Admin API 使用 [`Server.requireAdmin()`](internal/httpserver/server.go:74) 做鉴权：

- 优先读取 `X-Admin-Key`
- 其次兼容 `Authorization: Bearer <token>`
- token 必须等于 [`cfg.AdminAPIKey`](internal/httpserver/server.go:83)

此外还支持策略层 RBAC：

- 从 query 中读取 `tenant_id` [`internal/httpserver/server.go:88`](internal/httpserver/server.go:88)
- 从 Bearer 或 `X-Subject` 读取 subject [`currentSubject()`](internal/httpserver/server.go:105)
- 通过 [`policy.RoleFor()`](internal/httpserver/server.go:92) 获取角色
- 通过 [`roleAllowsMethod()`](internal/httpserver/server.go:113) 实现 `admin/operator/readonly` 的方法级访问控制

控制面 Admin Handler 使用 [`AdminHandler.isAuthorizedAdminRequest()`](internal/httpserver/admin_handler.go:130) 做鉴权，要求：

- 配置 [`adminToken`](internal/httpserver/admin_handler.go:29)
- 使用 `Authorization: Bearer <token>`
- token 与管理 token 完全匹配

### 2.4 可观测性

#### 2.4.1 审计事件

控制面审计结构为 [`audit.ControlPlaneEvent`](internal/audit/recorder.go:10)，记录字段包括：

- 事件类型
- 模块、租户、环境
- 源环境 / 源版本
- 版本号
- actor / reason / created_at

[`audit.Recorder`](internal/audit/recorder.go:23) 提供：

- [`RecordRelease()`](internal/audit/recorder.go:32)
- [`RecordInheritanceDraft()`](internal/audit/recorder.go:47)
- [`Events()`](internal/audit/recorder.go:64)

#### 2.4.2 运行时事件

运行时事件由 [`runtime.Event`](internal/runtime/publisher.go:5) 表示，内部携带 [`controlplane.ConfigVersion`](internal/runtime/publisher.go:6)。

[`runtime.Publisher`](internal/runtime/publisher.go:9) 提供：

- [`PublishIfReleased()`](internal/runtime/publisher.go:17)：仅发布 released 配置
- [`Events()`](internal/runtime/publisher.go:25)：读取发布事件列表

#### 2.4.3 Billing 指标

从 [`Server.Handler()`](internal/httpserver/server.go:47) 注册的接口可见，系统已将 billing / observability 指标固化为 Admin API：

- [`/admin/observability/summary`](internal/httpserver/server.go:55)
- [`/admin/observability/cache`](internal/httpserver/server.go:56)
- [`/admin/observability/providers`](internal/httpserver/server.go:57)
- [`/admin/observability/hotspots`](internal/httpserver/server.go:58)
- [`/admin/observability/quota`](internal/httpserver/server.go:59)
- [`/admin/observability/quota/trends`](internal/httpserver/server.go:60)

这些接口分别调用：

- [`billing.Summary()`](internal/httpserver/server.go:252)
- [`billing.CacheBreakdown()`](internal/httpserver/server.go:271)
- [`billing.ProviderBreakdown()`](internal/httpserver/server.go:290)
- [`billing.Hotspots()`](internal/httpserver/server.go:309)
- [`quota.Summary()`](internal/httpserver/server.go:329)
- [`quota.Trends()`](internal/httpserver/server.go:355)

说明系统已具备请求量、缓存命中、Provider 分布、热点与配额趋势等观测面。

## 3. 核心模块详解

以下按 [`internal`](internal) 子包说明。

### 3.1 [`internal/admin`](internal/admin)

- **职责**：提供 Admin 数据访问能力，支撑 usage、audit、资产管理、版本回滚与复用审计等接口。
- **主要类型和接口**：从 [`Server`](internal/httpserver/server.go:29) 与相关 handler 调用可见，核心类型为 `Store`。
- **关键方法**：
  - [`RecentUsage()`](internal/httpserver/server.go:213)
  - [`RecentAudit()`](internal/httpserver/server.go:233)
  - [`ListAssets()`](internal/httpserver/server.go:444)
  - [`CreateAsset()`](internal/httpserver/server.go:462)
  - [`UpdateAsset()`](internal/httpserver/server.go:480)
  - [`DeleteAsset()`](internal/httpserver/server.go:495)
  - [`AssetStats()`](internal/httpserver/server.go:519)
  - [`RecentAssetReuse()`](internal/httpserver/server.go:548)
  - [`ListAssetVersions()`](internal/httpserver/server.go:579)
  - [`RollbackAsset()`](internal/httpserver/server.go:616)

### 3.2 [`internal/audit`](internal/audit)

- **职责**：记录控制面审计事件，并为网关侧异步审计写入提供存储支持。
- **主要类型和接口**：[`ControlPlaneEvent`](internal/audit/recorder.go:10)、[`Recorder`](internal/audit/recorder.go:23)、`Store`。
- **关键方法**：[`Recorder.RecordRelease()`](internal/audit/recorder.go:32)、[`Recorder.RecordInheritanceDraft()`](internal/audit/recorder.go:47)、[`Recorder.Events()`](internal/audit/recorder.go:64)、[`Store.Ping()`](internal/httpserver/server.go:160)

### 3.3 [`internal/billing`](internal/billing)

- **职责**：汇总请求使用量、缓存分布、Provider 分布、热点与计费统计。
- **主要类型和接口**：`Store`。
- **关键方法**：[`Summary()`](internal/httpserver/server.go:252)、[`CacheBreakdown()`](internal/httpserver/server.go:271)、[`ProviderBreakdown()`](internal/httpserver/server.go:290)、[`Hotspots()`](internal/httpserver/server.go:309)、[`Ping()`](internal/httpserver/server.go:184)

### 3.4 [`internal/cache`](internal/cache)

- **职责**：实现 L1 精确缓存与缓存 Key 构造。
- **主要类型和接口**：[`L1Cache`](internal/cache/l1.go:16)、[`NormalizedRequest`](internal/cache/l1.go:23)、[`MemoryL1Cache`](internal/cache/l1.go:30)
- **关键方法**：[`Get()`](internal/cache/l1.go:45)、[`Set()`](internal/cache/l1.go:59)、[`BuildKey()`](internal/cache/l1.go:70)、[`buildL1Key()`](internal/cache/l1.go:74)

### 3.5 [`internal/config`](internal/config)

- **职责**：从环境变量加载运行配置。
- **主要类型和接口**：[`Config`](internal/config/config.go:9)
- **关键方法**：[`Load()`](internal/config/config.go:42)、[`Addr()`](internal/config/config.go:78)

### 3.6 [`internal/controlplane`](internal/controlplane)

- **职责**：实现配置版本、继承草稿、发布、推广、配置合并与当前发布版本解析。
- **主要类型和接口**：[`ConfigSource`](internal/controlplane/service.go:23)、[`CreateVersionInput`](internal/controlplane/service.go:30)、[`CreateInheritanceDraftInput`](internal/controlplane/service.go:44)、[`Service`](internal/controlplane/service.go:59)
- **关键方法**：[`CreateVersion()`](internal/controlplane/service.go:81)、[`CreateInheritanceDraft()`](internal/controlplane/service.go:109)、[`ReleaseDraft()`](internal/controlplane/service.go:135)、[`PromoteReleased()`](internal/controlplane/service.go:162)、[`CurrentReleased()`](internal/controlplane/service.go:188)、[`GetVersion()`](internal/controlplane/service.go:196)、[`ListVersions()`](internal/controlplane/service.go:212)、[`ResolveConfig()`](internal/controlplane/service.go:238)

### 3.7 [`internal/health`](internal/health)

- **职责**：执行 Provider 健康检查并生成健康摘要。
- **主要类型和接口**：未直接展开源码，但由 [`health.CheckProviders()`](internal/httpserver/server.go:198) 使用。
- **关键方法**：[`CheckProviders()`](internal/httpserver/server.go:198)

### 3.8 [`internal/httpserver`](internal/httpserver)

- **职责**：承载网关 HTTP 接口、Admin API、鉴权、参数校验与 JSON 响应封装。
- **主要类型和接口**：[`Server`](internal/httpserver/server.go:29)、[`AdminHandler`](internal/httpserver/admin_handler.go:24)
- **关键方法**：[`Handler()`](internal/httpserver/server.go:47)、[`requireAdmin()`](internal/httpserver/server.go:74)、[`adminHealth()`](internal/httpserver/server.go:193)、[`adminUsage()`](internal/httpserver/server.go:201)、[`adminPoliciesModels()`](internal/httpserver/server.go:372)、[`adminAssets()`](internal/httpserver/server.go:414)、[`AdminHandler.registerRoutes()`](internal/httpserver/admin_handler.go:112)

### 3.9 [`internal/memory`](internal/memory)

- **职责**：提供长期记忆持久化与健康检查。
- **主要类型和接口**：`Store`。
- **关键方法**：[`Ping()`](internal/httpserver/server.go:174)

### 3.10 [`internal/policy`](internal/policy)

- **职责**：执行模型可用策略、敏感规则与 RBAC 管控。
- **主要类型和接口**：`Store`、`SensitiveRule`。
- **关键方法**：[`RoleFor()`](internal/httpserver/server.go:92)、[`AllowedModels()`](internal/httpserver/server.go:382)、[`Upsert()`](internal/httpserver/server.go:404)

### 3.11 [`internal/providers`](internal/providers)

- **职责**：定义统一 Provider 抽象，封装请求/响应协议。
- **主要类型和接口**：[`ChatMessage`](internal/providers/provider.go:5)、[`ChatCompletionRequest`](internal/providers/provider.go:10)、[`ChatCompletionResponse`](internal/providers/provider.go:22)、[`Provider`](internal/providers/provider.go:42)
- **关键方法**：[`Provider.ChatCompletion()`](internal/providers/provider.go:43)、[`Provider.Name()`](internal/providers/provider.go:44)

### 3.12 [`internal/quota`](internal/quota)

- **职责**：提供租户级限流与配额摘要、趋势观测。
- **主要类型和接口**：`Limiter`。
- **关键方法**：[`Summary()`](internal/httpserver/server.go:329)、[`Trends()`](internal/httpserver/server.go:355)

### 3.13 [`internal/router`](internal/router)

- **职责**：根据任务类型、候选模型、偏好模型与全局策略做模型路由决策。
- **主要类型和接口**：[`ModelProfile`](internal/router/router.go:10)、[`CandidateScore`](internal/router/router.go:21)、[`Decision`](internal/router/router.go:29)、[`Router`](internal/router/router.go:39)
- **关键方法**：[`New()`](internal/router/router.go:46)、[`Models()`](internal/router/router.go:60)、[`SetGlobalPolicy()`](internal/router/router.go:63)、[`Decide()`](internal/router/router.go:67)、[`classifyTask()`](internal/router/router.go:114)、[`scoreCandidates()`](internal/router/router.go:165)

### 3.14 [`internal/runtime`](internal/runtime)

- **职责**：将 released 配置版本发布为运行时事件，供动态配置重载或事件总线消费。
- **主要类型和接口**：[`Event`](internal/runtime/publisher.go:5)、[`Publisher`](internal/runtime/publisher.go:9)
- **关键方法**：[`PublishIfReleased()`](internal/runtime/publisher.go:17)、[`Events()`](internal/runtime/publisher.go:25)

### 3.15 [`internal/semantic`](internal/semantic)

- **职责**：提供语义缓存接口与实现，用于 L2 相似检索增强。
- **主要类型和接口**：[`semantic.L2Cache`](internal/httpserver/server.go:35)
- **关键方法**：[`EnsureCollection()`](cmd/server/main.go:73)、[`semantic.NewMemoryL2Cache()`](cmd/server/main.go:70)

## 4. API 接口清单

以下为从 [`internal/httpserver/server.go`](internal/httpserver/server.go) 和 [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go) 提取的全部 `/admin/*` 路由。

### 4.1 数据面 Admin API

| 路由 | 方法 | 说明 |
| --- | --- | --- |
| [`/admin/health`](internal/httpserver/server.go:52) | GET | 管理健康检查，返回 provider 健康与 admin_auth 状态 |
| [`/admin/usage`](internal/httpserver/server.go:53) | GET | 最近使用量列表 |
| [`/admin/audit`](internal/httpserver/server.go:54) | GET | 最近审计记录列表 |
| [`/admin/observability/summary`](internal/httpserver/server.go:55) | GET | 观测摘要 |
| [`/admin/observability/cache`](internal/httpserver/server.go:56) | GET | 缓存命中层级与分布 |
| [`/admin/observability/providers`](internal/httpserver/server.go:57) | GET | Provider 维度统计 |
| [`/admin/observability/hotspots`](internal/httpserver/server.go:58) | GET | 热点租户/模型等 |
| [`/admin/observability/quota`](internal/httpserver/server.go:59) | GET | 租户配额摘要 |
| [`/admin/observability/quota/trends`](internal/httpserver/server.go:60) | GET | 配额趋势 |
| [`/admin/policies/models`](internal/httpserver/server.go:61) | GET / POST | 查询或更新租户允许模型列表 |
| [`/admin/assets`](internal/httpserver/server.go:62) | GET / POST / PUT / DELETE | 资产列表、创建、更新、删除 |
| [`/admin/assets/stats`](internal/httpserver/server.go:63) | GET | 资产统计 |
| [`/admin/assets/reuse-audits`](internal/httpserver/server.go:64) | GET | 资产复用审计 |
| [`/admin/assets/versions`](internal/httpserver/server.go:65) | GET | 资产版本列表 |
| [`/admin/assets/rollback`](internal/httpserver/server.go:66) | POST | 资产版本回滚 |
| [`/admin/control-plane/compensations`](internal/httpserver/server.go:67) | GET | 控制面补偿任务占位接口 |
| [`/admin/ui`](internal/httpserver/server.go:68) | GET | Admin UI 入口 |
| [`/admin/ui/`](internal/httpserver/server.go:69) | GET | Admin UI 静态资源路径 |

### 4.2 控制面 Admin API

| 路由 | 方法 | 说明 |
| --- | --- | --- |
| [`/admin/inheritance-drafts`](internal/httpserver/admin_handler.go:113) | POST | 创建继承草稿 |
| [`/admin/releases`](internal/httpserver/admin_handler.go:114) | POST | 发布草稿版本 |
| [`/admin/promotions`](internal/httpserver/admin_handler.go:115) | POST | 跨环境推广已发布版本 |
| [`/admin/audit-events`](internal/httpserver/admin_handler.go:116) | GET | 查询控制面审计事件，支持 summary 模式 |
| [`/admin/runtime-events`](internal/httpserver/admin_handler.go:117) | GET | 查询运行时发布事件，支持 summary 模式 |
| [`/admin/config-versions`](internal/httpserver/admin_handler.go:118) | GET | 版本列表 |
| [`/admin/config-versions/`](internal/httpserver/admin_handler.go:119) | GET | 单版本详情 |

## 5. 配置与部署

### 5.1 环境变量配置项

配置由 [`config.Load()`](internal/config/config.go:42) 加载，字段定义于 [`Config`](internal/config/config.go:9)。主要环境变量如下：

#### 基础服务配置

- `APP_ENV`：默认 `development` [`internal/config/config.go:44`](internal/config/config.go:44)
- `APP_NAME`：默认 `llm-gateway` [`internal/config/config.go:45`](internal/config/config.go:45)
- `APP_PORT`：默认 `8080` [`internal/config/config.go:46`](internal/config/config.go:46)

#### 存储与缓存配置

- `POSTGRES_DSN` [`internal/config/config.go:47`](internal/config/config.go:47)
- `REDIS_ADDR` [`internal/config/config.go:48`](internal/config/config.go:48)
- `REDIS_PASSWORD` [`internal/config/config.go:49`](internal/config/config.go:49)
- `REDIS_DB` [`internal/config/config.go:50`](internal/config/config.go:50)
- `L1_CACHE_TTL_SECONDS` [`internal/config/config.go:51`](internal/config/config.go:51)

#### 语义缓存 / 向量相关

- `QDRANT_URL` [`internal/config/config.go:52`](internal/config/config.go:52)
- `QDRANT_API_KEY` [`internal/config/config.go:53`](internal/config/config.go:53)
- `QDRANT_COLLECTION` [`internal/config/config.go:54`](internal/config/config.go:54)
- `SEMANTIC_CACHE_ENABLED` [`internal/config/config.go:55`](internal/config/config.go:55)
- `SEMANTIC_CACHE_THRESHOLD` [`internal/config/config.go:56`](internal/config/config.go:56)
- `SEMANTIC_VECTOR_SIZE` [`internal/config/config.go:57`](internal/config/config.go:57)

#### 记忆与默认路由

- `MEMORY_ENABLED` [`internal/config/config.go:58`](internal/config/config.go:58)
- `MEMORY_MAX_ITEMS` [`internal/config/config.go:59`](internal/config/config.go:59)
- `DEFAULT_PROVIDER` [`internal/config/config.go:60`](internal/config/config.go:60)
- `DEFAULT_MODEL` [`internal/config/config.go:61`](internal/config/config.go:61)
- `MOCK_MODE` [`internal/config/config.go:62`](internal/config/config.go:62)

#### OpenAI Provider 配置

- `OPENAI_BASE_URL` [`internal/config/config.go:63`](internal/config/config.go:63)
- `OPENAI_API_KEY` [`internal/config/config.go:64`](internal/config/config.go:64)
- `OPENAI_TIMEOUT_SEC` [`internal/config/config.go:65`](internal/config/config.go:65)

#### 审计 / 计费 / 配额 / 管理配置

- `AUDIT_LOG_ENABLED` [`internal/config/config.go:66`](internal/config/config.go:66)
- `BILLING_ENABLED` [`internal/config/config.go:67`](internal/config/config.go:67)
- `TENANT_RPM` [`internal/config/config.go:68`](internal/config/config.go:68)
- `ADMIN_API_KEY` [`internal/config/config.go:69`](internal/config/config.go:69)

#### Provider 稳定性控制

- `PROVIDER_MAX_RETRIES` [`internal/config/config.go:70`](internal/config/config.go:70)
- `PROVIDER_FAILURE_THRESHOLD` [`internal/config/config.go:71`](internal/config/config.go:71)
- `PROVIDER_OPEN_SECONDS` [`internal/config/config.go:72`](internal/config/config.go:72)
- `PROVIDER_HEALTH_TIMEOUT_SEC` [`internal/config/config.go:73`](internal/config/config.go:73)

### 5.2 启动方式

服务启动入口为 [`cmd/server/main.go`](cmd/server/main.go)。启动流程如下：

1. 调用 [`config.Load()`](cmd/server/main.go:25) 读取环境变量。
2. 初始化 Provider、缓存、路由、配额、审计、billing、admin、policy、semantic、memory 组件。
3. 构造 [`httpserver.New()`](cmd/server/main.go:88) 创建服务。
4. 通过 [`http.ListenAndServe()`](cmd/server/main.go:90) 在 [`cfg.Addr()`](internal/config/config.go:78) 监听 HTTP 请求。

典型启动命令可写为：

```bash
go run ./cmd/server
```

也可先编译再运行：

```bash
go build ./cmd/server
./server
```

## 6. 测试覆盖

### 6.1 各模块测试状态

当前通过任务背景可确认：

- [`go build ./...`](docs/plans/2026-03-25-llm-gateway-project-report.md) ✅
- `go test -count=1 ./...` ✅
- 11 个 `internal` 包全部通过
- 7 个 `cmd/verify` 子命令运行正常

从当前代码扫描结果看，工作区中未检索到 `*_test.go` 下的 [`func Test`](internal/router/router.go:165) 定义，因此可以推断：

- 项目当前的验证方式除了标准 `go test` 外，还依赖 [`cmd/verify`](cmd/verify) 目录下的专项验证程序。
- 测试资产更偏向集成验证 / 场景验证，而不是大量分散的传统单元测试文件暴露在当前扫描范围中。

### 6.2 验证脚本说明

`cmd/verify` 下当前可见验证目录包括：

- [`cmd/verify/compensation`](cmd/verify/compensation)
- [`cmd/verify/inheritance_promotion`](cmd/verify/inheritance_promotion)
- [`cmd/verify/observability`](cmd/verify/observability)
- [`cmd/verify/policy_engine`](cmd/verify/policy_engine)
- [`cmd/verify/project_scope`](cmd/verify/project_scope)
- [`cmd/verify/promotion`](cmd/verify/promotion)
- [`cmd/verify/quota_observability`](cmd/verify/quota_observability)
- [`cmd/verify/routing`](cmd/verify/routing)
- [`cmd/verify/runtime_bus`](cmd/verify/runtime_bus)

结合目录命名，这些验证脚本覆盖了：

- 配置继承与推广生命周期
- 观测与计费链路
- 策略引擎
- 项目级作用域
- 配额观测
- 路由决策
- 运行时事件总线
- 控制面补偿流程

尽管用户说明中强调“7 个 `cmd/verify` 子命令运行正常”，当前目录中可见专项验证目录数量多于 7，说明仓库可能存在扩展验证场景，报告中仍以用户确认结论为准。

## 7. 当前完工状态

根据当前代码实现与用户提供的验收结论，项目可认定为已完成的 LLM Gateway 项目，当前完工状态如下：

- [`go build ./...`](docs/plans/2026-03-25-llm-gateway-project-report.md) ✅
- `go test -count=1 ./...` ✅
- 11 个 `internal` 包全部通过 ✅
- 7 个 `cmd/verify` 子命令运行正常 ✅

从实现角度，已具备以下交付能力：

1. **服务启动与运行**：[`main()`](cmd/server/main.go:24) 可装配完整运行栈并启动 HTTP 服务。
2. **OpenAI 兼容数据面**：[`/v1/chat/completions`](internal/httpserver/server.go:51) 与 [`/v1/models`](internal/httpserver/server.go:50) 已接入统一网关。
3. **控制面生命周期**：[`CreateInheritanceDraft()`](internal/controlplane/service.go:109)、[`ReleaseDraft()`](internal/controlplane/service.go:135)、[`PromoteReleased()`](internal/controlplane/service.go:162) 已实现。
4. **路由与缓存**：[`Router.Decide()`](internal/router/router.go:67) 与 [`buildL1Key()`](internal/cache/l1.go:74) 已构成核心链路。
5. **管理与治理**：Admin API、模型策略、资产版本、配额与观测接口均已具备。
6. **可观测性闭环**：审计事件、运行时事件、billing/quota 观测均已接入。

## 8. 总结

该项目已经从设计文档中的企业级 LLM Gateway 设想，落地为一个具备以下特征的可运行 Go 网关系统：

- 对外提供统一模型接入 API
- 对内具备 Provider 抽象与路由调度
- 具备 L1/L2/L3 长期缓存基础设施
- 具备控制面版本治理能力
- 具备 Admin API、RBAC、审计、运行时发布与 billing/quota 可观测性

若后续继续演进，建议优先沿着设计文档所描述的方向推进：

- 将语义缓存与长期记忆从基础设施接入升级为质量受控的生产链路
- 完善控制面 / 数据面解耦与运行时动态配置下发
- 强化跨模型复用资产层与多租户隔离治理
