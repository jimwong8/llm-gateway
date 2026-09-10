import { useTranslation } from 'react-i18next'
import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'
import type { HotspotsResult } from '../../types/observability'

type Props = {
  hotspots: HotspotsResult | undefined
}

export function ObservabilityHotspotsSection({ hotspots }: Props) {
  const { t } = useTranslation()
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label={t('observability.hotspotTenant')} value={hotspots?.tenants[0]?.key ?? '—'} />
      <SummaryMetricCard label={t('observability.hotspotModel')} value={hotspots?.models[0]?.key ?? '—'} />
    </div>
  )
}
