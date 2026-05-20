export type HealthLevel = 'healthy' | 'degraded' | 'critical' | 'disabled'

export type TopMetric = {
  id: string
  label: string
  value: string
  trend?: string
  level: HealthLevel
}

export type ChannelNode = {
  id: string
  name: string
  provider: string
  status: HealthLevel
  latency: string
  errorRate: string
  weight: number
  requestCount: number
  note?: string
}

export type CostQuotaItem = {
  id: string
  label: string
  used: number
  limit: number
  percent: number
  level: HealthLevel
}

export type SecurityEventItem = {
  id: string
  level: 'info' | 'warning' | 'critical'
  title: string
  summary: string
  timestamp: string
  source?: string
}

export type DrawerPayload =
  | { kind: 'channel'; channelId: string }
  | { kind: 'security-event'; eventId: string }
  | { kind: 'cost-item'; itemId: string }
  | null