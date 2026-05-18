import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { SummaryMetricCard } from './SummaryMetricCard'
import { DailyRequestsChart, ModelDistributionChart } from '../charts'
import { getUserDashboard, getUserUsage, getUserUsageLogs, getUserCostTrend } from '../../lib/api/dashboard'
import type { UserDashboardData, UserUsageLog, CostTrendPoint } from '../../types/dashboard'

export function UserDashboardView() {
  const { t } = useTranslation()
  const [logSearch, setLogSearch] = useState('')
  const [logPage, setLogPage] = useState(0)
  const logLimit = 20

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

  const logsQuery = useQuery({
    queryKey: ['user-usage-logs', logPage],
    queryFn: () => getUserUsageLogs(logLimit, logPage * logLimit),
    refetchInterval: 30_000,
  })

  const costTrendQuery = useQuery({
    queryKey: ['user-cost-trend', 30],
    queryFn: () => getUserCostTrend(30),
    refetchInterval: 60_000,
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

      <div className="page-surface" style={{ marginTop: '1rem' }}>
        <h3 style={{ marginBottom: '0.75rem', fontSize: '1rem', fontWeight: 600 }}>成本趋势</h3>
        {costTrendQuery.isLoading ? (
          <div className="event-state">加载中...</div>
        ) : (
          <CostTrendSummary data={costTrendQuery.data?.data} />
        )}
      </div>

      <div className="page-surface" style={{ marginTop: '1rem' }}>
        <h3 style={{ marginBottom: '0.75rem', fontSize: '1rem', fontWeight: 600 }}>调用日志</h3>
        <div style={{ marginBottom: '1rem' }}>
          <input
            type="text"
            placeholder="搜索 provider 或 model..."
            value={logSearch}
            onChange={e => { setLogSearch(e.target.value); setLogPage(0) }}
            style={{ width: '100%', padding: '0.5rem', borderRadius: '6px', border: '1px solid #d1d5db' }}
          />
        </div>
        {logsQuery.isLoading ? (
          <div className="event-state">加载中...</div>
        ) : (
          <>
            <UsageLogTable
              logs={(logsQuery.data?.data || []).filter((l: UserUsageLog) =>
                !logSearch || l.provider.includes(logSearch) || l.model.includes(logSearch)
              )}
            />
            <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '1rem' }}>
              <button type="button" onClick={() => setLogPage(p => Math.max(0, p - 1))} disabled={logPage === 0}>
                上一页
              </button>
              <span>第 {logPage + 1} 页</span>
              <button type="button" onClick={() => setLogPage(p => p + 1)} disabled={(logsQuery.data?.data || []).length < logLimit}>
                下一页
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

function CostTrendSummary({ data }: { data?: CostTrendPoint[] }) {
  if (!data || data.length === 0) {
    return <div className="event-state">暂无数据</div>
  }
  const totalCost = data.reduce((s, d) => s + d.cost_cents, 0)
  const totalTokens = data.reduce((s, d) => s + d.tokens, 0)
  const totalRequests = data.reduce((s, d) => s + d.requests, 0)
  return (
    <div className="summary-card-grid" style={{ gridTemplateColumns: 'repeat(3, 1fr)' }}>
      <div className="summary-card">
        <span>总费用</span>
        <strong>¥{(totalCost / 100).toFixed(2)}</strong>
      </div>
      <div className="summary-card">
        <span>总 Token</span>
        <strong>{totalTokens.toLocaleString()}</strong>
      </div>
      <div className="summary-card">
        <span>总请求</span>
        <strong>{totalRequests}</strong>
      </div>
    </div>
  )
}

function UsageLogTable({ logs }: { logs: UserUsageLog[] }) {
  if (logs.length === 0) {
    return <div className="event-state">暂无调用记录</div>
  }
  return (
    <table className="data-table">
      <thead>
        <tr>
          <th>时间</th>
          <th>Provider</th>
          <th>Model</th>
          <th>Tokens</th>
          <th>费用</th>
          <th>状态</th>
          <th>耗时</th>
        </tr>
      </thead>
      <tbody>
        {logs.map(l => (
          <tr key={l.id}>
            <td>{new Date(l.created_at).toLocaleString()}</td>
            <td>{l.provider}</td>
            <td>{l.model}</td>
            <td>{l.total_tokens.toLocaleString()}</td>
            <td>¥{(l.cost_cents / 100).toFixed(4)}</td>
            <td>{l.status_code}</td>
            <td>{l.duration_ms}ms</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
