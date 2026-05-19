import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { PoliciesSummarySection } from '../components/policies/PoliciesSummarySection'
import { PoliciesModelsSection } from '../components/policies/PoliciesModelsSection'
import { apiRequest } from '../lib/http'

type PoliciesResponse = {
  tenant_id: string
  models: string[]
}

export function PoliciesPage() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['policies-models'],
    queryFn: () => apiRequest<PoliciesResponse>('/admin/policies/models'),
  })

  return (
    <AppShell
      title={t('policies.title')}
      description={t('policies.description')}
    >
      <div className="events-page">
        {query.isLoading ? <div className="event-state">{t('policies.loading')}</div> : null}
        {query.error ? <div className="config-error">{t('policies.loadError')}</div> : null}

        {!query.isLoading && !query.error ? (
          <PoliciesSummarySection
            tenantId={query.data?.tenant_id ?? '—'}
            modelCount={query.data?.models?.length ?? 0}
          />
        ) : null}

        {!query.isLoading && !query.error ? (
          <PoliciesModelsSection models={query.data?.models ?? []} />
        ) : null}
      </div>
    </AppShell>
  )
}
