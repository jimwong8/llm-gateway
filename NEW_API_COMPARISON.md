# new-api vs llm-gateway 前端功能对比 & 优化改进清单

> 基于对 https://github.com/QuantumNous/new-api 前端（web/default + web/classic）的深入研究
> 对比目标：llm-gateway（当前项目 /home/jimwong/projects/llm-gateway）

---

## 一、技术栈对比

| 维度 | new-api (default) | llm-gateway (当前) | 建议 |
|------|-------------------|-------------------|------|
| **React 版本** | React 19 | React 18 | ✅ 升级到 React 19 |
| **构建工具** | Rsbuild 2 (极速) | Vite 5 | ⚡ 考虑迁移到 Rsbuild，构建速度提升 5-10x |
| **CSS 方案** | Tailwind CSS v4 | 纯 CSS (BEM) | ✅ 迁移到 Tailwind CSS v4，大幅提升开发效率 |
| **UI 组件库** | Base UI + shadcn (60+ 组件) | 自研 (少量) | ✅ 引入 shadcn/ui 组件库 |
| **路由** | TanStack Router v1 (文件系统) | React Router v6 | ⚡ 考虑迁移，类型安全 + 自动路由树 |
| **状态管理** | Zustand v5 + TanStack Query v5 | TanStack Query v5 | ✅ 引入 Zustand 替代分散的 useState |
| **图表** | VChart + Recharts 3.8 | Recharts (旧版) | ✅ 升级 Recharts，考虑引入 VChart |
| **动画** | Motion (Framer Motion 12) | 无 | ✅ 引入 Motion 提升交互体验 |
| **通知** | Sonner (toast) | alert() | ✅ 引入 Sonner 替换原生 alert |
| **表单** | react-hook-form v7 + zod v4 | 原生表单 | ✅ 引入 react-hook-form + zod |
| **国际化** | i18next (6 种语言) | 无 | ✅ 引入 i18next 国际化 |
| **包管理器** | Bun | npm | ⚡ 考虑迁移到 Bun，安装速度提升 10x |
| **TypeScript** | 5.9 (严格模式) | 5.x | ✅ 升级并开启严格模式 |
| **代码规范** | ESLint 10 + Prettier 3.8 | 无 | ✅ 引入 ESLint + Prettier |
| **请求去重** | 内置 GET 去重 | 无 | ✅ 实现请求去重机制 |
| **SSE** | sse.js (流式聊天) | 无 | ✅ 引入 SSE 支持流式响应 |

---

## 二、功能模块对比

### 2.1 new-api 有但 llm-gateway 缺失的功能模块

| 模块 | new-api 功能 | llm-gateway 现状 | 优先级 | 说明 |
|------|-------------|-----------------|--------|------|
| **🔐 用户注册/登录** | 完整注册、登录、找回密码、OTP、OAuth (GitHub/微信/Discord)、Passkey | 仅管理员 Token 登录 | 🔴 P0 | 需要完整用户体系 |
| **💬 AI 聊天 (Chat)** | 完整聊天界面，支持流式响应、多模型切换、聊天分享链接 | 无 | 🔴 P0 | 核心功能缺失 |
| **💰 钱包/充值** | 余额查看、充值码兑换、Stripe/Creem/Waffo 支付、推广返利 | 无 | 🔴 P0 | 商业化能力 |
| **📦 订阅管理** | 订阅计划、Stripe/Creem 订阅支付、订阅状态管理 | 无 | 🟡 P1 | SaaS 必备 |
| **🔑 API Key (Token) 管理** | 用户级 API Key 创建/搜索/分页/批量操作/获取明文密钥 | 仅有租户密钥 | 🔴 P0 | 用户需要自己的 API Key |
| **📊 数据面板 (Dashboard)** | 配额使用、请求量统计、余额概览、图表 | 仅有服务状态 | 🟡 P1 | 用户需要看到自己的数据 |
| **👤 用户管理 (Admin)** | 用户 CRUD、搜索、分页、角色管理、2FA 重置、Passkey 重置 | 无 | 🔴 P0 | 管理员需要管理用户 |
| **🔧 系统设置** | 分模块设置：认证/模型/支付/站点/安全/内容/运营/请求限制/维护 | 无 | 🔴 P0 | 系统配置能力 |
| **🏷️ 模型管理 (Models)** | 模型 CRUD、搜索、分页、同步上游、缺失模型检测、供应商管理 | 无 | 🔴 P0 | 核心功能 |
| **🚀 模型部署 (Deployments)** | 部署管理、容器管理、硬件类型、位置选择、价格估算 | 无 | 🟢 P2 | 高级功能 |
| **🎫 兑换码 (Redemption)** | 兑换码 CRUD、搜索、批量生成、清理无效码 | 无 | 🟡 P1 | 运营工具 |
| **📈 性能指标** | 模型性能指标摘要和详情 | 无 | 🟢 P2 | 高级监控 |
| **🏆 排行榜** | 用户使用量排名 | 无 | 🟢 P2 | 社区功能 |
| **💳 支付集成** | Stripe、Creem、Waffo、Epay 多支付渠道 | 无 | 🟡 P1 | 商业化必备 |
| **🔗 OAuth 绑定** | GitHub、微信、Discord、OIDC 多平台 OAuth | 无 | 🟡 P1 | 登录方式丰富化 |
| **🛡️ 2FA/Passkey** | 双因素认证、WebAuthn 无密码登录 | 无 | 🟡 P1 | 安全增强 |
| **🌐 国际化** | 中/英/法/俄/日/越 6 种语言 | 无 | 🟡 P1 | 国际化支持 |
| **🎨 主题系统** | 亮/暗主题、主题定制 (预设/圆角/缩放/布局) | 无 | 🟢 P2 | 用户体验 |
| **📱 响应式/移动端** | 完整响应式布局、移动端适配 | 基础响应式 | 🟡 P1 | 移动端体验 |
| **🔔 通知系统** | 系统公告、通知已读状态、关闭直到某日期 | 无 | 🟡 P1 | 运营通知 |
| **🔍 命令面板** | Cmd+K 全局搜索/导航 | 无 | 🟢 P2 | 效率工具 |
| **📋 使用日志 (Logs)** | 详细 API 调用日志、多维度筛选、统计、Midjourney/任务日志 | 基础审计日志 | 🔴 P0 | 用户需要查看自己的调用记录 |
| **👥 用户分组** | 分组管理、分组倍率 | 无 | 🟡 P1 | 差异化定价 |
| **🔄 倍率同步** | 从上游拉取模型倍率 | 无 | 🟢 P2 | 运维工具 |
| **📄 法律页面** | 隐私政策、用户协议 | 无 | 🟡 P1 | 合规要求 |
| **⚙️ 首次安装向导** | 初始化管理员账号和系统配置 | 无 | 🟡 P1 | 降低部署门槛 |

### 2.2 llm-gateway 有但 new-api 缺失的功能模块

| 模块 | llm-gateway 功能 | 说明 |
|------|-----------------|------|
| **📦 资产管理** | 知识资产浏览、搜索/筛选/分页、删除、统计 | L4 资产复用层 |
| **🔄 发布管理** | 发布草稿、执行推广、灰度发布 | 控制面/数据面分离 |
| **📡 运行时观测** | 活跃策略、缓存状态、运行时决策、分发事件 | 实时监控 |
| **🔍 漂移检测** | 模型漂移检测仪表盘 | 模型质量监控 |
| **📋 审批管理** | 审批队列、模型推荐审批 | 治理工作流 |
| **🧠 记忆治理** | 候选事实/项目事实管理、确认/驳回/提升 | L3 会话记忆 |
| **🔍 可观测性** | Provider 请求量/Token/成本/延迟/缓存命中/错误率 | 详细指标 |

---

## 三、具体优化改进清单（按优先级排序）

### 🔴 P0 — 核心缺失，必须添加

#### 1. 完整用户认证体系
```
新增功能：
- 用户注册（邮箱验证）
- 用户登录（用户名/密码）
- 找回密码（邮件重置）
- 2FA 双因素认证
- OAuth 登录（GitHub 优先）
- Passkey/WebAuthn 无密码登录

影响文件：
- web/admin/src/pages/LoginPage.tsx → 重构为完整登录页
- 新增：SignUpPage.tsx、ForgotPasswordPage.tsx、ResetPasswordPage.tsx
- 新增：features/auth/ 模块
- 新增：stores/auth-store.ts (Zustand)
- 后端：需要添加用户注册/登录/2FA API
```

#### 2. AI 聊天功能 (Chat)
```
新增功能：
- 聊天界面（流式响应）
- 多模型切换
- 聊天历史
- 聊天分享链接 (chat2link)
- SSE 流式输出

影响文件：
- 新增：features/chat/ 完整模块
- 新增：routes/_authenticated/chat/
- 新增：components/ai-elements/ (AI 相关 UI 组件)
- 新增：hooks/use-streaming.ts
- 后端：需要支持 SSE 流式响应
```

#### 3. API Key (Token) 管理
```
新增功能：
- 用户级 API Key CRUD
- 搜索/分页/批量操作
- 获取明文密钥（带 2FA 验证）
- Key 状态管理（启用/禁用）

影响文件：
- 新增：features/keys/ 模块
- 新增：routes/_authenticated/keys/
- 后端：需要 Token 管理 API
```

#### 4. 用户管理 (Admin)
```
新增功能：
- 用户列表（搜索/分页）
- 用户 CRUD
- 角色管理
- 2FA/Passkey 重置
- 用户分组管理

影响文件：
- 新增：features/users/ 模块
- 新增：routes/_authenticated/users/
- 后端：需要用户管理 API
```

#### 5. 系统设置
```
新增功能：
- 分模块设置页面：
  - 认证设置（登录方式、注册配置）
  - 模型设置（默认模型、倍率）
  - 支付设置（Stripe/Creem 配置）
  - 站点设置（名称、Logo、公告）
  - 安全设置（Turnstile、2FA 策略）
  - 内容设置（首页内容、法律页面）
  - 运营设置（签到、邀请）
  - 请求限制（RPM 限制）
  - 维护模式
- 仅提交脏数据（diff 提交）

影响文件：
- 新增：features/system-settings/ 完整模块
- 新增：routes/_authenticated/system-settings/
- 新增：hooks/use-settings-form.ts
- 后端：需要选项管理 API
```

#### 6. 模型管理 (Models)
```
新增功能：
- 模型 CRUD（创建/编辑/删除）
- 搜索/分页
- 同步上游模型
- 缺失模型检测
- 供应商 (Vendor) 管理
- 模型部署管理

影响文件：
- 新增：features/models/ 完整模块
- 新增：routes/_authenticated/models/
- 后端：需要模型管理 API
```

#### 7. 数据面板 (Dashboard) — 用户视角
```
新增功能：
- 配额使用概览
- 请求量统计图表
- 余额/消费趋势
- Token 使用分布
- 模型使用分布

影响文件：
- 重构：features/dashboard/ 模块
- 新增：hooks/use-status.ts
- 后端：需要用户数据 API
```

#### 8. 使用日志 (Usage Logs)
```
新增功能：
- API 调用日志列表
- 多维度筛选（时间/模型/Token/分组/渠道）
- 日志统计
- 导出功能
- Midjourney/任务日志

影响文件：
- 新增：features/usage-logs/ 模块
- 新增：routes/_authenticated/usage-logs/
- 后端：需要日志查询 API
```

---

### 🟡 P1 — 重要增强，建议添加

#### 9. 钱包/充值系统
```
新增功能：
- 余额查看
- 充值码兑换
- 支付集成（Stripe 优先）
- 充值记录
- 推广返利

影响文件：
- 新增：features/wallet/ 模块
- 新增：routes/_authenticated/wallet/
- 后端：需要充值/支付 API
```

#### 10. 订阅管理
```
新增功能：
- 订阅计划 CRUD
- 用户订阅管理
- Stripe/Creem 订阅支付
- 订阅状态管理

影响文件：
- 新增：features/subscriptions/ 模块
- 新增：routes/_authenticated/subscriptions/
- 后端：需要订阅 API
```

#### 11. 兑换码管理
```
新增功能：
- 兑换码 CRUD
- 批量生成
- 搜索/分页
- 清理无效码

影响文件：
- 新增：features/redemption-codes/ 模块
- 新增：routes/_authenticated/redemption-codes/
- 后端：需要兑换码 API
```

#### 12. 国际化 (i18n)
```
新增功能：
- i18next 集成
- 中/英双语支持
- 语言自动检测
- 语言切换器

影响文件：
- 新增：i18n/ 目录
- 新增：context/i18n-provider.tsx
- 新增：components/language-switcher.tsx
- 修改：所有页面组件添加翻译 key
```

#### 13. 主题系统
```
新增功能：
- 亮/暗主题切换
- 主题定制（预设/圆角/缩放）
- 主题持久化 (Cookie)

影响文件：
- 新增：context/theme-provider.tsx
- 新增：components/theme-switch.tsx
- 修改：CSS 添加 CSS 变量支持
```

#### 14. 通知系统
```
新增功能：
- 系统公告
- 通知已读状态
- Sonner toast 通知（替换 alert）

影响文件：
- 新增：stores/notification-store.ts
- 新增：components/notification-button.tsx
- 修改：所有 alert() 替换为 toast
```

#### 15. 支付集成
```
新增功能：
- Stripe 支付
- Creem 支付
- 多支付渠道配置

影响文件：
- 新增：features/payment/ 模块
- 后端：需要支付 API
```

#### 16. OAuth 登录
```
新增功能：
- GitHub OAuth
- 微信登录
- Discord/OIDC

影响文件：
- 新增：features/auth/oauth/
- 新增：routes/oauth/
- 后端：需要 OAuth API
```

#### 17. 2FA/Passkey 安全
```
新增功能：
- 2FA 设置/启用/禁用
- 备份码管理
- Passkey/WebAuthn 无密码登录

影响文件：
- 新增：features/auth/2fa/
- 新增：features/auth/passkey/
- 后端：需要 2FA/Passkey API
```

#### 18. 法律页面
```
新增功能：
- 隐私政策页面
- 用户协议页面
- 内容可配置

影响文件：
- 新增：routes/privacy-policy.tsx
- 新增：routes/user-agreement.tsx
- 新增：features/legal/ 模块
```

#### 19. 首次安装向导
```
新增功能：
- 初始化管理员账号
- 基础系统配置
- 引导式设置流程

影响文件：
- 新增：features/setup/ 模块
- 新增：routes/setup/
- 后端：需要 setup API
```

#### 20. 用户分组
```
新增功能：
- 分组 CRUD
- 分组倍率设置
- 用户分组分配

影响文件：
- 新增：features/groups/ 模块
- 后端：需要分组 API
```

---

### 🟢 P2 — 体验优化，有时间再做

#### 21. 技术栈升级
```
- React 18 → React 19
- Vite → Rsbuild (构建速度提升 5-10x)
- 纯 CSS → Tailwind CSS v4
- npm → Bun (包管理速度提升 10x)
- 引入 shadcn/ui 组件库
- 引入 Motion (Framer Motion) 动画
- 引入 react-hook-form + zod 表单
- 引入 VChart 图表库
- TypeScript 严格模式
- ESLint + Prettier 代码规范
```

#### 22. 模型部署管理
```
- 部署 CRUD
- 容器管理
- 硬件类型/位置选择
- 价格估算
```

#### 23. 性能指标监控
```
- 模型性能指标摘要
- 单模型性能详情
```

#### 24. 排行榜
```
- 用户使用量排名
- 多维度排名
```

#### 25. 倍率同步
```
- 从上游拉取模型倍率
- 预览变更
```

#### 26. 命令面板
```
- Cmd+K 全局搜索
- 快速导航
```

#### 27. 响应式/移动端优化
```
- 完整移动端适配
- 触摸优化
- 底部导航栏
```

---

## 四、架构改进建议

### 4.1 状态管理重构

**当前问题**：llm-gateway 前端几乎没有客户端状态管理，大量使用 useState

**建议方案**（参考 new-api）：
```
stores/
├── auth-store.ts          # 认证状态 (Zustand + localStorage)
├── notification-store.ts  # 通知状态 (Zustand + persist)
└── system-config-store.ts # 系统配置 (Zustand + persist)

hooks/
├── use-admin.ts           # 管理员权限检查
├── use-status.ts          # 系统状态（同步到 store）
├── use-table-url-state.ts # 表格 URL 状态同步
├── use-dialog.ts          # 对话框状态管理
└── use-debounce.ts        # 防抖
```

### 4.2 API 层重构

**当前问题**：API 调用分散，无统一错误处理、无请求去重

**建议方案**（参考 new-api）：
```
lib/
├── api.ts                 # Axios 实例 + 拦截器 + 请求去重
├── handle-server-error.ts # 统一错误处理
├── roles.ts               # 角色常量
└── secure-verification.ts # 安全验证检测

features/<module>/
├── api.ts                 # 模块 API 函数
├── types.ts               # 类型定义
├── hooks/                 # 数据 hooks
├── lib/                   # 工具函数
└── components/            # 模块组件
```

### 4.3 路由重构

**当前问题**：React Router v6 手动配置路由，无类型安全

**建议方案**（参考 new-api 的 TanStack Router）：
```
routes/
├── __root.tsx             # 根路由
├── index.tsx              # 首页
├── (auth)/                # 认证路由分组
│   ├── sign-in.tsx
│   ├── sign-up.tsx
│   └── ...
├── _authenticated/        # 需认证路由分组
│   ├── route.tsx          # 认证守卫
│   ├── dashboard/
│   ├── chat/
│   ├── channels/
│   ├── keys/
│   ├── models/
│   ├── users/
│   └── ...
└── (errors)/              # 错误页面分组
    ├── 401.tsx
    ├── 403.tsx
    ├── 404.tsx
    └── ...
```

### 4.4 组件库建设

**当前问题**：自研组件少，无统一设计系统

**建议方案**（参考 new-api 的 shadcn/ui）：
```
components/
├── ui/                    # 基础 UI 组件 (~60 个)
│   ├── accordion.tsx
│   ├── alert-dialog.tsx
│   ├── alert.tsx
│   ├── avatar.tsx
│   ├── badge.tsx
│   ├── breadcrumb.tsx
│   ├── button.tsx
│   ├── calendar.tsx
│   ├── card.tsx
│   ├── carousel.tsx
│   ├── chart.tsx
│   ├── checkbox.tsx
│   ├── combobox.tsx
│   ├── command.tsx
│   ├── dialog.tsx
│   ├── drawer.tsx
│   ├── dropdown-menu.tsx
│   ├── form.tsx
│   ├── input.tsx
│   ├── input-otp.tsx
│   ├── markdown.tsx
│   ├── navigation-menu.tsx
│   ├── pagination.tsx
│   ├── popover.tsx
│   ├── select.tsx
│   ├── sheet.tsx
│   ├── sidebar.tsx
│   ├── skeleton.tsx
│   ├── sonner.tsx
│   ├── switch.tsx
│   ├── table.tsx
│   ├── tabs.tsx
│   ├── textarea.tsx
│   ├── tooltip.tsx
│   └── ...
├── data-table/            # 数据表格封装
├── layout/                # 布局系统
│   ├── sidebar/
│   ├── header/
│   └── ...
└── ...
```

---

## 五、实施路线图

### 第一阶段 (1-2 周) — 基础架构升级
1. ✅ 引入 Tailwind CSS v4
2. ✅ 引入 shadcn/ui 组件库
3. ✅ 引入 Zustand 状态管理
4. ✅ 重构 API 层（统一拦截器、错误处理、请求去重）
5. ✅ 引入 react-hook-form + zod
6. ✅ 引入 Sonner toast（替换 alert）

### 第二阶段 (2-3 周) — 核心功能添加
7. ✅ 完整用户认证体系（注册/登录/找回密码）
8. ✅ 用户管理 (Admin)
9. ✅ API Key (Token) 管理
10. ✅ 模型管理
11. ✅ 系统设置
12. ✅ 数据面板（用户视角）

### 第三阶段 (2-3 周) — 聊天与商业化
13. ✅ AI 聊天功能
14. ✅ 使用日志
15. ✅ 钱包/充值系统
16. ✅ 订阅管理
17. ✅ 支付集成

### 第四阶段 (1-2 周) — 增强功能
18. ✅ 国际化
19. ✅ 主题系统
20. ✅ 通知系统
21. ✅ OAuth 登录
22. ✅ 2FA/Passkey
23. ✅ 兑换码管理
24. ✅ 法律页面
25. ✅ 首次安装向导

### 第五阶段 (持续) — 体验优化
26. ⚡ 技术栈升级（React 19、Rsbuild、Bun）
27. ⚡ 路由重构（TanStack Router）
28. ⚡ 响应式/移动端优化
29. ⚡ 性能指标监控
30. ⚡ 排行榜
31. ⚡ 命令面板

---

## 六、冲突说明

以下 new-api 功能与 llm-gateway 现有架构**无冲突**，可直接借鉴：

| new-api 功能 | 冲突情况 | 说明 |
|-------------|---------|------|
| 用户认证体系 | ✅ 无冲突 | llm-gateway 目前只有管理员 Token，需扩展 |
| AI 聊天 | ✅ 无冲突 | 全新功能 |
| API Key 管理 | ✅ 无冲突 | 与现有租户密钥不冲突，是用户级 Key |
| 模型管理 | ✅ 无冲突 | 与现有渠道管理互补 |
| 系统设置 | ✅ 无冲突 | 全新功能 |
| 数据面板 | ✅ 无冲突 | 扩展现有仪表盘 |
| 使用日志 | ✅ 无冲突 | 扩展现有审计日志 |
| 钱包/充值 | ✅ 无冲突 | 全新功能 |
| 订阅管理 | ✅ 无冲突 | 全新功能 |
| 兑换码 | ✅ 无冲突 | 全新功能 |
| 国际化 | ✅ 无冲突 | 全新功能 |
| 主题系统 | ✅ 无冲突 | 全新功能 |
| 通知系统 | ✅ 无冲突 | 全新功能 |
| OAuth | ✅ 无冲突 | 全新功能 |
| 2FA/Passkey | ✅ 无冲突 | 全新功能 |
| 支付集成 | ✅ 无冲突 | 全新功能 |
| 用户分组 | ✅ 无冲突 | 全新功能 |
| 法律页面 | ✅ 无冲突 | 全新功能 |
| 安装向导 | ✅ 无冲突 | 全新功能 |
| 响应式优化 | ✅ 无冲突 | 增强现有 |
| 图表升级 | ✅ 无冲突 | 增强现有 |
| 状态管理 | ✅ 无冲突 | 增强现有 |
| API 层重构 | ✅ 无冲突 | 增强现有 |
| 组件库 | ✅ 无冲突 | 增强现有 |

**唯一需要注意的冲突**：
- llm-gateway 的"租户密钥 (TenantKeys)"与 new-api 的"API Key (Token)"概念类似但不同
  - llm-gateway：租户级别的 Provider API Key（BYOK）
  - new-api：用户级别的 API 访问密钥
  - **建议**：两者共存，分别管理
