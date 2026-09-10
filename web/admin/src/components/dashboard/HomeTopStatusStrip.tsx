import type { TopMetric } from './dashboard-home.types'
import { SummaryMetricCard } from './SummaryMetricCard'

export function HomeTopStatusStrip({
  metrics,
  onClickMetric,
}: {
  metrics: TopMetric[]
  onClickMetric: (metric: TopMetric) => void
}) {
  return (
    <section className="gateway-top-strip" aria-label="全局北极星指标">
      {metrics.map((metric) => (
        <button
          key={metric.id}
          type="button"
          className={`gateway-top-strip__item ${metric.level}`}
          onClick={() => onClickMetric(metric)}
        >
          <SummaryMetricCard label={metric.label} value={metric.value} />
        </button>
      ))}
    </section>
  )
}
