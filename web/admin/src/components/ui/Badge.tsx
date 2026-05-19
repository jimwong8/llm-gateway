import React from 'react'

type BadgeVariant = 'default' | 'success' | 'warning' | 'danger' | 'info'

type BadgeProps = {
  children: React.ReactNode
  variant?: BadgeVariant
  className?: string
}

const variantStyles: Record<BadgeVariant, string> = {
  default: 'badge badge--default',
  success: 'badge badge--success',
  warning: 'badge badge--warning',
  danger: 'badge badge--danger',
  info: 'badge badge--info',
}

export const Badge = React.memo(function Badge({ children, variant = 'default', className = '' }: BadgeProps) {
  return (
    <span className={`${variantStyles[variant]} ${className}`.trim()}>
      {children}
    </span>
  )
})
