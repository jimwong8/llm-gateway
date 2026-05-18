import { createContext, useContext, useState, useCallback, type ReactNode } from 'react'
import { cn } from '../../lib/cn'
import styles from './Toast.module.css'

type ToastVariant = 'success' | 'error' | 'warning' | 'info'

type ToastItem = {
  id: number
  message: string
  variant: ToastVariant
}

type ToastContextValue = {
  addToast: (message: string, variant?: ToastVariant) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

let toastIdCounter = 0

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    throw new Error('useToast must be used within a ToastProvider')
  }
  return ctx
}

type ToastProviderProps = {
  children: ReactNode
}

export function ToastProvider({ children }: ToastProviderProps) {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const addToast = useCallback((message: string, variant: ToastVariant = 'info') => {
    const id = ++toastIdCounter
    setToasts((prev) => [...prev, { id, message, variant }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 4000)
  }, [])

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  return (
    <ToastContext.Provider value={{ addToast }}>
      {children}
      <div className={styles.container} aria-live="polite" aria-label="通知">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className={cn(styles.toast, styles[`variant-${toast.variant}`])}
            role="alert"
          >
            <span className={styles.icon}>
              {toast.variant === 'success' ? '✓' : toast.variant === 'error' ? '✕' : toast.variant === 'warning' ? '⚠' : 'ℹ'}
            </span>
            <span className={styles.message}>{toast.message}</span>
            <button
              type="button"
              className={styles.dismiss}
              onClick={() => removeToast(toast.id)}
              aria-label="关闭通知"
            >
              ✕
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

type ToastProps = {
  message: string
  variant?: ToastVariant
  onDismiss?: () => void
}

export function Toast({ message, variant = 'info', onDismiss }: ToastProps) {
  return (
    <div className={cn(styles.toast, styles[`variant-${variant}`])} role="alert">
      <span className={styles.icon}>
        {variant === 'success' ? '✓' : variant === 'error' ? '✕' : variant === 'warning' ? '⚠' : 'ℹ'}
      </span>
      <span className={styles.message}>{message}</span>
      {onDismiss ? (
        <button type="button" className={styles.dismiss} onClick={onDismiss} aria-label="关闭通知">
          ✕
        </button>
      ) : null}
    </div>
  )
}
