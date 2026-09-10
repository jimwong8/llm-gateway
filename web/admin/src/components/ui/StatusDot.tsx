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

const ariaLabels: Record<StatusDotVariant, string> = {
  healthy: '健康',
  degraded: '降级',
  down: '宕机',
  active: '活跃',
  disabled: '禁用',
  error: '错误',
}

export const StatusDot = React.memo(function StatusDot({ status, label, className = '' }: StatusDotProps) {
  const displayLabel = label ?? ariaLabels[status]
  return (
    <span
      className={`status-dot-wrapper ${className}`.trim()}
      role="status"
      aria-label={`${displayLabel}${label ? `: ${label}` : ''}`}
    >
      <span className={dotStyles[status]} aria-hidden="true" />
      {label ? <span className="status-dot__label">{label}</span> : null}
    </span>
  )
})
