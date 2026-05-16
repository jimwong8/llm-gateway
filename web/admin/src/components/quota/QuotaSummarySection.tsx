import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'

type Props = {
  tenantId: string
  used: number
  remaining: number
  rejectRate: number
}

export function QuotaSummarySection({ tenantId, used, remaining, rejectRate }: Props) {
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label="租户" value={tenantId || '—'} />
      <SummaryMetricCard label="已用" value={used} />
      <SummaryMetricCard label="剩余" value={remaining} />
      <SummaryMetricCard label="拒绝率" value={`${(rejectRate * 100).toFixed(1)}%`} />
    </div>
  )
}
