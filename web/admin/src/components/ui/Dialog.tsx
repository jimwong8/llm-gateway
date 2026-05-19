import { type ReactNode, useEffect, useRef } from 'react'
import { cn } from '../../lib/cn'
import styles from './Dialog.module.css'

type DialogProps = {
  open: boolean
  onClose: () => void
  title?: string
  description?: string
  children: ReactNode
  className?: string
}

export function Dialog({ open, onClose, title, description, children, className }: DialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const previousFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (open) {
      previousFocusRef.current = document.activeElement as HTMLElement
      dialogRef.current?.focus()
    } else if (previousFocusRef.current) {
      previousFocusRef.current?.focus()
      previousFocusRef.current = null
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        onClose()
        return
      }
      if (e.key === 'Tab') {
        const dialog = dialogRef.current
        if (!dialog) return
        const focusable = dialog.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
        )
        if (focusable.length === 0) return
        const first = focusable[0]
        const last = focusable[focusable.length - 1]
        if (e.shiftKey) {
          if (document.activeElement === first) {
            e.preventDefault()
            last.focus()
          }
        } else {
          if (document.activeElement === last) {
            e.preventDefault()
            first.focus()
          }
        }
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

  if (!open) return null

  return (
    <div className={styles.backdrop} onClick={onClose} role="presentation">
      <div
        ref={dialogRef}
        className={cn(styles.dialog, className)}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
      >
        {title ? (
          <div className={styles.header}>
            <div>
              <h2 className={styles.title}>{title}</h2>
              {description ? <p className={styles.description}>{description}</p> : null}
            </div>
            <button
              type="button"
              className={styles.close}
              onClick={onClose}
              aria-label="关闭"
            >
              ✕
            </button>
          </div>
        ) : null}
        {children}
      </div>
    </div>
  )
}
