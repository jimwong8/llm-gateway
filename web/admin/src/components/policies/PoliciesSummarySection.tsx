import { useTranslation } from 'react-i18next'
import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'

type Props = {
  tenantId: string
  modelCount: number
}

export function PoliciesSummarySection({ tenantId, modelCount }: Props) {
  const { t } = useTranslation()
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label={t('policies.tenant')} value={tenantId || '—'} />
      <SummaryMetricCard label={t('policies.allowedModels')} value={modelCount} />
    </div>
  )
}
