import { SummaryMetricCard } from '../dashboard/SummaryMetricCard'

type Props = {
  requests: number
  cacheHitRate: number
  providerErrorRate: number
  avgLatencyMs: number
}

export function ObservabilitySummarySection({ requests, cacheHitRate, providerErrorRate, avgLatencyMs }: Props) {
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label="Requests" value={requests} />
      <SummaryMetricCard label="Cache Hit Rate" value={`${(cacheHitRate * 100).toFixed(1)}%`} />
      <SummaryMetricCard label="Provider Error Rate" value={`${(providerErrorRate * 100).toFixed(1)}%`} />
      <SummaryMetricCard label="Avg Latency" value={`${avgLatencyMs.toFixed(1)} ms`} />
    </div>
  )
}
