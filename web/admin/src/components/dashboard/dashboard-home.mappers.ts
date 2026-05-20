import type { AdminHealth, AdminSummary } from '../../types/dashboard'
import type { Channel } from '../../types/channel'
import type { TopMetric, ChannelNode } from './dashboard-home.types'

export function mapTopMetrics(
  health: AdminHealth | undefined,
  summary: AdminSummary | undefined,
  channels: Channel[]
): TopMetric[] {
  const healthyChannels = channels.filter((c) => c.status === 'active').length
  const openCircuit = channels.filter((c) => c.status === 'error').length

  return [
    { id: 'rps', label: 'RPS', value: String(summary?.requests ?? 0), level: 'healthy' },
    { id: 'tokens_per_sec', label: 'TOKEN/s', value: String(summary?.total_tokens ?? 0), level: 'healthy' },
    { id: 'ttff', label: 'TTFF', value: `${summary?.avg_latency_ms ?? 0}ms`, level: (summary?.avg_latency_ms ?? 0) > 1500 ? 'critical' : 'healthy' },
    { id: 'p95', label: 'P95', value: `${summary?.avg_latency_ms ?? 0}ms`, level: 'degraded' },
    { id: 'healthy_channels', label: 'HEALTHY CH', value: `${healthyChannels}/${channels.length}`, level: healthyChannels === channels.length ? 'healthy' : 'degraded' },
    { id: 'open_circuit', label: 'OPEN CIRCUIT', value: String(openCircuit), level: openCircuit > 0 ? 'critical' : 'healthy' },
    { id: 'today_cost', label: 'TODAY COST', value: `$${0}`, level: 'healthy' }, // estimated_cost not available in AdminSummary
    { id: 'alert_count', label: 'ALERTS', value: `0`, level: 'healthy' }, // compensation_stats not available in AdminHealth
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