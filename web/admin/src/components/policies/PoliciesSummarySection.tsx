import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'

type Props = {
  tenantId: string
  modelCount: number
}

export function PoliciesSummarySection({ tenantId, modelCount }: Props) {
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label="Tenant" value={tenantId || '—'} />
      <SummaryMetricCard label="Allowed Models" value={modelCount} />
    </div>
  )
}
