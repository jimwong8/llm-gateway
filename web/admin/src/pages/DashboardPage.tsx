import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { apiRequest } from '../lib/http'
import type { BillingSummary } from '../types/observability'

type AdminHealth = {
  service?: string
  admin_auth?: string
  time?: string
}

function formatPercent(value: number | undefined) {
  return `${((value ?? 0) * 100).toFixed(1)}%`
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

  const loading = healthQuery.isLoading || summaryQuery.isLoading
  const hasError = healthQuery.error || summaryQuery.error

  return (
    <AppShell
      title="Dashboard"
      description="聚合展示服务状态、请求量、缓存命中率与 Provider 错误率，作为控制台首页。"
    >
      <div className="events-page">
        {loading ? <div className="event-state">正在加载首页概览…</div> : null}
        {hasError ? <div className="config-error">首页概览加载失败，请检查后端接口状态。</div> : null}

        {!loading && !hasError ? (
          <div className="summary-card-grid">
            <section className="summary-card">
              <span>Service</span>
              <strong>{healthQuery.data?.service ?? '—'}</strong>
            </section>
            <section className="summary-card">
              <span>Admin Auth</span>
              <strong>{healthQuery.data?.admin_auth ?? '—'}</strong>
            </section>
            <section className="summary-card">
              <span>Requests</span>
              <strong>{summaryQuery.data?.requests ?? 0}</strong>
            </section>
            <section className="summary-card">
              <span>Cache Hit Rate</span>
              <strong>{formatPercent(summaryQuery.data?.cache_hit_rate)}</strong>
            </section>
            <section className="summary-card">
              <span>Provider Error Rate</span>
              <strong>{formatPercent(summaryQuery.data?.provider_error_rate)}</strong>
            </section>
            <section className="summary-card">
              <span>Total Tokens</span>
              <strong>{summaryQuery.data?.total_tokens ?? 0}</strong>
            </section>
          </div>
        ) : null}
      </div>
    </AppShell>
  )
}
