import { apiRequest } from '../http'

export type Asset = {
  id: number
  tenant_id: string
  user_id: string
  session_id: string
  source_model: string
  task_type: string
  title: string
  summary: string
  content_hash: string
  source_request_id: string
  hit_count: number
  last_hit_at: string
  last_hit_source: string
  current_version: number
  is_deleted: boolean
  deleted_at: string
  tags: string[]
  created_at: string
  updated_at: string
}

export type AssetStats = {
  total_assets: number
  total_hits: number
  by_task: { task_type: string; count: number }[]
  by_model: { source_model: string; count: number }[]
}

export type AssetFilter = {
  tenant_id?: string
  task_type?: string
  source_model?: string
  keyword?: string
  limit?: number
  offset?: number
}

export type ListResponse<T> = {
  object: string
  data: T[]
  total?: number
}

export async function listAssets(filters?: AssetFilter): Promise<ListResponse<Asset>> {
  const params = new URLSearchParams()
  if (filters?.tenant_id) params.set('tenant_id', filters.tenant_id)
  if (filters?.task_type) params.set('task_type', filters.task_type)
  if (filters?.source_model) params.set('source_model', filters.source_model)
  if (filters?.keyword) params.set('keyword', filters.keyword)
  if (filters?.limit) params.set('limit', String(filters.limit))
  if (filters?.offset) params.set('offset', String(filters.offset))
  const qs = params.toString()
  return apiRequest<ListResponse<Asset>>(`/admin/assets${qs ? `?${qs}` : ''}`)
}

export async function getAssetStats(): Promise<AssetStats> {
  return apiRequest<AssetStats>('/admin/assets/stats')
}

export async function getAsset(id: number): Promise<Asset> {
  return apiRequest<Asset>(`/admin/assets/${id}`)
}

export async function deleteAsset(id: number): Promise<void> {
  return apiRequest<void>(`/admin/assets/${id}`, { method: 'DELETE' })
}
