import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { AppShell } from '../components/layout/AppShell'
import { PromotionPanel } from '../components/releases/PromotionPanel'
import { ReleaseDraftPanel } from '../components/releases/ReleaseDraftPanel'
import type { ConfigVersion } from '../types/admin'

export function ReleasesPage() {
  const { t } = useTranslation()
  const [lastResult, setLastResult] = useState<ConfigVersion | null>(null)

  return (
    <AppShell
      title={t('releases.title')}
      description={t('releases.description')}
    >
      <div className="releases-page">
        <div className="releases-grid">
          <ReleaseDraftPanel onReleased={setLastResult} />
          <PromotionPanel onPromoted={setLastResult} />
        </div>

        <section className="release-result-card">
          <h2>{t('releases.lastResult')}</h2>
          {lastResult ? (
            <dl className="release-result-grid">
              <div>
                <dt>{t('releases.versionId')}</dt>
                <dd>{lastResult.version_id}</dd>
              </div>
              <div>
                <dt>{t('releases.status')}</dt>
                <dd>{lastResult.status}</dd>
              </div>
              <div>
                <dt>{t('releases.environment')}</dt>
                <dd>{lastResult.environment}</dd>
              </div>
              <div>
                <dt>{t('releases.source')}</dt>
                <dd>
                  {lastResult.source
                    ? `${lastResult.source.source_environment} / ${lastResult.source.source_version_id}`
                    : '—'}
                </dd>
              </div>
            </dl>
          ) : (
            <div className="config-drawer__empty">{t('releases.emptyResult')}</div>
          )}
        </section>
      </div>
    </AppShell>
  )
}
