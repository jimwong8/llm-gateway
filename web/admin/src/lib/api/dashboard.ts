import { getUserToken } from './identity'
import { apiRequest } from '../http'
import type {
  UserDashboardData,
  UserUsageResponse,
  TokenUsagePoint,
  ModelDistributionPoint,
  CacheHitPoint,
  ChannelStatusPoint,
  LatencyPoint,
  ErrorRatePoint,
  UserUsageLog,
  CostTrendPoint,
} from '../../types/dashboard'

export type {
  UserDashboardData,
  UserUsageResponse,
  UserDashboardSummary,
  UserApiKey,
  UserModelDistribution,
  UserDailyUsagePoint,
  TokenUsagePoint,
  ModelDistributionPoint,
  CacheHitPoint,
  ChannelStatusPoint,
  LatencyPoint,
  ErrorRatePoint,
} from '../../types/dashboard'

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

export async function getLatencyTrend(days = 7): Promise<{ data: LatencyPoint[] }> {
  return apiRequest<{ data: LatencyPoint[] }>(`/admin/observability/latency?days=${days}`)
}

export async function getErrorRateTrend(days = 7): Promise<{ data: ErrorRatePoint[] }> {
  return apiRequest<{ data: ErrorRatePoint[] }>(`/admin/observability/error-rate?days=${days}`)
}

async function userFetch(input: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init)
  if (res.status === 401) {
    sessionStorage.removeItem('llm_gateway_user_token')
    window.location.href = '/admin/ui/login'
    throw new Error('Unauthorized')
  }
  return res
}

export async function getUserDashboard(): Promise<UserDashboardData> {
  const res = await userFetch('/api/user/dashboard', {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) {
    throw new Error('Failed to fetch user dashboard')
  }
  return res.json()
}

export async function getUserUsage(days = 7): Promise<UserUsageResponse> {
  const res = await userFetch(`/api/user/usage?days=${days}`, {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) {
    throw new Error('Failed to fetch user usage')
  }
  return res.json()
}

export async function getUserUsageLogs(limit = 50, offset = 0): Promise<{ object: string; data: UserUsageLog[]; total: number }> {
  const res = await userFetch(`/api/user/usage-logs?limit=${limit}&offset=${offset}`, {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) throw new Error('Failed to fetch usage logs')
  return res.json()
}

export async function getUserCostTrend(days = 30): Promise<{ object: string; data: CostTrendPoint[]; days: number }> {
  const res = await userFetch(`/api/user/cost-trend?days=${days}`, {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) throw new Error('Failed to fetch cost trend')
  return res.json()
}
