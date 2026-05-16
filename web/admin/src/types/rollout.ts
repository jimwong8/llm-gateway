/** 灰度发布行记录 */
export type RolloutRow = {
  id: string              // 发布 ID
  policy_version_id: string  // 关联策略版本 ID
  environment: string        // 目标环境
  rollout_mode: string       // 发布模式
  rollout_percent: number    // 发布百分比
  status: string             // 状态
  trigger_reason?: string    // 触发原因
  triggered_by?: string      // 触发人
  error_rate?: number        // 错误率
  p95_latency?: number       // P95 延迟
  fallback_rate?: number     // 回退率
  sample_count?: number      // 样本数
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

export type GovernanceRollbackRequest = {
  rollout_id: string
  actor: string
  reason?: string
}
