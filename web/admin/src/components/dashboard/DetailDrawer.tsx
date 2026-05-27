import type { DrawerPayload } from './dashboard-home.types'

export function DetailDrawer({
  open,
  title,
  subtitle,
  children,
  onClose,
}: {
  open: boolean
  title: string
  subtitle?: string
  children: React.ReactNode
  onClose: () => void
}) {
  if (!open) return null
  return (
    <aside className="detail-drawer" aria-label="详情抽屉">
      <header className="detail-drawer__header">
        <div>
          <h2>{title}</h2>
          {subtitle ? <p>{subtitle}</p> : null}
        </div>
        <button type="button" aria-label="close" onClick={onClose}>×</button>
      </header>
      <div className="detail-drawer__body">{children}</div>
    </aside>
  )
}
