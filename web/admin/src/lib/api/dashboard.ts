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
