/** 可观测性摘要 — 请求量、Token、成本、延迟与错误率 */
export type BillingSummary = {
  requests: number          // 请求总数
  prompt_tokens: number     // 提示 Token 数
  completion_tokens: number // 补全 Token 数
  total_tokens: number      // 总 Token 数
  estimated_cost: number    // 估算成本
  avg_latency_ms: number    // 平均延迟（毫秒）
  provider_error_rate: number  // Provider 错误率
  cache_hit_rate: number    // 缓存命中率
}

export type CacheBreakdownRow = {
  cache_status: string
  cache_layer: string
  requests: number
}

/** Provider 维度统计行 */
export type ProviderBreakdownRow = {
  provider: string           // Provider 名称
  requests: number           // 请求量
  prompt_tokens: number      // 提示 Token
  completion_tokens: number  // 补全 Token
  total_tokens: number       // 总 Token
  estimated_cost: number     // 估算成本
  avg_latency_ms: number     // 平均延迟
  provider_error_rate: number  // 错误率
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
