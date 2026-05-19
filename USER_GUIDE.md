# LLM Gateway — 完整使用手册

> 适用版本：当前部署于 `http://10.100.1.17:8080`

---

## 一、系统概述

LLM Gateway 是一个企业级 AI 大模型中转站，核心作用是：

- **对外**：提供与 OpenAI 兼容的 API 接口，任何支持 OpenAI SDK 的客户端都能直接接入
- **对内**：统一管理多个 LLM 供应商（OpenAI、Anthropic、Google、Azure、AWS 等），自动路由、故障切换、缓存加速、审计合规

```
你的应用  ──→  LLM Gateway (http://10.100.1.17:8080)  ──→  OpenAI / Anthropic / Google / ...
                │
                ├── 智能路由（按能力/成本/延迟/健康度自动选择）
                ├── 多级缓存（精确缓存 → 语义缓存 → 会话记忆 → 资产复用）
                ├── 租户隔离（每个租户独立配额、密钥、策略）
                └── 审计合规（完整请求日志、数据导出、保留策略）
```

---

## 二、快速上手（5 分钟）

### 2.1 登录管理控制台

1. 浏览器打开 `http://10.100.1.17:8080/admin/ui`
2. 输入管理员 Token：`ok0115ok`
3. 点击「进入控制台」→ 自动跳转到仪表盘

### 2.2 配置第一个 LLM 渠道

这是使用系统的**第一步**，没有渠道就无法转发请求。

1. 左侧菜单 → **渠道管理**
2. 点击右上角「+ 添加渠道」
3. 填写表单：

| 字段 | 说明 | 示例 |
|------|------|------|
| 名称 | 渠道标识名 | `OpenAI-生产` |
| 供应商 | 选择 LLM 提供商 | `OpenAI` |
| Base URL | API 端点 | `https://api.openai.com/v1` |
| API Key | 供应商密钥 | `sk-...` |
| 优先级 | 路由优先级（最高/高/中/低/最低） | `高` |
| 权重 | 同优先级下的流量分配权重（1-100） | `10` |
| 模型列表 | 该渠道支持的模型 ID | `gpt-4o`、`gpt-4o-mini` |

4. 点击「保存」
5. 在渠道列表中点击「测试」验证连接是否成功

### 2.3 发起第一个请求

用 curl 测试（OpenAI 兼容格式）：

```bash
curl http://10.100.1.17:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <你的API-Key>" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "你好，请介绍一下你自己"}]
  }'
```

或者使用管理控制台内置的**在线测试**功能（无需 curl）：

1. 左侧菜单 → **在线测试**
2. 填写模型名（如 `gpt-4o-mini`）、租户 ID（如 `tenant-a`）
3. 在消息内容框输入你想说的话
4. 点击「发送请求」
5. 右侧面板会显示响应内容、状态码、耗时、缓存命中情况

---

## 三、核心功能模块详解

### 3.1 仪表盘（首页）

**位置**：登录后默认页面 / 左侧菜单「仪表盘」

**功能**：
- **服务状态卡片**：显示 llm-gateway 服务健康状态、管理员认证状态、请求总量、缓存命中率、Provider 错误率、总 Token 数
- **会话与运维概览**：总体健康状态、续接状态、重复分组数、告警问题数、共享项目会话数、待续接数、知识图谱成功/失败数
- **最近告警**：仅显示前 3 条问题摘要
- **AI 运维建议**：系统自动生成的优化建议
- **最近操作**：展示最近 3 条操作历史
- **图表 Tab**：
  - Token 趋势：Completion/Prompt Tokens 按日期变化折线图
  - 模型分布：各模型使用量分布
  - 缓存命中：各级缓存命中率
  - 渠道状态：各渠道健康状态

---

### 3.2 渠道管理

**位置**：左侧菜单「管理 → 渠道管理」

**作用**：管理 LLM 供应商连接配置，系统支持 OpenAI、Anthropic、Google AI、Azure OpenAI、AWS Bedrock、自定义共 6 类供应商。

**操作流程**：

#### 添加渠道
1. 点击「+ 添加渠道」
2. 基础信息：名称、供应商、Base URL、API Key
3. 路由配置：优先级（最高/高/中/低/最低）、权重（1-100）、模型列表（输入模型 ID 后回车添加）
4. 高级配置：标签、备注
5. 点击「保存」

#### 编辑渠道
- 点击行末「编辑」按钮 → 修改 → 保存

#### 测试渠道
- 点击行末「测试」按钮 → 系统发送探测请求 → 弹窗显示「测试成功」或「测试失败: 原因」

#### 删除渠道
- 点击行末「删除」→ 确认弹窗 → 删除

#### 批量操作
1. 勾选多条渠道（或使用表头全选）
2. 出现批量操作栏：
   - 「批量启用」：将选中渠道状态改为 active
   - 「批量停用」：将选中渠道状态改为 inactive
   - 「批量删除」：删除选中渠道（需二次确认）

#### 搜索与筛选
- 搜索框：按渠道名称模糊搜索
- 状态下拉：全部/启用/停用/异常/维护中

---

### 3.3 资产管理

**位置**：左侧菜单「管理 → 资产管理」

**作用**：浏览和管理系统自动生成的知识资产（标准化摘要、结构化抽取等），支持搜索、筛选、分页和删除。

**操作流程**：
1. 顶部搜索框：按标题或内容关键词搜索
2. 类型下拉：按任务类型筛选（code/analysis/general）
3. 资产列表展示：ID、标题、类型、来源模型、命中次数、创建时间
4. 删除：点击行末「删除」→ 确认 → 删除
5. 分页：底部分页控件（上一页/下一页/页码）

**统计卡片**（页面顶部）：总资产数、总命中次数、各类型资产数量

---

### 3.4 配置中心

**位置**：左侧菜单「管理 → 配置中心」

**作用**：管理配置版本，支持配置的继承、发布、回滚。

---

### 3.5 发布管理

**位置**：左侧菜单「管理 → 发布管理」

**作用**：管理配置发布流程，支持发布草稿和执行推广。

**操作流程**：
1. 查看发布列表（发布 ID、策略版本、环境、状态、推广比例、错误率、P95 延迟、回退率、样本数、触发人、更新时间）
2. 「发布草稿」：创建新的发布草稿
3. 「执行推广」：将草稿推广到目标环境

---

### 3.6 审计与运行时

**位置**：左侧菜单「监控 → 审计与运行时」

**作用**：查看审计事件和运行时事件，支持按租户、环境筛选。

**操作流程**：
1. 顶部筛选：租户 ID、环境、条数限制
2. Tab 切换：「审计事件」/「运行时事件」
3. 事件列表展示

---

### 3.7 审计导出

**位置**：左侧菜单「监控 → 审计导出」

**作用**：导出租户审计数据、执行数据清理、查看保留策略。

**操作流程**：
1. 输入租户 ID
2. 选择导出格式（JSON / CSV）
3. 点击「导出」下载数据文件
4. 点击「立即清理」执行数据清理（按保留天数删除过期数据）
5. 查看当前保留策略配置

---

### 3.8 可观测性

**位置**：左侧菜单「监控 → 可观测性」

**作用**：查看系统整体运行指标。

**展示内容**：
- 各 Provider 的请求量、总 Token 数、错误率
- 缓存命中率
- 延迟分布

---

### 3.9 漂移仪表盘

**位置**：左侧菜单「监控 → 漂移仪表盘」

**作用**：检测模型输出漂移，监控模型质量变化。

---

### 3.10 运行时观测

**位置**：左侧菜单「监控 → 运行时观测」

**作用**：实时观察运行时状态，包括活跃策略、缓存状态、运行时决策、分发事件。

**操作流程**：
1. 选择环境（如 `prod`）
2. 点击「刷新观察数据」获取最新状态
3. 查看运行时决策列表和分发事件

---

### 3.11 策略管理

**位置**：左侧菜单「策略 → 策略管理」

**作用**：管理租户级别的模型路由策略。

---

### 3.12 策略版本

**位置**：左侧菜单「策略 → 策略版本」

**作用**：管理策略的版本历史，支持审批、激活、差异对比。

---

### 3.13 审批管理

**位置**：左侧菜单「策略 → 审批管理」

**作用**：处理待审批的配置变更和模型推荐。

---

### 3.14 灰度发布

**位置**：左侧菜单「策略 → 灰度发布」

**作用**：管理灰度发布流程，支持按比例逐步推广和回滚。

**监控指标**：平均错误率、P95 延迟、回退率

---

### 3.15 配额管理

**位置**：左侧菜单「系统 → 配额管理」

**作用**：管理各租户的配额限制（RPM、Token 上限等），查看使用量趋势。

---

### 3.16 记忆治理

**位置**：左侧菜单「系统 → 记忆治理」

**作用**：管理会话记忆和知识图谱事实，支持候选事实/项目事实的确认、驳回、提升。

---

### 3.17 租户密钥（BYOK）

**位置**：左侧菜单「系统 → 租户密钥」

**作用**：为每个租户配置独立的 API 密钥（Bring Your Own Key），租户密钥优先于全局密钥使用。

**操作流程**：

#### 添加密钥
1. 点击「添加密钥」
2. 填写：
   - 租户 ID（如 `tenant-xxx`）
   - Provider（如 `openai`）
   - API Key（如 `sk-...`）
3. 点击「保存」

#### 删除密钥
- 点击行末「删除」→ 确认 → 删除

#### 搜索
- 顶部搜索框按租户 ID 模糊搜索

---

### 3.18 在线测试（Playground）

**位置**：左侧菜单「系统 → 在线测试」

**作用**：直接在浏览器里发起 LLM 请求，无需 curl 或 Postman，用于快速验证网关和渠道配置。

**操作流程**：
1. 填写模型名（如 `gpt-4o-mini`）
2. 填写租户 ID（如 `tenant-a`）
3. （可选）填写任务提示（`analysis` / `code` / `chat`）
4. 在消息编辑器中输入对话内容
   - 可添加多条消息
   - 每条消息可设置角色（user/assistant/system）和内容
5. 点击「发送请求」
6. 右侧响应面板显示：
   - 状态码（200 = 成功）
   - 耗时（毫秒）
   - X-Cache（缓存命中状态：HIT / MISS）
   - X-Semantic-Score（语义缓存相似度分数）
   - 响应 JSON 全文
   - 请求预览（实际发送的 JSON）
   - 最近请求历史（可点击快速回填）

---

### 3.19 系统状态

**位置**：左侧菜单「系统 → 系统状态」

**作用**：查看系统整体健康状态和各组件状态。

---

## 四、API 接口参考

### 4.1 生产 API（OpenAI 兼容）

所有请求地址：`http://10.100.1.17:8080`

#### 聊天补全
```http
POST /v1/chat/completions
Authorization: Bearer <api-key>
Content-Type: application/json

{
  "model": "gpt-4o-mini",
  "messages": [
    {"role": "system", "content": "你是一个有帮助的助手"},
    {"role": "user", "content": "你好"}
  ],
  "tenant_id": "tenant-a",
  "task_hint": "chat"
}
```

#### 模型列表
```http
GET /v1/models
Authorization: Bearer <api-key>
```

#### Embedding
```http
POST /v1/embeddings
Authorization: Bearer <api-key>
Content-Type: application/json

{
  "model": "text-embedding-3-small",
  "input": "需要嵌入的文本"
}
```

### 4.2 管理 API

所有管理接口需要携带管理员 Token：
```
Authorization: Bearer ok0115ok
```

#### 渠道管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/channels` | 渠道列表 |
| POST | `/admin/channels` | 创建渠道 |
| GET | `/admin/channels/{id}` | 渠道详情 |
| PUT | `/admin/channels/{id}` | 更新渠道 |
| DELETE | `/admin/channels/{id}` | 删除渠道 |
| POST | `/admin/channels/{id}/test` | 测试渠道连接 |
| POST | `/admin/channels/batch-delete` | 批量删除 |
| POST | `/admin/channels/batch-status` | 批量更新状态 |

#### 资产管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/assets?keyword=xxx&task_type=code&limit=20&offset=0` | 资产列表 |
| GET | `/admin/assets/{id}` | 资产详情 |
| DELETE | `/admin/assets/{id}` | 删除资产 |
| GET | `/admin/assets/stats` | 资产统计 |

#### 租户密钥
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/tenant-keys?tenant_id=xxx` | 密钥列表 |
| POST | `/admin/tenant-keys` | 创建/更新密钥 |
| DELETE | `/admin/tenant-keys/{tenant_id}/{provider}` | 删除密钥 |

#### 审计与合规
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/audit/export?tenant_id=xxx&format=json` | 导出审计数据 |
| POST | `/admin/audit/cleanup?retention_days=90` | 清理过期数据 |
| GET | `/admin/audit/retention` | 查看保留策略 |

#### 运行时观测
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/governance/runtime-observer?environment=prod&limit=20` | 运行时状态 |

#### 仪表盘
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/dashboard` | 仪表盘摘要 |
| GET | `/admin/dashboard/charts/token-usage` | Token 使用趋势 |
| GET | `/admin/dashboard/charts/model-distribution` | 模型分布 |
| GET | `/admin/dashboard/charts/cache-hit-rate` | 缓存命中率 |
| GET | `/admin/dashboard/charts/channel-status` | 渠道状态 |

#### 健康检查
```http
GET /healthz
```

---

## 五、典型使用场景

### 场景 1：快速接入 OpenAI

1. 登录控制台 → 渠道管理 → 添加渠道
2. 供应商选 `OpenAI`，填入你的 `sk-...` 密钥
3. 添加模型：`gpt-4o`、`gpt-4o-mini`
4. 保存并测试
5. 在你的应用中：
   ```python
   from openai import OpenAI
   client = OpenAI(
       base_url="http://10.100.1.17:8080/v1",
       api_key="your-api-key"
   )
   response = client.chat.completions.create(
       model="gpt-4o-mini",
       messages=[{"role": "user", "content": "你好"}]
   )
   ```

### 场景 2：多供应商高可用

1. 添加多个渠道（如 OpenAI + Anthropic + Azure）
2. 设置优先级：OpenAI 为「高」，Anthropic 为「中」，Azure 为「低」
3. 系统自动按优先级路由，OpenAI 故障时自动切换到 Anthropic

### 场景 3：租户隔离

1. 在租户密钥页面为 `tenant-a` 配置专属 OpenAI 密钥
2. 在租户密钥页面为 `tenant-b` 配置专属 Anthropic 密钥
3. 请求时携带 `tenant_id` 参数，系统自动使用对应密钥

### 场景 4：在线调试

1. 进入「在线测试」页面
2. 填写模型和消息
3. 发送请求，查看响应和缓存状态
4. 根据 X-Cache 和 X-Semantic-Score 判断缓存命中情况

### 场景 5：审计与合规

1. 进入「审计导出」页面
2. 输入租户 ID，选择 JSON 或 CSV 格式
3. 点击「导出」下载数据
4. 定期点击「立即清理」删除过期审计数据

---

## 六、常见问题

**Q: 渠道测试失败？**
- 检查 API Key 是否正确
- 检查 Base URL 是否可访问
- 检查网络连通性

**Q: 请求返回 401？**
- 检查 Authorization Header 是否正确
- 检查 API Key 是否有效

**Q: 请求返回 502/503？**
- 检查对应供应商服务是否正常
- 查看「渠道管理」中渠道状态是否为「异常」
- 查看「运行时观测」页面排查

**Q: 缓存未命中？**
- 首次请求不会命中缓存
- 语义缓存需要配置 Qdrant 才能生效
- 检查「可观测性」页面的缓存命中率

**Q: 如何切换模型？**
- 修改请求中的 `model` 字段
- 确保对应渠道已配置该模型

---

## 七、运维命令

```bash
# 查看服务状态
systemctl status llm-gateway

# 重启服务
systemctl restart llm-gateway

# 查看日志
journalctl -u llm-gateway -f

# 健康检查
curl http://10.100.1.17:8080/healthz

# 管理 API 健康检查
curl -H "Authorization: Bearer ok0115ok" http://10.100.1.17:8080/admin/health
```
