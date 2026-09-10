# LLM Gateway 主页重设计（B：极客终端面板风格）Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前首页重构为面向 AI Gateway 运维场景的极客终端风格首页，突出全局健康、渠道调度、限流成本、安全风控与快捷处置能力。

**Architecture:** 保持现有 `AppShell + DashboardPage + dashboard components + charts API` 架构不变，优先复用现有 `/admin/health`、`/admin/dashboard`、`/admin/observability/summary` 和 `charts/*` 数据接口完成 P0 首页重构。新增一组聚合型前端视图组件组织信息密度与交互联动，后续再增补后端聚合接口支持路由链路流、熔断历史和安全事件流。

**Tech Stack:** React 18, TypeScript, Vite, TanStack Query, Recharts, Zustand, Sonner, CSS/Tailwind variables, existing admin HTTP APIs.

---

## File Structure

### Existing files to modify
- `web/admin/src/pages/DashboardPage.tsx`
  - 首页主容器与数据组合入口；重构为新的首页编排层。
- `web/admin/src/components/dashboard/DashboardAdminOverviewSection.tsx`
  - 改造成北极星状态带入口。
- `web/admin/src/components/dashboard/DashboardSessionOpsSection.tsx`
  - 复用并重构为“运维概览 / 最近操作 / 快捷动作”区块。
- `web/admin/src/components/dashboard/SummaryMetricCard.tsx`
  - 重写为终端风格高密度指标卡。
- `web/admin/src/lib/api/dashboard.ts`
  - 补充首页所需聚合接口调用封装。
- `web/admin/src/types/dashboard.ts`
  - 增补首页新视图所需类型。
- `web/admin/src/styles/index.css`
  - 增加首页专用终端风格样式、状态色、Drawer、矩阵、事件流布局。
- `web/admin/src/components/layout/AppShell.tsx`
  - 只在需要时调整 header / quick nav / content 容器结构，尽量少改。

### New frontend files
- `web/admin/src/components/dashboard/HomeTopStatusStrip.tsx`
  - 顶部北极星状态带组件。
- `web/admin/src/components/dashboard/ChannelHealthMatrix.tsx`
  - 渠道健康矩阵组件（基于现有 channels 数据）。
- `web/admin/src/components/dashboard/CostQuotaPanel.tsx`
  - 成本 / 配额 / 限流控制区。
- `web/admin/src/components/dashboard/SecurityRadarPanel.tsx`
  - 安全风控与审计雷达（P0 先做占位聚合版）。
- `web/admin/src/components/dashboard/OperationsActionPanel.tsx`
  - 运维快捷动作区。
- `web/admin/src/components/dashboard/DetailDrawer.tsx`
  - 首页统一右侧 Drawer，下钻渠道、请求、告警详情。
- `web/admin/src/components/dashboard/RouteTracePanel.tsx`
  - 路由链路流组件（P0 先用 mock/placeholder + 现有 summary 拼接；P1 再接后端聚合接口）。
- `web/admin/src/components/dashboard/dashboard-home.types.ts`
  - 首页内部视图模型类型定义。
- `web/admin/src/components/dashboard/dashboard-home.mappers.ts`
  - 将现有 API 数据映射成首页视图模型。
- `web/admin/src/components/dashboard/dashboard-home.constants.ts`
  - 状态色、告警优先级、按钮配置、模块配置。

### New backend files (P1 readiness only, not P0 unless needed)
- `internal/httpserver/dashboard_runtime_handler.go`
  - 提供 recent-routes / circuit-status / rate-limit-summary / security-events 聚合端点。
- `internal/httpserver/dashboard_runtime_handler_test.go`
  - 上述聚合端点测试。

### Tests
- `web/admin/src/pages/DashboardPage.test.tsx`
  - 首页渲染、状态带、模块布局、错误/加载态。
- `web/admin/src/components/dashboard/*.test.tsx`
  - 关键组件交互测试（矩阵、Drawer、快捷动作、图表切换）。
- `internal/httpserver/dashboard_runtime_handler_test.go`
  - 如进入 P1 后端聚合接口实现则补充测试。

---

## Chunk 1: P0 首页结构重构（仅复用现有接口）

### Task 1: 建立首页视图模型与常量层

**Files:**
- Create: `web/admin/src/components/dashboard/dashboard-home.types.ts`
- Create: `web/admin/src/components/dashboard/dashboard-home.constants.ts`
- Create: `web/admin/src/components/dashboard/dashboard-home.mappers.ts`

- [ ] **Step 1: 写类型定义文件**

```ts
// dashboard-home.types.ts
export type HealthLevel = 'healthy' | 'degraded' | 'critical' | 'disabled'

export type TopMetric = {
  id: 'rps' | 'tokens_per_sec' | 'ttff' | 'p95' | 'healthy_channels' | 'open_circuit' | 'today_cost' | 'alert_count'
  label: string
  value: string
  trend?: string
  level: HealthLevel
}

export type ChannelNode = {
  id: string
  name: string
  provider: string
  status: HealthLevel
  latency: string
  errorRate: string
  weight: number
  requestCount: number
  note?: string
}

export type CostQuotaItem = {
  id: string
  label: string
  used: number
  limit: number
  percent: number
  level: HealthLevel
}

export type SecurityEventItem = {
  id: string
  level: 'info' | 'warning' | 'critical'
  title: string
  summary: string
  timestamp: string
  source?: string
}

export type DrawerPayload =
  | { kind: 'channel'; channelId: string }
  | { kind: 'security-event'; eventId: string }
  | { kind: 'cost-item'; itemId: string }
  | null
```

- [ ] **Step 2: 写常量映射**

```ts
// dashboard-home.constants.ts
export const METRIC_ORDER = [
  'rps',
  'tokens_per_sec',
  'ttff',
  'p95',
  'healthy_channels',
  'open_circuit',
  'today_cost',
  'alert_count',
] as const

export const LEVEL_TO_CLASS = {
  healthy: 'is-healthy',
  degraded: 'is-degraded',
  critical: 'is-critical',
  disabled: 'is-disabled',
} as const

export const DASHBOARD_REFRESH = {
  fast: 5000,
  medium: 15000,
  slow: 30000,
} as const
```

- [ ] **Step 3: 写数据映射器**

```ts
// dashboard-home.mappers.ts
import type { AdminHealth, AdminSummary } from '../../types/dashboard'
import type { Channel } from '../../types/channel'
import type { TopMetric, ChannelNode } from './dashboard-home.types'

export function mapTopMetrics(
  health: AdminHealth | undefined,
  summary: AdminSummary | undefined,
  channels: Channel[]
): TopMetric[] {
  const healthyChannels = channels.filter((c) => c.status === 'active').length
  const openCircuit = channels.filter((c) => c.status === 'error').length

  return [
    { id: 'rps', label: 'RPS', value: String(summary?.requests ?? 0), level: 'healthy' },
    { id: 'tokens_per_sec', label: 'TOKEN/s', value: String(summary?.total_tokens ?? 0), level: 'healthy' },
    { id: 'ttff', label: 'TTFF', value: `${summary?.avg_latency_ms ?? 0}ms`, level: (summary?.avg_latency_ms ?? 0) > 1500 ? 'critical' : 'healthy' },
    { id: 'p95', label: 'P95', value: `${summary?.avg_latency_ms ?? 0}ms`, level: 'degraded' },
    { id: 'healthy_channels', label: 'HEALTHY CH', value: `${healthyChannels}/${channels.length}`, level: healthyChannels === channels.length ? 'healthy' : 'degraded' },
    { id: 'open_circuit', label: 'OPEN CIRCUIT', value: String(openCircuit), level: openCircuit > 0 ? 'critical' : 'healthy' },
    { id: 'today_cost', label: 'TODAY COST', value: `$${(summary?.estimated_cost ?? 0).toFixed(2)}`, level: 'healthy' },
    { id: 'alert_count', label: 'ALERTS', value: String(health?.compensation_stats?.total ?? 0), level: (health?.compensation_stats?.total ?? 0) > 0 ? 'warning' : 'healthy' },
  ]
}

export function mapChannelNodes(channels: Channel[] = []): ChannelNode[] {
  return channels.map((c) => ({
    id: c.id,
    name: c.name,
    provider: c.provider,
    status: c.status === 'active' ? 'healthy' : c.status === 'inactive' ? 'disabled' : 'critical',
    latency: c.latency_ms ? `${c.latency_ms}ms` : '--',
    errorRate: c.status === 'error' ? '100%' : '0%',
    weight: c.weight,
    requestCount: c.total_requests ?? 0,
    note: c.notes,
  }))
}
```

- [ ] **Step 4: 运行类型检查**

Run: `cd web/admin && npx tsc --noEmit`
Expected: no type errors from new files

- [ ] **Step 5: Commit**

```bash
git add web/admin/src/components/dashboard/dashboard-home.types.ts \
  web/admin/src/components/dashboard/dashboard-home.constants.ts \
  web/admin/src/components/dashboard/dashboard-home.mappers.ts
git commit -m "feat(dashboard): add home dashboard view models"
```

---

### Task 2: 重写顶部北极星状态带与指标卡

**Files:**
- Modify: `web/admin/src/components/dashboard/SummaryMetricCard.tsx`
- Create: `web/admin/src/components/dashboard/HomeTopStatusStrip.tsx`
- Modify: `web/admin/src/components/dashboard/DashboardAdminOverviewSection.tsx`
- Test: `web/admin/src/components/dashboard/HomeTopStatusStrip.test.tsx`

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from '@testing-library/react'
import { HomeTopStatusStrip } from './HomeTopStatusStrip'

it('renders top metrics in order', () => {
  render(
    <HomeTopStatusStrip
      metrics={[
        { id: 'rps', label: 'RPS', value: '1284', level: 'healthy' },
        { id: 'ttff', label: 'TTFF', value: '842ms', level: 'degraded' },
      ]}
      onClickMetric={() => {}}
    />
  )

  expect(screen.getByText('RPS')).toBeInTheDocument()
  expect(screen.getByText('1284')).toBeInTheDocument()
  expect(screen.getByText('TTFF')).toBeInTheDocument()
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web/admin && npx vitest run src/components/dashboard/HomeTopStatusStrip.test.tsx`
Expected: FAIL because component doesn't exist

- [ ] **Step 3: 实现 HomeTopStatusStrip**

```tsx
// HomeTopStatusStrip.tsx
import type { TopMetric } from './dashboard-home.types'
import { SummaryMetricCard } from './SummaryMetricCard'

export function HomeTopStatusStrip({
  metrics,
  onClickMetric,
}: {
  metrics: TopMetric[]
  onClickMetric: (metric: TopMetric) => void
}) {
  return (
    <section className="gateway-top-strip" aria-label="全局北极星指标">
      {metrics.map((metric) => (
        <button
          key={metric.id}
          type="button"
          className={`gateway-top-strip__item ${metric.level}`}
          onClick={() => onClickMetric(metric)}
        >
          <SummaryMetricCard label={metric.label} value={metric.value} />
        </button>
      ))}
    </section>
  )
}
```

- [ ] **Step 4: 重写 SummaryMetricCard 为终端指标卡**

```tsx
export function SummaryMetricCard({ label, value }: SummaryMetricCardProps) {
  return (
    <section className="summary-metric-card">
      <span className="summary-metric-card__label">{label}</span>
      <strong className="summary-metric-card__value">{value}</strong>
    </section>
  )
}
```

- [ ] **Step 5: 改造 DashboardAdminOverviewSection**

```tsx
import { HomeTopStatusStrip } from './HomeTopStatusStrip'
import { mapTopMetrics } from './dashboard-home.mappers'

export function DashboardAdminOverviewSection(props: DashboardAdminOverviewSectionProps) {
  const metrics = mapTopMetrics(props.health, props.summary, props.channels)
  return <HomeTopStatusStrip metrics={metrics} onClickMetric={() => {}} />
}
```

- [ ] **Step 6: 运行测试**

Run: `cd web/admin && npx vitest run src/components/dashboard/HomeTopStatusStrip.test.tsx`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/admin/src/components/dashboard/SummaryMetricCard.tsx \
  web/admin/src/components/dashboard/HomeTopStatusStrip.tsx \
  web/admin/src/components/dashboard/DashboardAdminOverviewSection.tsx \
  web/admin/src/components/dashboard/HomeTopStatusStrip.test.tsx
git commit -m "feat(dashboard): redesign top status strip"
```

---

### Task 3: 实现渠道健康矩阵

**Files:**
- Create: `web/admin/src/components/dashboard/ChannelHealthMatrix.tsx`
- Modify: `web/admin/src/pages/DashboardPage.tsx`
- Test: `web/admin/src/components/dashboard/ChannelHealthMatrix.test.tsx`

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from '@testing-library/react'
import { ChannelHealthMatrix } from './ChannelHealthMatrix'

it('renders channel nodes and status', () => {
  render(
    <ChannelHealthMatrix
      channels={[
        {
          id: 'ch-1',
          name: 'OPENAI-A',
          provider: 'openai',
          status: 'healthy',
          latency: '312ms',
          errorRate: '0.4%',
          weight: 10,
          requestCount: 1200,
        },
      ]}
      onSelect={() => {}}
    />
  )

  expect(screen.getByText('OPENAI-A')).toBeInTheDocument()
  expect(screen.getByText('openai')).toBeInTheDocument()
  expect(screen.getByText('312ms')).toBeInTheDocument()
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web/admin && npx vitest run src/components/dashboard/ChannelHealthMatrix.test.tsx`
Expected: FAIL because component doesn't exist

- [ ] **Step 3: 实现矩阵组件**

```tsx
import type { ChannelNode } from './dashboard-home.types'

export function ChannelHealthMatrix({
  channels,
  onSelect,
}: {
  channels: ChannelNode[]
  onSelect: (node: ChannelNode) => void
}) {
  return (
    <section className="channel-health-matrix" aria-label="渠道健康矩阵">
      <header className="panel-header">
        <h2>[ CHANNEL HEALTH ]</h2>
        <span>{channels.length} channels</span>
      </header>
      <div className="channel-health-matrix__grid">
        {channels.map((node) => (
          <button
            key={node.id}
            type="button"
            className={`channel-health-node ${node.status}`}
            onClick={() => onSelect(node)}
          >
            <div className="channel-health-node__header">
              <span className="channel-health-node__dot" />
              <strong>{node.name}</strong>
            </div>
            <div className="channel-health-node__meta">{node.provider}</div>
            <div className="channel-health-node__stats">
              <span>{node.latency}</span>
              <span>{node.errorRate}</span>
              <span>w:{node.weight}</span>
              <span>req:{node.requestCount}</span>
            </div>
          </button>
        ))}
      </div>
    </section>
  )
}
```

- [ ] **Step 4: 接入 DashboardPage**

在 `DashboardPage.tsx` 中新增 channels query：

```tsx
import { listChannels } from '../lib/channels'
import { ChannelHealthMatrix } from '../components/dashboard/ChannelHealthMatrix'
import { mapChannelNodes } from '../components/dashboard/dashboard-home.mappers'

const channelsQuery = useQuery({
  queryKey: ['dashboard-channels-health'],
  queryFn: listChannels,
  refetchInterval: 5000,
})
```

并在图表区前插入矩阵：

```tsx
<ChannelHealthMatrix
  channels={mapChannelNodes(channelsQuery.data ?? [])}
  onSelect={(node) => setDrawerPayload({ kind: 'channel', channelId: node.id })}
/>
```

- [ ] **Step 5: 运行测试**

Run: `cd web/admin && npx vitest run src/components/dashboard/ChannelHealthMatrix.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/admin/src/components/dashboard/ChannelHealthMatrix.tsx \
  web/admin/src/components/dashboard/ChannelHealthMatrix.test.tsx \
  web/admin/src/pages/DashboardPage.tsx
git commit -m "feat(dashboard): add channel health matrix"
```

---

## Chunk 2: P0 Drawer 与成本/风控聚合

### Task 4: 实现统一 DetailDrawer 基础版

**Files:**
- Create: `web/admin/src/components/dashboard/DetailDrawer.tsx`
- Modify: `web/admin/src/pages/DashboardPage.tsx`
- Test: `web/admin/src/components/dashboard/DetailDrawer.test.tsx`

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DetailDrawer } from './DetailDrawer'

it('opens and closes drawer', async () => {
  const onClose = vi.fn()
  render(
    <DetailDrawer open title="OPENAI-A" onClose={onClose}>
      <div>detail body</div>
    </DetailDrawer>
  )

  expect(screen.getByText('OPENAI-A')).toBeInTheDocument()
  expect(screen.getByText('detail body')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: /close/i }))
  expect(onClose).toHaveBeenCalled()
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web/admin && npx vitest run src/components/dashboard/DetailDrawer.test.tsx`
Expected: FAIL because component doesn't exist

- [ ] **Step 3: 实现 Drawer 基础版**

```tsx
export function DetailDrawer({
  open,
  title,
  subtitle,
  children,
  onClose,
}: {
  open: boolean
  title: string
  subtitle?: string
  children: React.ReactNode
  onClose: () => void
}) {
  if (!open) return null
  return (
    <aside className="detail-drawer" aria-label="详情抽屉">
      <header className="detail-drawer__header">
        <div>
          <h2>{title}</h2>
          {subtitle ? <p>{subtitle}</p> : null}
        </div>
        <button type="button" aria-label="close" onClick={onClose}>×</button>
      </header>
      <div className="detail-drawer__body">{children}</div>
    </aside>
  )
}
```

- [ ] **Step 4: 接入 DashboardPage 的 drawer state**

```tsx
const [drawerPayload, setDrawerPayload] = useState<DrawerPayload>(null)

<DetailDrawer
  open={!!drawerPayload}
  title={drawerPayload?.kind === 'channel' ? selectedChannel?.name ?? 'Channel Detail' : 'Detail'}
  subtitle={drawerPayload?.kind === 'channel' ? selectedChannel?.provider : undefined}
  onClose={() => setDrawerPayload(null)}
>
  {drawerPayload?.kind === 'channel' ? <ChannelDetailPanel node={selectedChannel} /> : null}
</DetailDrawer>
```

- [ ] **Step 5: 运行测试**

Run: `cd web/admin && npx vitest run src/components/dashboard/DetailDrawer.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/admin/src/components/dashboard/DetailDrawer.tsx \
  web/admin/src/components/dashboard/DetailDrawer.test.tsx \
  web/admin/src/pages/DashboardPage.tsx
git commit -m "feat(dashboard): add detail drawer"
```

---

### Task 5: 实现成本/配额/限流控制区（基于现有 summary）

**Files:**
- Create: `web/admin/src/components/dashboard/CostQuotaPanel.tsx`
- Modify: `web/admin/src/pages/DashboardPage.tsx`
- Test: `web/admin/src/components/dashboard/CostQuotaPanel.test.tsx`

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from '@testing-library/react'
import { CostQuotaPanel } from './CostQuotaPanel'

it('renders cost and quota summary', () => {
  render(
    <CostQuotaPanel
      todayCost="$184.23"
      monthCost="$3940.20"
      cacheHitRate="24.5%"
      providerErrorRate="1.2%"
      totalTokens="1284000"
    />
  )

  expect(screen.getByText('$184.23')).toBeInTheDocument()
  expect(screen.getByText('$3940.20')).toBeInTheDocument()
  expect(screen.getByText('24.5%')).toBeInTheDocument()
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web/admin && npx vitest run src/components/dashboard/CostQuotaPanel.test.tsx`
Expected: FAIL because component doesn't exist

- [ ] **Step 3: 实现面板**

```tsx
export function CostQuotaPanel(props: {
  todayCost: string
  monthCost: string
  cacheHitRate: string
  providerErrorRate: string
  totalTokens: string
}) {
  return (
    <section className="cost-quota-panel" aria-label="成本与配额控制区">
      <header className="panel-header">
        <h2>[ COST / QUOTA ]</h2>
        <span>budget watch</span>
      </header>
      <div className="cost-quota-panel__grid">
        <article><span>TODAY COST</span><strong>{props.todayCost}</strong></article>
        <article><span>MONTH COST</span><strong>{props.monthCost}</strong></article>
        <article><span>CACHE HIT</span><strong>{props.cacheHitRate}</strong></article>
        <article><span>ERR RATE</span><strong>{props.providerErrorRate}</strong></article>
        <article><span>TOTAL TOKENS</span><strong>{props.totalTokens}</strong></article>
      </div>
    </section>
  )
}
```

- [ ] **Step 4: 接入 DashboardPage**

```tsx
<CostQuotaPanel
  todayCost={`$${(summaryQuery.data?.estimated_cost ?? 0).toFixed(2)}`}
  monthCost={`$${(summaryQuery.data?.estimated_cost ?? 0).toFixed(2)}`}
  cacheHitRate={formatPercent(summaryQuery.data?.cache_hit_rate)}
  providerErrorRate={formatPercent(summaryQuery.data?.provider_error_rate)}
  totalTokens={String(summaryQuery.data?.total_tokens ?? 0)}
/>
```

- [ ] **Step 5: 运行测试**

Run: `cd web/admin && npx vitest run src/components/dashboard/CostQuotaPanel.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/admin/src/components/dashboard/CostQuotaPanel.tsx \
  web/admin/src/components/dashboard/CostQuotaPanel.test.tsx \
  web/admin/src/pages/DashboardPage.tsx
git commit -m "feat(dashboard): add cost quota panel"
```

---

### Task 6: 实现安全风控与运维快捷区（P0 占位版）

**Files:**
- Create: `web/admin/src/components/dashboard/SecurityRadarPanel.tsx`
- Create: `web/admin/src/components/dashboard/OperationsActionPanel.tsx`
- Modify: `web/admin/src/pages/DashboardPage.tsx`
- Test: `web/admin/src/components/dashboard/SecurityRadarPanel.test.tsx`

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from '@testing-library/react'
import { SecurityRadarPanel } from './SecurityRadarPanel'

it('renders empty-state security radar', () => {
  render(<SecurityRadarPanel events={[]} onSelectEvent={() => {}} />)
  expect(screen.getByText('[ SECURITY EVENTS ]')).toBeInTheDocument()
  expect(screen.getByText(/当前没有高风险事件/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web/admin && npx vitest run src/components/dashboard/SecurityRadarPanel.test.tsx`
Expected: FAIL because component doesn't exist

- [ ] **Step 3: 实现安全雷达与快捷操作**

```tsx
export function SecurityRadarPanel({ events, onSelectEvent }: {
  events: SecurityEventItem[]
  onSelectEvent: (event: SecurityEventItem) => void
}) {
  return (
    <section className="security-radar-panel">
      <header className="panel-header">
        <h2>[ SECURITY EVENTS ]</h2>
        <span>{events.length} active</span>
      </header>
      {events.length === 0 ? (
        <div className="security-radar-panel__empty">当前没有高风险事件</div>
      ) : (
        <div className="security-radar-panel__list">
          {events.map((event) => (
            <button key={event.id} type="button" className={`security-event ${event.level}`} onClick={() => onSelectEvent(event)}>
              <span>{event.timestamp}</span>
              <strong>{event.title}</strong>
              <p>{event.summary}</p>
            </button>
          ))}
        </div>
      )}
    </section>
  )
}

export function OperationsActionPanel() {
  return (
    <section className="operations-action-panel">
      <header className="panel-header">
        <h2>[ QUICK ACTIONS ]</h2>
        <span>ops shortcuts</span>
      </header>
      <div className="operations-action-panel__grid">
        <button type="button">刷新全部</button>
        <button type="button">打开在线测试</button>
        <button type="button">导出错误日志</button>
        <button type="button">查看审计导出</button>
      </div>
    </section>
  )
}
```

- [ ] **Step 4: 接入 DashboardPage**

```tsx
<SecurityRadarPanel events={[]} onSelectEvent={() => {}} />
<OperationsActionPanel />
```

- [ ] **Step 5: 运行测试**

Run: `cd web/admin && npx vitest run src/components/dashboard/SecurityRadarPanel.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/admin/src/components/dashboard/SecurityRadarPanel.tsx \
  web/admin/src/components/dashboard/OperationsActionPanel.tsx \
  web/admin/src/components/dashboard/SecurityRadarPanel.test.tsx \
  web/admin/src/pages/DashboardPage.tsx
git commit -m "feat(dashboard): add security radar and quick actions"
```

---

## Chunk 3: Styling, validation, and P1 handoff

### Task 7: 写入终端风格样式系统

**Files:**
- Modify: `web/admin/src/styles/index.css`

- [ ] **Step 1: 在失败前先记录当前视觉基线**

Run: `cd web/admin && npm run build`
Expected: PASS before style changes

- [ ] **Step 2: 添加首页专用样式块**

新增以下样式分组：

```css
/* homepage terminal dashboard */
.gateway-top-strip { ... }
.summary-metric-card { ... }
.channel-health-matrix { ... }
.channel-health-node { ... }
.cost-quota-panel { ... }
.security-radar-panel { ... }
.operations-action-panel { ... }
.detail-drawer { ... }
.panel-header { ... }
```

并遵循：
- 深色基底
- 高对比边框
- 等宽数字字号
- 红黄绿语义色
- 轻量 hover / pulse 动效
- 不引入大面积花哨背景球

- [ ] **Step 3: 运行构建验证**

Run: `cd web/admin && npm run build`
Expected: PASS and generated CSS bundle

- [ ] **Step 4: Commit**

```bash
git add web/admin/src/styles/index.css
git commit -m "style(dashboard): add terminal-style homepage theme"
```

---

### Task 8: 验证首页端到端渲染

**Files:**
- Modify: `web/admin/src/pages/DashboardPage.test.tsx`
- Test: existing dashboard page tests

- [ ] **Step 1: 增加首页关键模块测试**

```tsx
it('renders terminal homepage sections', async () => {
  render(<DashboardPage />)
  expect(await screen.findByText(/RPS/i)).toBeInTheDocument()
  expect(await screen.findByText(/CHANNEL HEALTH/i)).toBeInTheDocument()
  expect(await screen.findByText(/COST \/ QUOTA/i)).toBeInTheDocument()
  expect(await screen.findByText(/SECURITY EVENTS/i)).toBeInTheDocument()
  expect(await screen.findByText(/QUICK ACTIONS/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: 运行测试**

Run: `cd web/admin && npx vitest run src/pages/DashboardPage.test.tsx`
Expected: PASS

- [ ] **Step 3: 运行整体验证**

Run: `cd web/admin && npm run build && npx vitest run src/pages/DashboardPage.test.tsx src/components/dashboard/*.test.tsx`
Expected: PASS all

- [ ] **Step 4: Commit**

```bash
git add web/admin/src/pages/DashboardPage.test.tsx
git commit -m "test(dashboard): cover terminal homepage layout"
```

---

### Task 9: P1 后端聚合接口实施计划占位

**Files:**
- Modify: `docs/plans/2026-05-20-homepage-b-terminal-redesign-design.md`

- [ ] **Step 1: 在设计文档末尾追加 P1 API backlog**

```md
## P1 API backlog
- /admin/runtime/recent-routes
- /admin/runtime/route-trace/{requestId}
- /admin/channels/circuit-status
- /admin/channels/circuit-history
- /admin/rate-limit/summary
- /admin/rate-limit/hot-keys
- /admin/rate-limit/hot-tenants
- /admin/security/events
- /admin/security/summary
- /admin/keys/expiring
- /admin/keys/quota-risk
```

- [ ] **Step 2: Commit**

```bash
git add docs/plans/2026-05-20-homepage-b-terminal-redesign-design.md
git commit -m "docs(dashboard): add p1 API backlog"
```

---

## Validation checklist

After implementation, verify:

- [ ] `cd web/admin && npm run build` passes
- [ ] `cd web/admin && npx vitest run src/pages/DashboardPage.test.tsx src/components/dashboard/*.test.tsx` passes
- [ ] 首页首屏出现：RPS / TTFF / 渠道健康矩阵 / 成本配额 / 安全事件 / 快捷动作
- [ ] 点击渠道节点可打开 Drawer
- [ ] 无渠道数据时矩阵显示 empty state 而不是崩溃
- [ ] 深色终端风格在 1440px 和 768px 下都可读

---

## Commit strategy

Recommended commit cadence:
1. `feat(dashboard): add home dashboard view models`
2. `feat(dashboard): redesign top status strip`
3. `feat(dashboard): add channel health matrix`
4. `feat(dashboard): add detail drawer`
5. `feat(dashboard): add cost quota panel`
6. `feat(dashboard): add security radar and quick actions`
7. `style(dashboard): add terminal-style homepage theme`
8. `test(dashboard): cover terminal homepage layout`
9. `docs(dashboard): add p1 API backlog`

---

Plan complete and saved to `docs/plans/2026-05-20-homepage-b-terminal-redesign-implementation.md`. Ready to execute?
