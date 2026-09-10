import { useTranslation } from 'react-i18next'
import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'

type Props = {
  tenantId: string
  used: number
  remaining: number
  rejectRate: number
}

export function QuotaSummarySection({ tenantId, used, remaining, rejectRate }: Props) {
  const { t } = useTranslation()
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label={t('quota.tenant')} value={tenantId || '—'} />
      <SummaryMetricCard label={t('quota.used')} value={used} />
      <SummaryMetricCard label={t('quota.remaining')} value={remaining} />
      <SummaryMetricCard label={t('quota.rejectRate')} value={`${(rejectRate * 100).toFixed(1)}%`} />
    </div>
  )
}
