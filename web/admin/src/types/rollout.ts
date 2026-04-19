export type GovernanceListResponse<T> = {
  object: string
  data: T[]
}

export type RolloutRow = {
  id: string
  policy_version_id: string
  environment: string
  rollout_mode: string
  rollout_percent: number
  status: string
  trigger_reason?: string
  triggered_by?: string
  error_rate?: number
  p95_latency?: number
  fallback_rate?: number
  sample_count?: number
  created_at?: string
  updated_at?: string
}

export type GovernanceRollbackRow = {
  id: string
  rollout_id: string
  actor: string
  environment: string
  restored_policy_version_id: string
  reverted_policy_version_id: string
  reason?: string
  status?: string
  created_at?: string
}

export type GovernanceRollbackResponse = {
  rollback: GovernanceRollbackRow
  result: unknown
}

export type GovernanceRollbackRequest = {
  rollout_id: string
  actor: string
  reason?: string
}
