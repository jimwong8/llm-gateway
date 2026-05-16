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
      <SummaryMetricCard label="请求量" value={requests} />
      <SummaryMetricCard label="缓存命中率" value={`${(cacheHitRate * 100).toFixed(1)}%`} />
      <SummaryMetricCard label="Provider 错误率" value={`${(providerErrorRate * 100).toFixed(1)}%`} />
      <SummaryMetricCard label="平均延迟" value={`${avgLatencyMs.toFixed(1)} ms`} />
    </div>
  )
}
