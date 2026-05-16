import type { ProviderBreakdownRow } from '../../types/observability'

type Props = {
  providers: ProviderBreakdownRow[]
}

export function ObservabilityProvidersSection({ providers }: Props) {
  return (
    <div className="event-table">
      <table>
        <thead>
          <tr>
            <th>Provider</th>
            <th>请求量</th>
            <th>总 Token 数</th>
            <th>错误率</th>
          </tr>
        </thead>
        <tbody>
          {providers.map((row) => (
            <tr key={row.provider}>
              <td>{row.provider}</td>
              <td>{row.requests}</td>
              <td>{row.total_tokens}</td>
              <td>{(row.provider_error_rate * 100).toFixed(1)}%</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
