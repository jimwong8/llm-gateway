import { useState } from 'react'
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
  const translatedError = error ? translateError(error) : null

  return (
    <section className="security-radar-panel" aria-label="安全风控与审计雷达">
      <header className="panel-header">
        <h2>安全事件</h2>
        <span>{loading ? '加载中…' : `${events.length} active`}</span>
      </header>

      {loading ? (
        <div className="security-radar-panel__empty" role="status">
          正在加载安全事件…
        </div>
      ) : error ? (
        <div className="security-radar-panel__empty" role="alert">
          <strong>{translatedError?.title ?? '加载失败'}</strong>
          <p style={{ marginTop: '0.5rem', fontSize: '0.85rem', color: 'var(--slate-400)' }}>
            {translatedError?.body ?? '请稍后重试'}
          </p>
          <button
            type="button"
            onClick={() => setExpandedError(v => !v)}
            style={{ marginTop: '0.5rem', fontSize: '0.75rem', color: 'var(--slate-400)', background: 'transparent', border: 'none', cursor: 'pointer' }}
          >
            {expandedError ? '收起原始错误' : '查看原始错误'}
          </button>
          {expandedError && (
            <pre style={{ marginTop: '0.5rem', padding: '0.5rem', background: 'var(--gray-900)', color: 'var(--gray-300)', fontSize: '0.7rem', borderRadius: '0.25rem', overflow: 'auto' }}>
              {error instanceof Error ? error.message : String(error)}
            </pre>
          )}
        </div>
      ) : events.length === 0 ? (
        <div className="security-radar-panel__empty">当前没有高风险事件</div>
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
