import { getUserToken } from './identity'
import { apiRequest } from '../http'
import type {
  UserDashboardData,
  UserUsageResponse,
  TokenUsagePoint,
  ModelDistributionPoint,
  CacheHitPoint,
  ChannelStatusPoint,
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

export async function getUserDashboard(): Promise<UserDashboardData> {
  const res = await fetch('/api/user/dashboard', {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) {
    throw new Error('Failed to fetch user dashboard')
  }
  return res.json()
}

export async function getUserUsage(days = 7): Promise<UserUsageResponse> {
  const res = await fetch(`/api/user/usage?days=${days}`, {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) {
    throw new Error('Failed to fetch user usage')
  }
  return res.json()
}

export async function getUserUsageLogs(limit = 50, offset = 0): Promise<{ object: string; data: UserUsageLog[]; total: number }> {
  return apiRequest(`/api/user/usage-logs?limit=${limit}&offset=${offset}`)
}

export async function getUserCostTrend(days = 30): Promise<{ object: string; data: CostTrendPoint[]; days: number }> {
  return apiRequest(`/api/user/cost-trend?days=${days}`)
}
