import { HomeTopStatusStrip } from './HomeTopStatusStrip'
import { mapTopMetrics } from './dashboard-home.mappers'
import { SecurityRadarPanel } from './SecurityRadarPanel'
import { OperationsActionPanel } from './OperationsActionPanel'
import { CostQuotaPanel } from './CostQuotaPanel'
import { apiRequest } from '../../lib/http'
import { getLatencyTrend } from '../../lib/api/dashboard'
import { useQuery } from '@tanstack/react-query'
import type { AdminHealth, AdminSummary, LatencyPoint } from '../../types/dashboard'
import type { Channel } from '../../types/channel'
import type { SecurityEventItem } from './dashboard-home.types'
import type { SessionAdminDashboard } from '../../types/sessionDashboard'

type DashboardAdminOverviewSectionProps = {
  health: AdminHealth | undefined
  summary: AdminSummary | undefined
  channels: Channel[]
  sessionData?: SessionAdminDashboard
}

// 后端 /admin/runtime-events 返回 AuditEvent[]（字段不同），
// 这里做最小映射；映射失败的行直接丢弃。
type RawRuntimeEvent = {
  id?: string | number
  timestamp?: string
  level?: string
  severity?: string
  title?: string
  message?: string
  summary?: string
  description?: string
  source?: string
  actor?: string
  user_id?: string
  ip?: string
  target?: string
  category?: string
  type?: string
}

function mapRuntimeEventsToSecurityEvents(raw: unknown): SecurityEventItem[] {
  if (!Array.isArray(raw)) return []
  const out: SecurityEventItem[] = []
  for (const item of raw) {
    const ev = item as RawRuntimeEvent
    if (!ev || typeof ev !== 'object') continue
    const levelRaw = (ev.level ?? ev.severity ?? '').toLowerCase()
    let level: SecurityEventItem['level'] = 'info'
    if (levelRaw.includes('crit')) level = 'critical'
    else if (levelRaw.includes('warn') || levelRaw.includes('error')) level = 'warning'
    else if (levelRaw.includes('info')) level = 'info'

    const title = ev.title ?? ev.category ?? ev.type ?? '运行时事件'
    const summary = ev.summary ?? ev.message ?? ev.description ?? ev.target ?? ''
    if (!title || !summary) continue

    out.push({
      id: String(ev.id ?? `${ev.timestamp}-${title}`),
      level,
      title,
      summary: String(summary).slice(0, 200),
      timestamp: ev.timestamp ?? '',
      source: ev.source ?? ev.actor ?? ev.user_id ?? ev.ip,
    })
  }
  return out.slice(0, 20)
}

export function DashboardAdminOverviewSection({
  health,
  summary,
  channels,
  sessionData,
}: DashboardAdminOverviewSectionProps) {
  // Fetch latency trend for real P95 in KPI strip
  const latencyQuery = useQuery({
    queryKey: ['dashboard-kpi-latency'],
    queryFn: () => getLatencyTrend(7),
    refetchInterval: 30_000,
    staleTime: 15_000,
    retry: false,
  })
  const latencyData: LatencyPoint[] | undefined = latencyQuery.data?.data

  // Count alerts from session data
  const alertCount = Array.isArray(sessionData?.alerts) ? sessionData.alerts.length : 0

  const metrics = mapTopMetrics(health, summary, channels, latencyData, alertCount)

  // 安全事件从后端 /admin/runtime-events 拉取；失败时显示错误态而非 mock。
  const securityEventsQuery = useQuery({
    queryKey: ['dashboard-security-events'],
    queryFn: async () => {
      const raw = await apiRequest<unknown>('/admin/runtime-events', undefined, {
        auth: 'admin',
        timeout: 15000,
      })
      return mapRuntimeEventsToSecurityEvents(raw)
    },
    refetchInterval: 60_000,
    staleTime: 30_000,
    retry: false,
  })

  const securityEvents = securityEventsQuery.data ?? []
  const securityLoading = securityEventsQuery.isLoading
  const securityError = securityEventsQuery.isError ? securityEventsQuery.error : null

  // CostQuotaPanel: use estimated_cost from billing summary as today cost
  const todayCostDisplay =
    summary?.estimated_cost != null
      ? `$${summary.estimated_cost.toFixed(2)}`
      : '--'
  const monthCostDisplay = '--'

  return (
    <section className="admin-overview-section">
      <HomeTopStatusStrip
        metrics={metrics}
        onClickMetric={(metric) => {
          console.log('Metric clicked:', metric)
        }}
      />
      <CostQuotaPanel
        todayCost={todayCostDisplay}
        monthCost={monthCostDisplay}
        cacheHitRate={`${(summary?.cache_hit_rate ?? 0).toFixed(1)}%`}
        providerErrorRate={`${(summary?.provider_error_rate ?? 0).toFixed(1)}%`}
        totalTokens={String(summary?.total_tokens ?? 0)}
      />
      <SecurityRadarPanel
        events={securityEvents}
        loading={securityLoading}
        error={securityError}
        onSelectEvent={(event) => {
          console.log('Security event selected:', event)
        }}
      />
      <OperationsActionPanel />
    </section>
  )
}
