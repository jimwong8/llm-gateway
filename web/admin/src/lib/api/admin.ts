import { apiRequest } from '../http'
import type { BillingSummary, HotspotsResult, ListResponse, ProviderBreakdownRow } from '../../types/observability'
import type { QuotaSummary, QuotaTrendsResponse } from '../../types/quota'
import type { ConfigVersion, ConfigVersionFilters } from '../../types/admin'
import type { AuditEvent, RuntimeEvent, SummaryResponse } from '../../types/runtime'

export type AdminHealth = {
  service?: string
  admin_auth?: string
  time?: string
}

export type SystemInfo = {
  service?: string
  time?: string
  admin_auth?: string
  object?: string
  data?: unknown[]
}

export type PoliciesModelsResponse = {
  tenant_id: string
  models: string[]
}

function toQuery(params: Record<string, string>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v.trim()) sp.set(k, v.trim())
  }
  const qs = sp.toString()
  return qs ? `?${qs}` : ''
}

// ── 健康检查 / 系统状态 ─────────────────────────────────
export function fetchAdminHealth() {
  return apiRequest<AdminHealth>('/admin/health')
}

export function fetchAdminUsage() {
  return apiRequest<SystemInfo>('/admin/usage')
}

export function fetchAdminAudit() {
  return apiRequest<SystemInfo>('/admin/audit')
}

// ── 可观测性 ─────────────────────────────────────────────
export function fetchObservabilitySummary(params?: {
  tenant_id?: string
  provider?: string
  model?: string
  limit?: string
}) {
  return apiRequest<BillingSummary>(
    `/admin/observability/summary${params ? toQuery(params as Record<string, string>) : ''}`,
  )
}

export function fetchObservabilityProviders(params?: {
  tenant_id?: string
  provider?: string
  model?: string
  limit?: string
}) {
  return apiRequest<ListResponse<ProviderBreakdownRow>>(
    `/admin/observability/providers${params ? toQuery(params as Record<string, string>) : ''}`,
  )
}

export function fetchObservabilityHotspots(params?: {
  tenant_id?: string
  provider?: string
  model?: string
  limit?: string
}) {
  return apiRequest<HotspotsResult>(
    `/admin/observability/hotspots${params ? toQuery(params as Record<string, string>) : ''}`,
  )
}

// ── 配额管理 ─────────────────────────────────────────────
export function fetchQuotaSummary(tenantID?: string) {
  const qs = tenantID?.trim()
    ? `?tenant_id=${encodeURIComponent(tenantID.trim())}`
    : ''
  return apiRequest<QuotaSummary>(`/admin/observability/quota${qs}`)
}

export function fetchQuotaTrends(params: { tenant_id: string; window_minutes: string }) {
  return apiRequest<QuotaTrendsResponse>(
    `/admin/observability/quota/trends${toQuery(params)}`,
  )
}

// ── 策略管理 ─────────────────────────────────────────────
export function fetchPoliciesModels() {
  return apiRequest<PoliciesModelsResponse>('/admin/policies/models')
}

// ── 配置版本 ─────────────────────────────────────────────
export function fetchConfigVersions(filters: ConfigVersionFilters) {
  const params: Record<string, string> = {}
  if (filters.module.trim()) params.module = filters.module.trim()
  if (filters.tenantID.trim()) params.tenant_id = filters.tenantID.trim()
  if (filters.environment.trim()) params.environment = filters.environment.trim()
  if (filters.scope.trim()) params.scope = filters.scope.trim()
  if (filters.projectID.trim()) params.project_id = filters.projectID.trim()
  return apiRequest<ConfigVersion[]>(
    `/admin/config-versions${toQuery(params)}`,
  )
}

// ── 事件 ─────────────────────────────────────────────────
export function fetchAuditEvents(filters: {
  tenantID: string
  environment: string
  limit: string
  summary: boolean
}) {
  const params: Record<string, string> = {}
  if (filters.tenantID.trim()) params.tenant_id = filters.tenantID.trim()
  if (filters.environment.trim()) params.environment = filters.environment.trim()
  if (filters.limit.trim()) params.limit = filters.limit.trim()
  if (filters.summary) params.summary = 'true'
  return apiRequest<AuditEvent[] | SummaryResponse>(
    `/admin/audit-events${toQuery(params)}`,
  )
}

export function fetchRuntimeEvents(filters: {
  tenantID: string
  environment: string
  limit: string
  summary: boolean
}) {
  const params: Record<string, string> = {}
  if (filters.tenantID.trim()) params.tenant_id = filters.tenantID.trim()
  if (filters.environment.trim()) params.environment = filters.environment.trim()
  if (filters.limit.trim()) params.limit = filters.limit.trim()
  if (filters.summary) params.summary = 'true'
  return apiRequest<RuntimeEvent[] | SummaryResponse>(
    `/admin/runtime-events${toQuery(params)}`,
  )
}
