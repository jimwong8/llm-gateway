import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { buildQuery } from '../lib/format'
import type { ConfigVersion, ConfigVersionFilters } from '../types/admin'

function buildConfigQuery(filters: ConfigVersionFilters) {
  const params: Record<string, string> = {}
  if (filters.module.trim()) params.module = filters.module.trim()
  if (filters.tenantID.trim()) params.tenant_id = filters.tenantID.trim()
  if (filters.environment.trim()) params.environment = filters.environment.trim()
  if (filters.scope.trim()) params.scope = filters.scope.trim()
  if (filters.projectID.trim()) params.project_id = filters.projectID.trim()
  return buildQuery('/admin/config-versions', params)
}

export function useConfigVersions(filters: ConfigVersionFilters) {
  return useQuery({
    queryKey: ['config-versions', filters],
    queryFn: () => apiRequest<ConfigVersion[]>(buildConfigQuery(filters)),
  })
}
