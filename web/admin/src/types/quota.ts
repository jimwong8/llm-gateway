export type QuotaSummary = {
  tenant_id: string
  used: number
  limit: number
  remaining: number
  rejected: number
  reject_rate: number
}

export type TrendPoint = {
  minute: string
  used: number
  rejected: number
  remaining_estimate: number
}

export type QuotaTrendsResponse = {
  tenant_id: string
  window_minutes: number
  points: TrendPoint[]
}
