import type { ReactNode } from 'react'

export function ChartEmptyState({ label }: { label: string }) {
  return (
    <div
      className="chart-empty-state"
      style={{
        display: 'grid',
        placeItems: 'center',
        minHeight: '220px',
        color: 'var(--slate-400)',
        fontSize: '0.9rem',
      }}
      role="status"
      aria-label={label}
    >
      <span>{label}</span>
    </div>
  )
}

export function ChartLoading({ label = '加载中…' }: { label?: string }) {
  return (
    <div
      className="chart-loading"
      style={{
        display: 'grid',
        placeItems: 'center',
        minHeight: '220px',
        color: 'var(--slate-400)',
      }}
      role="status"
      aria-live="polite"
    >
      <span>{label}</span>
    </div>
  )
}

export function ChartErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const message = error instanceof Error ? error.message : String(error ?? '加载失败')
  return (
    <div
      className="chart-error-state"
      style={{
        display: 'grid',
        placeItems: 'center',
        minHeight: '220px',
        color: 'var(--color-danger)',
        padding: '1rem',
        textAlign: 'center',
      }}
      role="alert"
    >
      <span>加载失败: {message}</span>
      {onRetry && (
        <button
          type="button"
          className="btn"
          style={{ marginTop: '0.75rem' }}
          onClick={onRetry}
          aria-label="重试加载"
        >
          重试
        </button>
      )}
    </div>
  )
}

export type { ReactNode }
