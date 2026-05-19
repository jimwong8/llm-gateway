export interface UserDashboardSummary {
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  estimated_cost: number
  avg_latency_ms: number
  provider_error_rate: number
  cache_hit_rate: number
}

export interface UserApiKey {
  id: number
  name: string
  key_prefix: string
  status: string
  last_used_at?: string
  created_at: string
}

export interface UserModelDistribution {
  key: string
  requests: number
  total_tokens: number
  estimated_cost: number
}

export interface UserDashboardData {
  summary: UserDashboardSummary
  recent_api_keys: UserApiKey[]
  model_distribution: UserModelDistribution[]
}

export interface UserDailyUsagePoint {
  date: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  estimated_cost: number
}

export interface UserUsageResponse {
  object: string
  data: UserDailyUsagePoint[]
}

export interface AdminHealth {
  service?: string
  admin_auth?: string
  time?: string
}

export interface AdminSummary {
  requests: number
  total_tokens: number
  cache_hit_rate: number
  provider_error_rate: number
  avg_latency_ms: number
}

export interface TokenUsagePoint {
  date: string
  prompt: number
  completion: number
  total: number
}

export interface ModelDistributionPoint {
  name: string
  value: number
}

export interface CacheHitPoint {
  date: string
  hitRate: number
  requests: number
}

export interface ChannelStatusPoint {
  name: string
  healthy: number
  degraded: number
  down: number
}

export interface UserUsageLog {
  id: number
  user_id: number
  provider: string
  model: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cost_cents: number
  status_code: number
  duration_ms: number
  created_at: string
}

export interface CostTrendPoint {
  date: string
  cost_cents: number
  tokens: number
  requests: number
}

export interface LatencyPoint {
  date: string
  p50: number
  p95: number
  p99: number
}

export interface ErrorRatePoint {
  date: string
  errorRate: number
  totalRequests: number
  errorRequests: number
}
