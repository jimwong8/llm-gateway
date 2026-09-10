import { apiRequest } from '../http'

export type TenantKey = {
  tenant_id: string
  provider: string
  is_active: boolean
  created_at: string
}

export type PutTenantKeyRequest = {
  tenant_id: string
  provider: string
  api_key: string
}

export async function listTenantKeys(tenantID?: string): Promise<{ data: TenantKey[] }> {
  const qs = tenantID ? `?tenant_id=${encodeURIComponent(tenantID)}` : ''
  return apiRequest<{ data: TenantKey[] }>(`/admin/tenant-keys${qs}`)
}

export async function putTenantKey(req: PutTenantKeyRequest): Promise<{ status: string }> {
  return apiRequest<{ status: string }>('/admin/tenant-keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export async function deleteTenantKey(tenantID: string, provider: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/admin/tenant-keys/${encodeURIComponent(tenantID)}/${encodeURIComponent(provider)}`, {
    method: 'DELETE',
  })
}
