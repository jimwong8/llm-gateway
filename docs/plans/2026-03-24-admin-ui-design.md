# 2026-03-24-admin-ui-design.md

## 1. 目标与范围

为现有 LLM Gateway 增加企业控制面前端（后端内置 Admin UI），重点覆盖 L4 资产管理闭环：

- 资产管理：列表、筛选、搜索、分页、软删除标记
- 统计可视化：总览卡片 + task/model/tag 维度分布
- 版本管理：版本查看、历史回滚入口
- 复用审计：复用来源与轨迹查询

本轮不新增后端业务语义，只消费已有管理接口。

---

## 2. 总体方案（已确认）

采用方案 A：在网关进程内托管 Admin UI 静态页面。

### 2.1 托管方式

- 后端提供前端入口：`/admin/ui`
- 同进程托管静态资源（HTML/CSS/JS）
- 前端直接调用现有管理 API（同域，减少跨域与部署复杂度）

### 2.2 信息架构

1. 资产页（主页面）
   - 列表、筛选、搜索、分页
   - 软删除状态标记
   - 操作列（查看版本、回滚入口、删除/更新入口）

2. 统计页
   - 基于 `/admin/assets/stats`
   - `overview` 总览卡片
   - `by_task`、`by_model`、`by_tag` 三组分布表

3. 版本页
   - 基于 `/admin/assets/versions`
   - 按资产查看历史版本

4. 审计页
   - 基于 `/admin/assets/reuse-audits`
   - 展示复用命中轨迹与来源

---

## 3. API 交互流（已确认）

### 3.1 资产与统计联动

- 资产列表：`GET /admin/assets`
  - 参数：`tenant_id/task_type/source_model/tag/keyword/include_deleted/limit/offset`
- 统计面板：`GET /admin/assets/stats`
  - 参数：`tenant_id/include_deleted/limit`
- 联动策略：筛选条件变化时，列表与统计并发刷新，且分页重置为 `offset=0`

### 3.2 版本与回滚

- 查看版本：`GET /admin/assets/versions`
  - 参数：`asset_id/tenant_id/limit/offset`
- 回滚版本：`POST /admin/assets/rollback`
  - Body：`asset_id/version/tenant_id`
- 回滚成功后：强制刷新资产列表 + 版本列表 + 统计面板

### 3.3 复用审计

- 查询审计：`GET /admin/assets/reuse-audits`
  - 参数：`tenant_id/limit/offset`
- 审计页默认按时间倒序展示

---

## 4. 认证与安全

- 前端统一透传管理密钥，沿用现有 `X-Admin-Key` 鉴权机制
- 不在 UI 中保存业务敏感字段，仅缓存必要筛选状态
- 管理端请求失败时，不暴露后端堆栈信息，按标准错误文案展示

---

## 5. 错误处理与交互约定

- 401：提示“管理密钥无效”，保留页面状态
- 404：提示“资源不存在或已删除”，并自动刷新当前列表
- 5xx：显示可重试按钮，保留筛选/分页上下文
- 写操作（回滚/删除/更新）：按钮禁用 + loading，完成后重拉对应数据源

---

## 6. 前端页面组件建议

### 6.1 通用组件

- 顶部过滤栏（tenant、task、model、tag、keyword、include_deleted）
- 数据表格组件（支持分页、空态、错误态）
- 统计卡片组件
- 右侧抽屉组件（版本详情）
- 操作确认弹窗（回滚/删除）

### 6.2 页面级组件

- AssetsPage
- StatsPage
- VersionsPanel（抽屉）
- ReuseAuditsPage

---

## 7. 数据流与状态管理

- Query State：由 URL query 持久化（便于分享与刷新恢复）
- Server State：按接口分区缓存
  - `assets`
  - `assetStats`
  - `assetVersions`
  - `assetReuseAudits`
- 写操作成功后采用精准失效：
  - 回滚：失效 `assets` + `assetStats` + 当前 `assetVersions`
  - 删除/更新：失效 `assets` + `assetStats`

---

## 8. 测试策略（针对 UI）

- 单元测试：
  - query 参数构建与解析
  - 错误状态映射（401/404/5xx）
- 集成测试：
  - 资产筛选与统计联动
  - 版本回滚后的三处刷新
- E2E（可选）：
  - 从资产页进入版本抽屉，执行回滚并验证统计变化

---

## 9. 验收标准

1. 能在 `/admin/ui` 打开管理端
2. 四个页面能力可用：资产、统计、版本、审计
3. 筛选、搜索、分页生效且状态可恢复
4. 回滚操作可触发并完成刷新闭环
5. 错误处理符合 401/404/5xx 约定

---

## 10. 非目标（本轮不做）

- 不引入多角色 RBAC（仅沿用管理员密钥）
- 不改造后端管理 API 协议
- 不做复杂图表系统（先用表格/卡片保证可用）
