import { useQuery } from '@tanstack/react-query'
import { SummaryMetricCard } from './SummaryMetricCard'
import { DailyRequestsChart, ModelDistributionChart } from '../charts'
import { getUserDashboard, getUserUsage } from '../../lib/api/dashboard'
import type { UserDashboardData } from '../../types/dashboard'

export function UserDashboardView() {
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
    return <div className="event-state">正在加载用户面板…</div>
  }

  if (dashboardQuery.error) {
    return <div className="config-error" role="alert">用户面板加载失败</div>
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
        <SummaryMetricCard label="总请求数" value={summary.requests} />
        <SummaryMetricCard label="总 Token" value={summary.total_tokens} />
        <SummaryMetricCard label="Prompt Tokens" value={summary.prompt_tokens} />
        <SummaryMetricCard label="Completion Tokens" value={summary.completion_tokens} />
        <SummaryMetricCard label="估算成本" value={summary.estimated_cost.toFixed(4)} />
        <SummaryMetricCard label="平均延迟 (ms)" value={Number(summary.avg_latency_ms.toFixed(1))} />
        <SummaryMetricCard label="缓存命中率" value={`${(summary.cache_hit_rate * 100).toFixed(1)}%`} />
        <SummaryMetricCard label="错误率" value={`${(summary.provider_error_rate * 100).toFixed(1)}%`} />
      </div>

      {data.recent_api_keys && data.recent_api_keys.length > 0 && (
        <div className="page-surface" style={{ marginTop: '1rem' }}>
          <h3 style={{ marginBottom: '0.75rem', fontSize: '1rem', fontWeight: 600 }}>我的 API Keys</h3>
          <table className="data-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>Key 前缀</th>
                <th>状态</th>
                <th>最近使用</th>
                <th>创建时间</th>
              </tr>
            </thead>
            <tbody>
              {data.recent_api_keys.map(k => (
                <tr key={k.id}>
                  <td>{k.name}</td>
                  <td><code>{k.key_prefix}...</code></td>
                  <td>{k.status}</td>
                  <td>{k.last_used_at ? new Date(k.last_used_at).toLocaleString() : '从未使用'}</td>
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
