import type { AuditEvent, SummaryResponse } from '../../types/runtime'

type AuditTableProps = {
  data: AuditEvent[] | SummaryResponse | undefined
  loading: boolean
}

export function AuditTable({ data, loading }: AuditTableProps) {
  if (loading) {
    return <div className="event-state">正在加载审计事件…</div>
  }

  if (!data) {
    return <div className="event-state">暂无审计数据。</div>
  }

  if (!Array.isArray(data)) {
    return (
      <div className="summary-card-grid">
        <div className="summary-card">
          <span>总数</span>
          <strong>{data.total}</strong>
        </div>
        <div className="summary-card">
          <span>按类型</span>
          <strong>{Object.keys(data.by_type).join(', ') || '—'}</strong>
        </div>
        <div className="summary-card">
          <span>按环境</span>
          <strong>{Object.keys(data.by_environment).join(', ') || '—'}</strong>
        </div>
      </div>
    )
  }

  if (data.length === 0) {
    return <div className="event-state">当前筛选条件下没有审计事件。</div>
  }

  return (
    <div className="event-table">
      <table>
        <thead>
          <tr>
            <th>类型</th>
            <th>环境</th>
            <th>租户</th>
            <th>版本</th>
            <th>操作人</th>
          </tr>
        </thead>
        <tbody>
          {data.map((event, index) => (
            <tr key={`${event.version_id}-${index}`}>
              <td>{event.type}</td>
              <td>{event.environment}</td>
              <td>{event.tenant_id}</td>
              <td>{event.version_id}</td>
              <td>{event.actor || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
