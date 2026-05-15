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
      <SummaryMetricCard label="Tenant" value={tenantId || '—'} />
      <SummaryMetricCard label="Used" value={used} />
      <SummaryMetricCard label="Remaining" value={remaining} />
      <SummaryMetricCard label="Reject Rate" value={`${(rejectRate * 100).toFixed(1)}%`} />
    </div>
  )
}
