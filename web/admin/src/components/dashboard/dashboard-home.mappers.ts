import type { AdminHealth, AdminSummary, LatencyPoint } from '../../types/dashboard'
import type { Channel } from '../../types/channel'
import type { TopMetric, ChannelNode } from './dashboard-home.types'

export function mapTopMetrics(
  health: AdminHealth | undefined,
  summary: AdminSummary | undefined,
  channels: Channel[],
  latencyData: LatencyPoint[] | undefined,
  alertCount: number | undefined,
): TopMetric[] {
  const healthyChannels = channels.filter((c) => c.status === 'active').length
  const openCircuit = channels.filter((c) => c.status === 'error').length

  // Latest P95 from latency trend (last data point)
  const latestP95 = latencyData && latencyData.length > 0
    ? latencyData[latencyData.length - 1].p95
    : undefined

  // Estimated cost from billing summary
  const estimatedCost = summary?.estimated_cost != null
    ? `$${summary.estimated_cost.toFixed(2)}`
    : '--'

  return [
    { id: 'rps', label: 'RPS', value: String(summary?.requests ?? 0), level: 'healthy' },
    { id: 'tokens_per_sec', label: 'TOKEN/s', value: String(summary?.total_tokens ?? 0), level: 'healthy' },
    { id: 'ttff', label: 'TTFF', value: `${Math.round(summary?.avg_latency_ms ?? 0)}ms`, level: (summary?.avg_latency_ms ?? 0) > 1500 ? 'critical' : 'healthy' },
    { id: 'p95', label: 'P95', value: latestP95 != null ? `${latestP95}ms` : '--', level: 'degraded' },
    { id: 'healthy_channels', label: 'HEALTHY CH', value: `${healthyChannels}/${channels.length}`, level: healthyChannels === channels.length ? 'healthy' : 'degraded' },
    { id: 'open_circuit', label: 'OPEN CIRCUIT', value: String(openCircuit), level: openCircuit > 0 ? 'critical' : 'healthy' },
    { id: 'today_cost', label: 'TODAY COST', value: estimatedCost, level: 'healthy' },
    { id: 'alert_count', label: 'ALERTS', value: String(alertCount ?? 0), level: (alertCount ?? 0) > 0 ? 'critical' : 'healthy' },
  ]
}

export function mapChannelNodes(channels: Channel[] = []): ChannelNode[] {
  return channels.map((c) => ({
    id: c.id,
    name: c.name,
    provider: c.provider,
    status: c.status === 'active' ? 'healthy' : c.status === 'inactive' ? 'disabled' : 'critical',
    latency: c.latency_ms ? `${c.latency_ms}ms` : '--',
    errorRate: c.status === 'error' ? '100%' : '0%',
    weight: c.weight,
    requestCount: c.total_requests ?? 0,
    note: c.notes,
  }))
}
