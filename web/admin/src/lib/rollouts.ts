import type { GovernanceRollbackRequest, GovernanceRollbackRow, RolloutRow } from '../types/rollout'
import type { ListResponse } from '../types/common'
import { apiRequest, jsonRequest } from './http'

export function listGovernanceRollouts() {
  return apiRequest<ListResponse<RolloutRow>>('/admin/governance/rollouts?limit=50')
}

export function listRolloutDashboard() {
  return apiRequest<ListResponse<RolloutRow>>('/admin/governance/dashboard/rollouts?limit=50')
}

export function createGovernanceRollback(input: GovernanceRollbackRequest) {
  return jsonRequest<GovernanceRollbackRow>('/admin/governance/rollbacks', input)
}
