import type { DriftListResponse } from '../types/drift'
import { apiRequest } from './http'

export function listGovernanceDrifts() {
  return apiRequest<DriftListResponse>('/admin/governance/drifts?limit=50')
}
