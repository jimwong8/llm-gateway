import type { QuotaTrendsResponse } from '../../types/quota'

type Props = {
  trends: QuotaTrendsResponse | undefined
}

export function QuotaTrendsSection({ trends }: Props) {
  const points = trends?.points ?? []
  return (
    <div className="event-table">
      <table>
        <thead>
          <tr>
            <th>时间（分钟）</th>
            <th>已用</th>
            <th>被拒</th>
            <th>剩余估算</th>
          </tr>
        </thead>
        <tbody>
          {points.map((point) => (
            <tr key={point.minute}>
              <td>{point.minute}</td>
              <td>{point.used}</td>
              <td>{point.rejected}</td>
              <td>{point.remaining_estimate}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
