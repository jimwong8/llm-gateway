import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import type { ConfigVersion, ConfigVersionFilters } from '../types/admin'

function buildQuery(filters: ConfigVersionFilters) {
  const params = new URLSearchParams()

  if (filters.module.trim()) params.set('module', filters.module.trim())
  if (filters.tenantID.trim()) params.set('tenant_id', filters.tenantID.trim())
  if (filters.environment.trim()) params.set('environment', filters.environment.trim())
  if (filters.scope.trim()) params.set('scope', filters.scope.trim())
  if (filters.projectID.trim()) params.set('project_id', filters.projectID.trim())

  const query = params.toString()
  return query ? `/admin/config-versions?${query}` : '/admin/config-versions'
}

export function useConfigVersions(filters: ConfigVersionFilters) {
  return useQuery({
    queryKey: ['config-versions', filters],
    queryFn: () => apiRequest<ConfigVersion[]>(buildQuery(filters)),
  })
}
