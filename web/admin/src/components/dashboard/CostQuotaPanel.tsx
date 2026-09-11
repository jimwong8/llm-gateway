import { useTranslation } from 'react-i18next'

export function CostQuotaPanel(props: {
  todayCost: string
  monthCost: string
  cacheHitRate: string
  providerErrorRate: string
  totalTokens: string
}) {
  const { t } = useTranslation()
  return (
    <section className="cost-quota-panel" aria-label={t('panel.costQuota.aria')}>
      <header className="panel-header">
        <h2>{t('panel.costQuota.title')}</h2>
        <span>{t('panel.costQuota.subtitle')}</span>
      </header>
      <div className="cost-quota-panel__grid">
        <article>
          <span>{t('panel.costQuota.todayCost')}</span>
          <strong>{props.todayCost}</strong>
        </article>
        <article>
          <span>{t('panel.costQuota.monthCost')}</span>
          <strong>{props.monthCost}</strong>
        </article>
        <article>
          <span>{t('panel.costQuota.cacheHit')}</span>
          <strong>{props.cacheHitRate}</strong>
        </article>
        <article>
          <span>{t('panel.costQuota.errorRate')}</span>
          <strong>{props.providerErrorRate}</strong>
        </article>
        <article>
          <span>{t('panel.costQuota.totalTokens')}</span>
          <strong>{props.totalTokens}</strong>
        </article>
      </div>
    </section>
  )
}
