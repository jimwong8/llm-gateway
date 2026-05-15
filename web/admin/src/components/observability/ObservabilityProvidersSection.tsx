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
            <th>Requests</th>
            <th>Total Tokens</th>
            <th>Error Rate</th>
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
