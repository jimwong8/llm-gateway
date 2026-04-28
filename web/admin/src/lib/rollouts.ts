import type { GovernanceListResponse, GovernanceRollbackRequest, GovernanceRollbackRow, RolloutRow } from '../types/rollout'
import { apiRequest, jsonRequest } from './http'

export function listGovernanceRollouts() {
  return apiRequest<GovernanceListResponse<RolloutRow>>('/admin/governance/rollouts?limit=50')
}

export function listRolloutDashboard() {
  return apiRequest<GovernanceListResponse<RolloutRow>>('/admin/governance/dashboard/rollouts?limit=50')
}

export function createGovernanceRollback(input: GovernanceRollbackRequest) {
  return jsonRequest<GovernanceRollbackRow>('/admin/governance/rollbacks', input)
}
