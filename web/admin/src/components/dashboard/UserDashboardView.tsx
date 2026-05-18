import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { SummaryMetricCard } from './SummaryMetricCard'
import { DailyRequestsChart, ModelDistributionChart } from '../charts'
import { getUserDashboard, getUserUsage } from '../../lib/api/dashboard'
import type { UserDashboardData } from '../../types/dashboard'

export function UserDashboardView() {
  const { t } = useTranslation()
  const dashboardQuery = useQuery({
    queryKey: ['user-dashboard'],
    queryFn: getUserDashboard,
    refetchInterval: 30_000,
  })

  const usageQuery = useQuery({
    queryKey: ['user-usage', 7],
    queryFn: () => getUserUsage(7),
    refetchInterval: 30_000,
  })

  if (dashboardQuery.isLoading) {
    return <div className="event-state">{t('dashboard.userLoading')}</div>
  }

  if (dashboardQuery.error) {
    return <div className="config-error" role="alert">{t('dashboard.userPanelLoadError')}</div>
  }

  const data = dashboardQuery.data
  if (!data) return null

  const summary = data.summary
  const usageData = usageQuery.data?.data?.map(d => ({
    date: d.date,
    requests: d.requests,
    tokens: d.total_tokens,
    errors: 0,
  }))

  const modelData = data.model_distribution?.map(m => ({
    name: m.key,
    value: m.requests,
  }))

  return (
    <div>
      <div className="summary-card-grid">
        <SummaryMetricCard label={t('dashboard.totalRequests')} value={summary.requests} />
        <SummaryMetricCard label={t('dashboard.totalTokens')} value={summary.total_tokens} />
        <SummaryMetricCard label="Prompt Tokens" value={summary.prompt_tokens} />
        <SummaryMetricCard label="Completion Tokens" value={summary.completion_tokens} />
        <SummaryMetricCard label={t('dashboard.estimatedCost')} value={summary.estimated_cost.toFixed(4)} />
        <SummaryMetricCard label={t('dashboard.avgLatency')} value={Number(summary.avg_latency_ms.toFixed(1))} />
        <SummaryMetricCard label={t('dashboard.cacheHitRate')} value={`${(summary.cache_hit_rate * 100).toFixed(1)}%`} />
        <SummaryMetricCard label={t('dashboard.errorRate')} value={`${(summary.provider_error_rate * 100).toFixed(1)}%`} />
      </div>

      {data.recent_api_keys && data.recent_api_keys.length > 0 && (
        <div className="page-surface" style={{ marginTop: '1rem' }}>
          <h3 style={{ marginBottom: '0.75rem', fontSize: '1rem', fontWeight: 600 }}>{t('dashboard.myApiKeys')}</h3>
          <table className="data-table">
            <thead>
              <tr>
                <th>{t('dashboard.keyName')}</th>
                <th>{t('dashboard.keyPrefix')}</th>
                <th>{t('dashboard.keyStatus')}</th>
                <th>{t('dashboard.keyLastUsed')}</th>
                <th>{t('dashboard.keyCreatedAt')}</th>
              </tr>
            </thead>
            <tbody>
              {data.recent_api_keys.map(k => (
                <tr key={k.id}>
                  <td>{k.name}</td>
                  <td><code>{k.key_prefix}...</code></td>
                  <td>{k.status}</td>
                  <td>{k.last_used_at ? new Date(k.last_used_at).toLocaleString() : t('dashboard.neverUsed')}</td>
                  <td>{new Date(k.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="page-surface" style={{ marginTop: '1rem' }}>
        <h3 style={{ marginBottom: '0.75rem', fontSize: '1rem', fontWeight: 600 }}>调用趋势</h3>
        <DailyRequestsChart data={usageData} />
      </div>

      {modelData && modelData.length > 0 && (
        <div className="page-surface" style={{ marginTop: '1rem' }}>
          <h3 style={{ marginBottom: '0.75rem', fontSize: '1rem', fontWeight: 600 }}>模型分布</h3>
          <ModelDistributionChart data={modelData} />
        </div>
      )}
    </div>
  )
}
