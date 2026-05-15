import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { DashboardAdminOverviewSection } from '../components/dashboard/DashboardAdminOverviewSection'
import { DashboardSessionOpsSection } from '../components/dashboard/DashboardSessionOpsSection'
import { apiRequest } from '../lib/http'
import type { BillingSummary } from '../types/observability'
import type { SessionAdminDashboard } from '../types/sessionDashboard'
import { formatPercent } from '../lib/format'

type AdminHealth = {
  service?: string
  admin_auth?: string
  time?: string
}

export function DashboardPage() {
  const healthQuery = useQuery({
    queryKey: ['dashboard-health'],
    queryFn: () => apiRequest<AdminHealth>('/admin/health'),
  })

  const summaryQuery = useQuery({
    queryKey: ['dashboard-observability-summary'],
    queryFn: () => apiRequest<BillingSummary>('/admin/observability/summary'),
  })

  const sessionQuery = useQuery({
    queryKey: ['dashboard-session'],
    queryFn: () => apiRequest<SessionAdminDashboard>('/admin/dashboard'),
    retry: false,
  })

  const loading = healthQuery.isLoading || summaryQuery.isLoading
  const hasError = healthQuery.error || summaryQuery.error

  return (
    <AppShell
      title="Dashboard"
      description="聚合展示服务状态、请求量、缓存命中率与 Provider 错误率，作为控制台首页。"
    >
      <div className="events-page">
        {loading ? <div className="event-state">正在加载首页概览…</div> : null}
        {hasError ? <div className="config-error" role="alert">首页概览加载失败，请检查后端接口状态。</div> : null}

        {!loading && !hasError ? (
          <DashboardAdminOverviewSection
            service={healthQuery.data?.service ?? '—'}
            adminAuth={healthQuery.data?.admin_auth ?? '—'}
            requests={summaryQuery.data?.requests ?? 0}
            cacheHitRate={formatPercent(summaryQuery.data?.cache_hit_rate)}
            providerErrorRate={formatPercent(summaryQuery.data?.provider_error_rate)}
            totalTokens={summaryQuery.data?.total_tokens ?? 0}
          />
        ) : null}

        <DashboardSessionOpsSection
          loading={sessionQuery.isLoading}
          hasError={!!sessionQuery.error}
          data={sessionQuery.data}
        />
      </div>
    </AppShell>
  )
}
