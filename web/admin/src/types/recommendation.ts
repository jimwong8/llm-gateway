import type { GovernanceListResponse } from './rollout'

export type RecommendationRow = {
  id: string
  agent_id: string
  task_type: string
  environment: string
  recommended_model: string
  status: string
  created_at?: string
  updated_at?: string
}

export type ApprovalRequest = {
  recommendation_id: string
  decision: 'approved' | 'rejected' | 'overridden'
  approved_by: string
  approval_reason?: string
  final_model?: string
  effective_scope: {
    scope: string
    project_id?: string
    environment: string
  }
}

export type RecommendationsListResponse = GovernanceListResponse<RecommendationRow>

export type ApprovalResponse = {
  id: string
  recommendation_id: string
  status: string
  final_model?: string
  actor?: string
}