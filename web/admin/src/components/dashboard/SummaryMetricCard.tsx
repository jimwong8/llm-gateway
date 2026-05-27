import React from 'react'

type SummaryMetricCardProps = {
  label: string
  value: string | number
}

export const SummaryMetricCard = React.memo(function SummaryMetricCard({ label, value }: SummaryMetricCardProps) {
  return (
    <section className="summary-metric-card">
      <span className="summary-metric-card__label">{label}</span>
      <strong className="summary-metric-card__value">{value}</strong>
    </section>
  )
})
