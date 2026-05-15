import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'
import type { HotspotsResult } from '../../types/observability'

type Props = {
  hotspots: HotspotsResult | undefined
}

export function ObservabilityHotspotsSection({ hotspots }: Props) {
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label="Top Tenant" value={hotspots?.tenants[0]?.key ?? '—'} />
      <SummaryMetricCard label="Top Model" value={hotspots?.models[0]?.key ?? '—'} />
    </div>
  )
}
