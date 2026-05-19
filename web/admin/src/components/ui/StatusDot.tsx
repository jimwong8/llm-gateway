import React from 'react'

type StatusDotVariant = 'healthy' | 'degraded' | 'down' | 'active' | 'disabled' | 'error'

type StatusDotProps = {
  status: StatusDotVariant
  label?: string
  className?: string
}

const dotStyles: Record<StatusDotVariant, string> = {
  healthy: 'status-dot status-dot--healthy',
  degraded: 'status-dot status-dot--degraded',
  down: 'status-dot status-dot--down',
  active: 'status-dot status-dot--active',
  disabled: 'status-dot status-dot--disabled',
  error: 'status-dot status-dot--error',
}

export const StatusDot = React.memo(function StatusDot({ status, label, className = '' }: StatusDotProps) {
  return (
    <span className={`status-dot-wrapper ${className}`.trim()} title={label}>
      <span className={dotStyles[status]} />
      {label ? <span className="status-dot__label">{label}</span> : null}
    </span>
  )
})
