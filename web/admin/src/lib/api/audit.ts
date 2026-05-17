import { apiRequest } from '../http'

export type AuditExportParams = {
  tenant_id: string
  format?: string
}

export type AuditRetentionPolicy = {
  retention_days: number
}

export type CleanupResult = {
  status: string
  deleted: number
  retention_days: number
}

export async function exportAuditData(tenantID: string, format = 'json'): Promise<Blob> {
  const token = sessionStorage.getItem('llm_gateway_admin_token')
  const resp = await fetch(`/admin/audit/export?tenant_id=${encodeURIComponent(tenantID)}&format=${format}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!resp.ok) {
    const text = await resp.text()
    throw new Error(`Export failed: ${text}`)
  }
  return resp.blob()
}

export async function triggerCleanup(retentionDays: number): Promise<CleanupResult> {
  return apiRequest<CleanupResult>(`/admin/audit/cleanup?retention_days=${retentionDays}`, {
    method: 'POST',
  })
}

export async function getRetentionPolicy(): Promise<AuditRetentionPolicy> {
  return apiRequest<AuditRetentionPolicy>('/admin/audit/retention')
}
