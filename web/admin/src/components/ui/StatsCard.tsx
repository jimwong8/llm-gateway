import React from 'react'

type StatsCardProps = {
  title: string
  value: string | number
  subtitle?: string
  trend?: { value: number; label: 'up' | 'down' }
  icon?: React.ReactNode
  className?: string
}

export const StatsCard = React.memo(function StatsCard({ title, value, subtitle, trend, icon, className = '' }: StatsCardProps) {
  return (
    <div className={`stats-card ${className}`.trim()}>
      <div className="stats-card__header">
        <span className="stats-card__title">{title}</span>
        {icon ? <span className="stats-card__icon">{icon}</span> : null}
      </div>
      <strong className="stats-card__value">{value}</strong>
      {trend ? (
        <span
          className={`stats-card__trend stats-card__trend--${trend.label}`}
        >
          {trend.label === 'up' ? '↑' : '↓'} {Math.abs(trend.value)}%
        </span>
      ) : null}
      {subtitle ? <span className="stats-card__subtitle">{subtitle}</span> : null}
    </div>
  )
})
