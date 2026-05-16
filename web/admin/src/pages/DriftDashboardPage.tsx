import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { listGovernanceDrifts } from '../lib/drifts'

function formatDate(value?: string) {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

export function DriftDashboardPage() {
  const driftsQuery = useQuery({
    queryKey: ['governance-drifts'],
    queryFn: listGovernanceDrifts,
  })

  const drifts = useMemo(() => driftsQuery.data?.data ?? [], [driftsQuery.data])

  const metrics = useMemo(() => {
    const total = drifts.length
    const detected = drifts.filter((item) => item.status === 'detected').length
    const accepted = drifts.filter((item) => item.status === 'accepted').length
    const resolved = drifts.filter((item) => item.status === 'resolved').length
    return { total, detected, accepted, resolved }
  }, [drifts])

  return (
    <AppShell
      title="漂移仪表盘"
      description="查看治理漂移列表，聚焦当前与推荐模型的偏移状态及检测时间。"
    >
      <div className="events-page">
        {driftsQuery.isLoading ? <div className="event-state">正在加载 drift 列表…</div> : null}
        {driftsQuery.error ? <div className="config-error">drift 列表加载失败，请检查 governance 接口状态。</div> : null}

        {!driftsQuery.isLoading && !driftsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>漂移总数</span>
                <strong>{metrics.total}</strong>
              </section>
              <section className="summary-card">
                <span>已检测</span>
                <strong>{metrics.detected}</strong>
              </section>
              <section className="summary-card">
                <span>已接受</span>
                <strong>{metrics.accepted}</strong>
              </section>
              <section className="summary-card">
                <span>已解决</span>
                <strong>{metrics.resolved}</strong>
              </section>
            </div>

            <div className="event-table">
              <table>
                <thead>
                  <tr>
                    <th>漂移 ID</th>
                    <th>环境</th>
                    <th>智能体</th>
                    <th>当前模型</th>
                    <th>推荐模型</th>
                    <th>状态</th>
                    <th>检测时间</th>
                  </tr>
                </thead>
                <tbody>
                  {drifts.map((row) => (
                    <tr key={row.id}>
                      <td>{row.id}</td>
                      <td>{row.environment || '—'}</td>
                      <td>{row.agent_id || '—'}</td>
                      <td>{row.active_model || '—'}</td>
                      <td>{row.recommended_model || '—'}</td>
                      <td>
                        <span className={`status-pill ${row.status}`}>{row.status}</span>
                      </td>
                      <td>{formatDate(row.detected_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {drifts.length === 0 ? <div className="config-table__state">当前没有漂移数据。</div> : null}
            </div>
          </>
        ) : null}
      </div>
    </AppShell>
  )
}
