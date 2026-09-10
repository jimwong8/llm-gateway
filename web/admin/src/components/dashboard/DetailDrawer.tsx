import { useEffect } from 'react'
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
  // Esc 关闭 + body scroll lock
  useEffect(() => {
    if (!open) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onClose()
      }
    }

    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    window.addEventListener('keydown', handleKeyDown, true)
    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
      document.body.style.overflow = previousOverflow
    }
  }, [open, onClose])

  if (!open) return null
  return (
    <aside className="detail-drawer" aria-label="详情抽屉" role="dialog" aria-modal="true">
      <header className="detail-drawer__header">
        <div>
          <h2>{title}</h2>
          {subtitle ? <p>{subtitle}</p> : null}
        </div>
        <button type="button" aria-label="关闭 Close" onClick={onClose}>
          ×
        </button>
      </header>
      <div className="detail-drawer__body">{children}</div>
    </aside>
  )
}
