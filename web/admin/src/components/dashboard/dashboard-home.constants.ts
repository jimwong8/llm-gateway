export const METRIC_ORDER = [
  'rps', 'tokens_per_sec', 'ttff', 'p95',
  'healthy_channels', 'open_circuit', 'today_cost', 'alert_count',
] as const

export const LEVEL_TO_CLASS: Record<string, string> = {
  healthy: 'is-healthy',
  degraded: 'is-degraded',
  critical: 'is-critical',
  disabled: 'is-disabled',
}

export const DASHBOARD_REFRESH = {
  fast: 5000,
  medium: 15000,
  slow: 30000,
} as const