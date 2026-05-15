export type BillingSummary = {
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  estimated_cost: number
  avg_latency_ms: number
  provider_error_rate: number
  cache_hit_rate: number
}

export type CacheBreakdownRow = {
  cache_status: string
  cache_layer: string
  requests: number
}

export type ProviderBreakdownRow = {
  provider: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  estimated_cost: number
  avg_latency_ms: number
  provider_error_rate: number
}

export type HotspotRow = {
  key: string
  requests: number
  total_tokens: number
  estimated_cost: number
}

export type HotspotsResult = {
  tenants: HotspotRow[]
  models: HotspotRow[]
}

import type { ListResponse } from './common'

export type { ListResponse }
