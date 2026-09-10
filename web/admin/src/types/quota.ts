export type QuotaSummary = {
  tenant_id: string
  used: number
  limit: number
  remaining: number
  rejected: number
  reject_rate: number
}

/** 配额趋势数据点 */
export type TrendPoint = {
  minute: string             // 时间（分钟粒度）
  used: number               // 已用量
  rejected: number           // 被拒绝量
  remaining_estimate: number // 剩余估算
}

export type QuotaTrendsResponse = {
  tenant_id: string
  window_minutes: number
  points: TrendPoint[]
}
