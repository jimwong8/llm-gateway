import type {
  ApprovalRequest,
  ApprovalResponse,
  RecommendationsListResponse,
} from '../types/recommendation'
import { apiRequest, jsonRequest } from './http'

export function listGovernanceRecommendations() {
  return apiRequest<RecommendationsListResponse>('/admin/governance/recommendations?limit=50')
}

export function createGovernanceApproval(input: ApprovalRequest) {
  return jsonRequest<ApprovalResponse>('/admin/governance/approvals', input)
}
