import type { PolicyVersionDiffPayload, PolicyVersionListResponse, PolicyVersionRow } from '../types/policyVersion'
import { apiRequest, jsonRequest } from './http'

export function listPolicyVersions(limit = 50) {
  return apiRequest<PolicyVersionListResponse>(`/admin/governance/policy-versions?limit=${limit}`)
}

export function approvePolicyVersion(versionID: string, approvedBy: string) {
  return jsonRequest<PolicyVersionRow>(`/admin/governance/policy-versions/${versionID}/approve`, {
    approved_by: approvedBy,
  })
}

export function activatePolicyVersion(versionID: string) {
  return jsonRequest<PolicyVersionRow>(`/admin/governance/policy-versions/${versionID}/activate`, {})
}

export function getPolicyVersionDiff(versionID: string) {
  return apiRequest<PolicyVersionDiffPayload>(`/admin/governance/policy-versions/${versionID}/diff`)
}
