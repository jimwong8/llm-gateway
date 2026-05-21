import type { SecurityEventItem } from './dashboard-home.types'

export function SecurityRadarPanel({ events, onSelectEvent }: {
  events: SecurityEventItem[]
  onSelectEvent: (event: SecurityEventItem) => void
}) {
  return (
    <section className="security-radar-panel" aria-label="安全风控与审计雷达">
      <header className="panel-header">
        <h2>[ SECURITY EVENTS ]</h2>
        <span>{events.length} active</span>
      </header>
      {events.length === 0 ? (
        <div className="security-radar-panel__empty">当前没有高风险事件</div>
      ) : (
        <div className="security-radar-panel__list">
          {events.map((event) => (
            <button key={event.id} type="button" className={`security-event ${event.level}`} onClick={() => onSelectEvent(event)}>
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