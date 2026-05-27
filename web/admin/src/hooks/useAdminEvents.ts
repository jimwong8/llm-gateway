import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { buildQuery } from '../lib/format'
import type { AuditEvent, RuntimeEvent, SummaryResponse } from '../types/runtime'

type EventFilters = {
  tenantID: string
  environment: string
  limit: string
  summary: boolean
}

function buildEventQuery(path: string, filters: EventFilters) {
  const params: Record<string, string> = {}
  if (filters.tenantID.trim()) params.tenant_id = filters.tenantID.trim()
  if (filters.environment.trim()) params.environment = filters.environment.trim()
  if (filters.limit.trim()) params.limit = filters.limit.trim()
  if (filters.summary) params.summary = 'true'
  return buildQuery(path, params)
}

export function useAuditEvents(filters: EventFilters) {
  return useQuery({
    queryKey: ['audit-events', filters],
    queryFn: () => apiRequest<AuditEvent[] | SummaryResponse>(buildEventQuery('/admin/audit-events', filters)),
  })
}

export function useRuntimeEvents(filters: EventFilters) {
  return useQuery({
    queryKey: ['runtime-events', filters],
    queryFn: () => apiRequest<RuntimeEvent[] | SummaryResponse>(buildEventQuery('/admin/runtime-events', filters)),
  })
}
