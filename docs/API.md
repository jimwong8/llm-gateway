# LLM Gateway API 文档

> 完整的 API 规范可通过 `GET /api/openapi.json` 获取。

## 认证方式

### 管理端点
所有管理端点（`/admin/*`）需要以下认证头之一：
- `Authorization: Bearer <admin-key>`
- `X-Admin-Key: <admin-key>`

### 用户端点
用户端点（`/api/*`）需要 JWT 认证：
- `Authorization: Bearer <jwt-token>`

JWT Token 通过 `POST /api/auth/login` 获取。

---

## 健康检查

### GET /healthz

公开端点，无需认证。返回服务基本健康状态。

**响应示例：**
```json
{
  "status": "ok",
  "service": "llm-gateway",
  "env": "production",
  "mock_mode": false,
  "cache": "ok",
  "audit": "ok",
  "semantic_cache": "ok",
  "memory": "ok",
  "billing": "ok",
  "time": "2026-01-15T10:30:00Z"
}
```

### GET /healthz/detailed

详细健康检查，包含各子系统状态。

### GET /admin/health

管理员健康检查，包含 Provider 状态、运行时摘要、补偿统计。需要管理认证。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `detailed` | bool | 显示详细 Provider 信息 |

---

## 生产 API（OpenAI 兼容）

### POST /v1/chat/completions

聊天补全，与 OpenAI Chat Completions API 兼容。支持 SSE 流式响应。

**请求头：**
- `Authorization: Bearer <api-key>`
- `Content-Type: application/json`

**请求体：**
```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": false,
  "temperature": 0.7,
  "max_tokens": 2048
}
```

**响应：**
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1705312200,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 8,
    "total_tokens": 18
  }
}
```

### GET /v1/models

列出所有可用模型。

**响应：**
```json
{
  "object": "list",
  "data": [
    {"id": "gpt-4", "object": "model", "created": 1705312200, "owned_by": "openai"},
    {"id": "gpt-4o-mini", "object": "model", "created": 1705312200, "owned_by": "openai"}
  ]
}
```

---

## 管理 API — 渠道管理

### GET /admin/channels

列出所有渠道。

**响应：** 渠道列表（object=list）

### POST /admin/channels

创建新渠道。

**请求体：**
```json
{
  "name": "OpenAI 主渠道",
  "provider": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "priority": "primary",
  "weight": 100,
  "models": ["gpt-4", "gpt-4o-mini"],
  "tags": ["production"],
  "notes": "主生产渠道",
  "status": "active"
}
```

### GET /admin/channels/{id}

获取渠道详情。

### PUT /admin/channels/{id}

更新渠道。

### DELETE /admin/channels/{id}

删除渠道。

### POST /admin/channels/{id}/test

测试渠道连接。

### POST /admin/channels/batch-delete

批量删除渠道。

**请求体：**
```json
{
  "ids": ["channel-1", "channel-2"]
}
```

### POST /admin/channels/batch-status

批量更新渠道状态。

**请求体：**
```json
{
  "ids": ["channel-1", "channel-2"],
  "status": "disabled"
}
```

---

## 管理 API — 资产管理

### GET /admin/assets

列出知识资产。

**查询参数：**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `tenant_id` | string | — | 按租户筛选 |
| `task_type` | string | — | 按任务类型筛选 |
| `source_model` | string | — | 按源模型筛选 |
| `tag` | string | — | 按标签筛选 |
| `keyword` | string | — | 关键词搜索 |
| `limit` | int | 20 | 每页数量 |
| `offset` | int | 0 | 偏移量 |
| `include_deleted` | bool | false | 包含已删除 |

### POST /admin/assets

创建知识资产。

**请求体：**
```json
{
  "tenant_id": "tenant-001",
  "source_model": "gpt-4",
  "task_type": "summarization",
  "title": "文档摘要",
  "summary": "...",
  "tags": ["doc", "summary"]
}
```

### GET /admin/assets/{id}

获取资产详情。

### PUT /admin/assets/{id}

更新资产。

### DELETE /admin/assets/{id}

删除资产。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `tenant_id` | string | 租户 ID（必填） |

### GET /admin/assets/stats

获取资产统计。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `tenant_id` | string | 租户 ID |
| `include_deleted` | bool | 包含已删除 |
| `limit` | int | 返回数量 |

### GET /admin/assets/reuse-audits

获取资产复用审计记录。

### GET /admin/assets/versions

获取资产版本列表。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `asset_id` | int | 资产 ID（必填） |
| `tenant_id` | string | 租户 ID |
| `limit` | int | 每页数量 |
| `offset` | int | 偏移量 |

### POST /admin/assets/rollback

回滚资产到指定版本。

**请求体：**
```json
{
  "asset_id": 123,
  "version": 2,
  "tenant_id": "tenant-001"
}
```

---

## 管理 API — 租户密钥 (BYOK)

### GET /admin/tenant-keys

列出所有租户密钥。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `tenant_id` | string | 按租户筛选 |

### POST /admin/tenant-keys

创建/更新租户密钥。

**请求体：**
```json
{
  "tenant_id": "tenant-001",
  "provider": "openai",
  "api_key": "sk-..."
}
```

### DELETE /admin/tenant-keys/{tenant_id}/{provider}

删除租户密钥。

---

## 管理 API — 审计与合规

### GET /admin/audit

获取最近审计事件。

**查询参数：**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | 20 | 返回数量 |

### GET /admin/audit/export

导出审计数据。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `tenant_id` | string | 按租户筛选 |
| `format` | string | 导出格式: `json` 或 `csv` |

### POST /admin/audit/cleanup

清理过期审计数据。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `retention_days` | int | 保留天数 |

### GET /admin/audit/retention

获取审计保留策略配置。

---

## 管理 API — 仪表盘

### GET /admin/dashboard

获取仪表盘概览数据。

### GET /admin/dashboard/charts/token-usage

获取 Token 使用趋势。

**查询参数：**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `days` | int | 7 | 天数（1-30） |

### GET /admin/dashboard/charts/model-distribution

获取模型使用分布。

### GET /admin/dashboard/charts/cache-hit-rate

获取缓存命中率趋势。

**查询参数：**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `days` | int | 7 | 天数（1-30） |

### GET /admin/dashboard/charts/channel-status

获取渠道健康状态分布。

---

## 管理 API — 可观测性

### GET /admin/observability/summary

获取可观测性摘要（计费汇总）。

### GET /admin/observability/cache

获取缓存分解数据。

### GET /admin/observability/providers

获取 Provider 分解数据。

### GET /admin/observability/hotspots

获取热点分析。

### GET /admin/observability/latency

获取延迟趋势（P50/P95/P99）。

**查询参数：**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `days` | int | 7 | 天数（1-30） |

### GET /admin/observability/error-rate

获取错误率趋势。

**查询参数：**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `days` | int | 7 | 天数（1-30） |

### GET /admin/observability/quota

获取配额摘要。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `tenant_id` | string | 租户 ID |

### GET /admin/observability/quota/trends

获取配额使用趋势。

**查询参数：**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `tenant_id` | string | — | 租户 ID |
| `window_minutes` | int | 5 | 时间窗口（分钟） |

---

## 管理 API — 策略管理

### GET /admin/policies/models

获取租户允许的模型列表。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `tenant_id` | string | 租户 ID |

### POST /admin/policies/models

更新租户模型策略。

**请求体：**
```json
{
  "tenant_id": "tenant-001",
  "model": "gpt-4",
  "enabled": true
}
```

---

## 管理 API — 控制面

### GET /admin/inheritance-drafts

列出继承草稿。

### POST /admin/releases

创建发布。

### POST /admin/releases/rollback

回滚发布。

### POST /admin/releases/replay

重放发布事件。

### GET /admin/promotions

列出晋升记录。

### GET /admin/audit-events

列出审计事件。

### GET /admin/runtime-events

列出运行时事件。

### GET /admin/config-versions

列出配置版本。

### GET /admin/config-versions/{versionID}

获取配置版本详情。

### GET /admin/control-plane/compensations

列出补偿记录。

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `tenant_id` | string | 按租户筛选 |
| `environment` | string | 按环境筛选 |
| `failed_stage` | string | 按失败阶段筛选 |
| `module` | string | 按模块筛选 |
| `limit` | int | 返回数量（最大 100） |

### POST /admin/control-plane/compensations/replay

重放补偿。

---

## 管理 API — 模型治理

### GET /admin/governance/recommendations

列出模型推荐。

### POST /admin/governance/recommendations

创建模型推荐。

### GET /admin/governance/approvals

列出审批队列。

### POST /admin/governance/approvals

处理审批。

### GET /admin/governance/policy-versions

列出策略版本。

### GET /admin/governance/policy-versions/{id}

获取策略版本详情。

### GET /admin/governance/rollouts

列出灰度发布。

### POST /admin/governance/rollouts

创建灰度发布。

### GET /admin/governance/rollouts/{id}

获取灰度发布详情。

### GET /admin/governance/dashboard/rollouts

获取灰度发布仪表盘数据。

### GET /admin/governance/rollbacks

列出回滚记录。

### POST /admin/governance/rollbacks

执行回滚。

### GET /admin/governance/evaluations

列出评估结果。

### POST /admin/governance/evaluations

创建评估。

### GET /admin/governance/drifts

列出漂移检测。

### GET /admin/governance/drifts/{id}

获取漂移检测详情。

---

## 管理 API — 运行时

### GET /admin/governance/runtime-observer

运行时观测器。

**查询参数：**
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `environment` | string | `prod` | 环境 |
| `limit` | int | 20 | 返回数量 |

### GET /v1/runtime/resolve

解析运行时模型决策。

### GET /admin/governance/runtime/resolve

管理员解析运行时模型决策。

### GET /admin/governance/runtime-decisions

列出运行时决策。

### GET /admin/governance/distribution-events

列出分发事件。

---

## 管理 API — Memory Governance

### GET /admin/memory/candidate-facts

列出候选事实。

### POST /admin/memory/candidate-facts

确认/驳回候选事实。

### GET /admin/memory/candidate-facts/{id}

获取候选事实详情。

### GET /admin/memory/project-facts

列出项目事实。

---

## 管理 API — 补偿机制

### GET /admin/control-plane/compensations

列出补偿记录（含过滤和分页）。

---

## 管理 API — 配置中心

### GET /admin/config/site

获取站点配置。

### PUT /admin/config/site

更新站点配置。

### POST /admin/config/jwt/rotate

轮换 JWT 密钥。

### GET /admin/config/versions

列出配置版本。

### POST /admin/config/versions

创建配置版本。

### GET /admin/config-versions/{versionID}

获取配置版本详情。

---

## 管理 API — 配额管理

### GET /admin/observability/quota

获取配额摘要。

### GET /admin/observability/quota/trends

获取配额趋势。

---

## 管理 API — 广播

### GET /admin/broadcasts

列出广播消息。

### POST /admin/broadcasts

创建广播消息。

### DELETE /admin/broadcasts/{id}

删除广播消息。

---

## 用户 API — 认证

### POST /api/auth/signup

用户注册。

**请求体：**
```json
{
  "email": "user@example.com",
  "password": "secure_password"
}
```

### POST /api/auth/login

用户登录，返回 JWT Token。

**请求体：**
```json
{
  "email": "user@example.com",
  "password": "secure_password"
}
```

**响应：**
```json
{
  "token": "eyJ...",
  "user": {
    "id": 1,
    "email": "user@example.com"
  }
}
```

### GET /api/auth/me

获取当前用户信息。需要 JWT 认证。

### POST /api/auth/verify-email

验证邮箱。

### POST /api/auth/resend-verification

重新发送验证邮件。需要 JWT 认证。

### POST /api/auth/forgot-password

忘记密码，发送重置邮件。

### POST /api/auth/reset-password

重置密码。

### GET /api/auth/oauth/config

获取 OAuth 配置。

### GET /api/auth/oauth/github

GitHub OAuth 登录入口。

### GET /api/auth/oauth/github/callback

GitHub OAuth 回调。

---

## 用户 API — 用户操作

### GET /api/user/dashboard

用户仪表盘。需要 JWT 认证。

### GET /api/user/usage

用户用量统计。需要 JWT 认证。

### GET /api/user/usage-logs

用户用量日志。需要 JWT 认证。

### GET /api/user/cost-trend

用户成本趋势。需要 JWT 认证。

### GET /api/user/api-keys

列出用户 API Key。需要 JWT 认证。

### POST /api/user/api-keys

创建用户 API Key。需要 JWT 认证。

### DELETE /api/user/api-keys/{id}

撤销用户 API Key。需要 JWT 认证。

### GET /api/user/api-keys/{id}/usage

获取 API Key 用量。需要 JWT 认证。

### GET /api/user/api-keys/usage

获取所有 API Key 用量。需要 JWT 认证。

### GET /api/user/oauth

列出 OAuth 绑定。需要 JWT 认证。

### DELETE /api/user/oauth/{provider}

删除 OAuth 绑定。需要 JWT 认证。

---

## 用户 API — 记忆与预设

### GET /api/memory/presets

列出 Prompt 预设。需要 JWT 认证。

### POST /api/memory/presets

创建 Prompt 预设。需要 JWT 认证。

**请求体：**
```json
{
  "name": "代码审查",
  "template": "请审查以下代码：\n{{code}}",
  "variables": ["code"],
  "tags": ["review"],
  "is_public": false
}
```

### GET /api/memory/presets/{id}

获取预设详情。需要 JWT 认证。

### PUT /api/memory/presets/{id}

更新预设。需要 JWT 认证。

### DELETE /api/memory/presets/{id}

删除预设。需要 JWT 认证。

### GET /api/memory/masks

列出掩码规则。需要 JWT 认证。

### POST /api/memory/masks

创建掩码规则。需要 JWT 认证。

### PUT /api/memory/masks/{id}

更新掩码规则。需要 JWT 认证。

### DELETE /api/memory/masks/{id}

删除掩码规则。需要 JWT 认证。

---

## 用户 API — 文件解析

### POST /api/files/parse

上传并解析文件（PDF、Word、Markdown）。需要 JWT 认证。

**请求：** `multipart/form-data`，字段 `file`（最大 10MB）

**响应：**
```json
{
  "text": "文件内容...",
  "filename": "document.pdf",
  "size": 102400
}
```

---

## 用户 API — WebSocket

### GET /api/ws/chat

WebSocket 实时聊天端点。需要 JWT 认证。

**协议：**
- 发送 `{type: 'ping'}` → 心跳
- 发送 `{type: 'chat', content: '...'}` → 发送消息
- 接收 `{type: 'message', content: '...'}` → 接收消息
- 接收 `{type: 'error', message: '...'}` → 错误

---

## 用户 API — 配置版本

### GET /api/config/versions

列出配置版本。需要 JWT 认证。

### POST /api/config/rollback

回滚配置版本。需要 JWT 认证。

---

## 用户 API — 广播

### GET /api/broadcasts

获取广播消息列表。需要 JWT 认证。

### POST /api/broadcasts/{id}/read

标记广播为已读。需要 JWT 认证。

---

## 公共 API

### GET /api/openapi.json

获取 OpenAPI 3.0 规范（无需认证）。

---

## 错误响应格式

所有 API 错误统一返回以下格式：

```json
{
  "error": {
    "message": "错误描述",
    "type": "error_type"
  }
}
```

**HTTP 状态码：**
| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 429 | 限流 |
| 500 | 服务器内部错误 |
