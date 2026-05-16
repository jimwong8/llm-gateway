import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'
import type { HotspotsResult } from '../../types/observability'

type Props = {
  hotspots: HotspotsResult | undefined
}

export function ObservabilityHotspotsSection({ hotspots }: Props) {
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label="热点租户" value={hotspots?.tenants[0]?.key ?? '—'} />
      <SummaryMetricCard label="热点模型" value={hotspots?.models[0]?.key ?? '—'} />
    </div>
  )
}
