import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'

type Props = {
  tenantId: string
  modelCount: number
}

export function PoliciesSummarySection({ tenantId, modelCount }: Props) {
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label="租户" value={tenantId || '—'} />
      <SummaryMetricCard label="允许的模型数" value={modelCount} />
    </div>
  )
}
