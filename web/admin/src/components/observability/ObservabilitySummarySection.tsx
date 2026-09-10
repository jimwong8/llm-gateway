import { useTranslation } from 'react-i18next'
import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'

type Props = {
  requests: number
  cacheHitRate: number
  providerErrorRate: number
  avgLatencyMs: number
}

export function ObservabilitySummarySection({ requests, cacheHitRate, providerErrorRate, avgLatencyMs }: Props) {
  const { t } = useTranslation()
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label={t('observability.requests')} value={requests} />
      <SummaryMetricCard label={t('observability.cacheHitRate')} value={`${(cacheHitRate * 100).toFixed(1)}%`} />
      <SummaryMetricCard label={t('observability.providerErrorRate')} value={`${(providerErrorRate * 100).toFixed(1)}%`} />
      <SummaryMetricCard label={t('observability.avgLatency')} value={`${avgLatencyMs.toFixed(1)} ms`} />
    </div>
  )
}
