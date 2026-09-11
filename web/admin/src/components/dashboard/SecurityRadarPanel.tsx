import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { SecurityEventItem } from './dashboard-home.types'
import { translateError } from '../../lib/error-translator'

type SecurityRadarPanelProps = {
  events: SecurityEventItem[]
  onSelectEvent: (event: SecurityEventItem) => void
  loading?: boolean
  error?: Error | unknown | null
}

export function SecurityRadarPanel({
  events,
  onSelectEvent,
  loading = false,
  error = null,
}: SecurityRadarPanelProps) {
  const [expandedError, setExpandedError] = useState(false)
  const { t } = useTranslation()
  const translatedError = error ? translateError(error) : null

  return (
    <section className="security-radar-panel" aria-label={t('panel.security.aria')}>
      <header className="panel-header">
        <h2>{t('panel.security.title')}</h2>
        <span>{loading ? t('panel.security.loading') : `${events.length} ${t('panel.security.active')}`}</span>
      </header>

      {loading ? (
        <div className="security-radar-panel__empty" role="status">
          {t('panel.security.loadingLong')}
        </div>
      ) : error ? (
        <div className="security-radar-panel__empty" role="alert">
          <strong>{translatedError?.title ?? t('panel.security.loadFail')}</strong>
          <p className="security-radar-panel__error-body">
            {translatedError?.body ?? t('panel.security.retryHint')}
          </p>
          <button
            type="button"
            className="security-radar-panel__toggle-raw"
            onClick={() => setExpandedError(v => !v)}
          >
            {expandedError ? t('panel.security.hideRaw') : t('panel.security.showRaw')}
          </button>
          {expandedError && (
            <pre className="security-radar-panel__raw">
              {error instanceof Error ? error.message : String(error)}
            </pre>
          )}
        </div>
      ) : events.length === 0 ? (
        <div className="security-radar-panel__empty">{t('panel.security.empty')}</div>
      ) : (
        <div className="security-radar-panel__list">
          {events.map((event) => (
            <button
              key={event.id}
              type="button"
              className={`security-event ${event.level}`}
              onClick={() => onSelectEvent(event)}
              aria-label={`${event.level} ${event.title}`}
            >
              <span>{event.timestamp}</span>
              <strong>{event.title}</strong>
              <p>{event.summary}</p>
            </button>
          ))}
        </div>
      )}
    </section>
  )
}
