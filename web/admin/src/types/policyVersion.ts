import type { GovernanceListResponse } from './rollout'

export type PolicyVersionStatus = 'draft' | 'approved' | 'active' | 'superseded' | 'rolled_back' | string

export type PolicyVersionRow = {
  id: string
  environment: string
  status: PolicyVersionStatus
  source_approval_id?: string
  created_by?: string
  approved_by?: string
  approved_at?: string
  activated_at?: string
  created_at?: string
}

export type PolicyVersionListResponse = GovernanceListResponse<PolicyVersionRow>

export type PolicyVersionDiffPayload = Record<string, unknown> | unknown[] | string | null
