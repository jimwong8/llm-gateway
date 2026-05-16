import type { RuntimeEvent, SummaryResponse } from '../../types/runtime'

type RuntimeTableProps = {
  data: RuntimeEvent[] | SummaryResponse | undefined
  loading: boolean
}

export function RuntimeTable({ data, loading }: RuntimeTableProps) {
  if (loading) {
    return <div className="event-state">正在加载运行时事件…</div>
  }

  if (!data) {
    return <div className="event-state">暂无运行时数据。</div>
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
    return <div className="event-state">当前筛选条件下没有运行时事件。</div>
  }

  return (
    <div className="event-table">
      <table>
        <thead>
          <tr>
            <th>版本</th>
            <th>环境</th>
            <th>模块</th>
            <th>租户</th>
            <th>来源</th>
          </tr>
        </thead>
        <tbody>
          {data.map((event, index) => (
            <tr key={`${event.version.version}-${index}`}>
              <td>{event.version.version}</td>
              <td>{event.version.environment}</td>
              <td>{event.version.module}</td>
              <td>{event.version.tenant_id}</td>
              <td>
                {event.version.source_environment && event.version.source_version
                  ? `${event.version.source_environment} / ${event.version.source_version}`
                  : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
