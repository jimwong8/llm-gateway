type SummaryMetricCardProps = {
  label: string
  value: string | number
}

export function SummaryMetricCard({ label, value }: SummaryMetricCardProps) {
  return (
    <section className="summary-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </section>
  )
}
