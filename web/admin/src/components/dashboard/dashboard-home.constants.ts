// ─── Chart Tab ────────────────────────────────────
export type ChartTab = 'tokens' | 'models' | 'cache' | 'channels' | 'latency' | 'errorRate'

export interface ChartTabConfig {
  key: ChartTab
  label: string
  icon: string
}

export const CHART_TAB_CONFIG: ChartTabConfig[] = [
  { key: 'tokens', label: 'Token 趋势', icon: '📊' },
  { key: 'models', label: '模型分布', icon: '🧩' },
  { key: 'cache', label: '缓存命中', icon: '⚡' },
  { key: 'channels', label: '渠道状态', icon: '🔗' },
  { key: 'latency', label: '延迟趋势', icon: '⏱️' },
  { key: 'errorRate', label: '错误率', icon: '🛡️' },
]

// ─── Polling ──────────────────────────────────────
export const DEFAULT_REFETCH_INTERVAL = 30_000 // ms
export const DEFAULT_TREND_DAYS = 7

// ─── Query Keys ───────────────────────────────────
export const QUERY_KEYS = {
  health: ['dashboard-health'],
  observabilitySummary: ['dashboard-observability-summary'],
  channels: ['dashboard-channels'],
  session: ['dashboard-session'],
  charts: {
    tokenUsage: ['dashboard-charts', 'token-usage'],
    modelDistribution: ['dashboard-charts', 'model-distribution'],
    cacheHitRate: ['dashboard-charts', 'cache-hit-rate'],
    channelStatus: ['dashboard-charts', 'channel-status'],
    latency: ['dashboard-charts', 'latency'],
    errorRate: ['dashboard-charts', 'error-rate'],
  },
} as const

// ─── API Endpoints ────────────────────────────────
export const API_ENDPOINTS = {
  health: '/admin/health',
  observabilitySummary: '/admin/observability/summary',
  session: '/admin/dashboard',
} as const
