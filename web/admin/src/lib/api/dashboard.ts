import { getUserToken } from './identity'
import { apiRequest } from '../http'

export type TokenUsagePoint = {
  date: string
  prompt: number
  completion: number
  total: number
}

export type ModelDistributionPoint = {
  name: string
  value: number
}

export type CacheHitPoint = {
  date: string
  hitRate: number
  requests: number
}

export type ChannelStatusPoint = {
  name: string
  healthy: number
  degraded: number
  down: number
}

export type UserDailyUsagePoint = {
  date: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  estimated_cost: number
}

export type UserDashboardSummary = {
  summary: {
    requests: number
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
    estimated_cost: number
    avg_latency_ms: number
    provider_error_rate: number
    cache_hit_rate: number
  }
  recent_api_keys: {
    id: number
    name: string
    key_prefix: string
    status: string
    last_used_at?: string
    created_at: string
  }[]
  model_distribution: {
    key: string
    requests: number
    total_tokens: number
    estimated_cost: number
  }[]
}

function userAuthHeaders(): HeadersInit {
  const token = getUserToken()
  if (token) {
    return { Authorization: `Bearer ${token}` }
  }
  return {}
}

export async function getTokenUsage(days = 7): Promise<{ data: TokenUsagePoint[] }> {
  return apiRequest<{ data: TokenUsagePoint[] }>(`/admin/dashboard/charts/token-usage?days=${days}`)
}

export async function getModelDistribution(): Promise<{ data: ModelDistributionPoint[] }> {
  return apiRequest<{ data: ModelDistributionPoint[] }>('/admin/dashboard/charts/model-distribution')
}

export async function getCacheHitRate(days = 7): Promise<{ data: CacheHitPoint[] }> {
  return apiRequest<{ data: CacheHitPoint[] }>(`/admin/dashboard/charts/cache-hit-rate?days=${days}`)
}

export async function getChannelStatus(): Promise<{ data: ChannelStatusPoint[] }> {
  return apiRequest<{ data: ChannelStatusPoint[] }>('/admin/dashboard/charts/channel-status')
}

export async function getUserDashboard(): Promise<UserDashboardSummary> {
  const res = await fetch('/api/user/dashboard', {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) {
    throw new Error('Failed to fetch user dashboard')
  }
  return res.json()
}

export async function getUserUsage(days = 7): Promise<{ object: string; data: UserDailyUsagePoint[] }> {
  const res = await fetch(`/api/user/usage?days=${days}`, {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) {
    throw new Error('Failed to fetch user usage')
  }
  return res.json()
}
