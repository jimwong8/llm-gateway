import { apiRequest, jsonRequest } from './http'
import type { ListCompensationsResponse, ReplayCompensationInput } from '../types/compensation'

export interface CompensationFilters {
  tenantID?: string
  environment?: string
  failedStage?: string
  module?: string
  limit?: number
}

export function listCompensations(filters: CompensationFilters = {}) {
  const params = new URLSearchParams()
  if (filters.tenantID) params.set('tenant_id', filters.tenantID)
  if (filters.environment) params.set('environment', filters.environment)
  if (filters.failedStage) params.set('failed_stage', filters.failedStage)
  if (filters.module) params.set('module', filters.module)
  if (filters.limit) params.set('limit', String(filters.limit))
  const qs = params.toString()
  return apiRequest<ListCompensationsResponse>(`/admin/control-plane/compensations${qs ? `?${qs}` : ''}`)
}

export function replayCompensation(input: ReplayCompensationInput) {
  return jsonRequest<unknown>('/admin/control-plane/compensations/replay', input, { method: 'POST' })
}
