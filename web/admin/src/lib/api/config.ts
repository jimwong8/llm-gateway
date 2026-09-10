import { apiRequest, jsonRequest } from '../http'
import type { ConfigVersion, SiteConfig, ConfigSnapshot } from '../../types/admin'

export function createInheritanceDraft(body: {
  module: string
  tenant_id: string
  scope: string
  project_id: string
  source_environment: string
  target_environment: string
  actor: string
  reason: string
}) {
  return jsonRequest<ConfigVersion>('/admin/inheritance-drafts', body)
}

export function releaseDraft(body: {
  module: string
  tenant_id: string
  environment: string
  scope: string
  project_id: string
  version_id: string
  actor: string
  reason: string
}) {
  return jsonRequest<ConfigVersion>('/admin/releases', body)
}

export function promoteConfig(body: {
  module: string
  tenant_id: string
  source_environment: string
  target_environment: string
  scope: string
  project_id: string
  actor: string
  reason: string
}) {
  return jsonRequest<ConfigVersion>('/admin/promotions', body)
}

// ── 站点配置 ─────────────────────────────────────────────
export function fetchSiteConfig() {
  return apiRequest<SiteConfig>('/admin/config/site')
}

export function updateSiteConfig(body: {
  site_name?: string
  logo_url?: string
  smtp_host?: string
  smtp_port?: number
  smtp_user?: string
  smtp_pass?: string
  smtp_from?: string
  allow_registration?: boolean
  default_user_role?: string
  default_user_quota?: number
  updated_by?: string
}) {
  return jsonRequest<SiteConfig>('/admin/config/site', body, { method: 'PUT' } as RequestInit)
}

export function rotateJWTSecret(updatedBy?: string) {
  return jsonRequest<{
    jwt_secret: string
    jwt_secret_rotated_at: string
    message: string
  }>('/admin/config/jwt/rotate', { updated_by: updatedBy ?? 'admin' })
}

// ── 配置版本快照 ─────────────────────────────────────────
export function fetchConfigSnapshots() {
  return apiRequest<{ object: string; data: ConfigSnapshot[] }>('/admin/config/versions')
}

export function createConfigSnapshot(body: {
  version: string
  config_snapshot: string
  notes: string
  created_by?: string
}) {
  return jsonRequest<ConfigSnapshot>('/admin/config/versions', body)
}

export function publishConfigSnapshot(id: number) {
  return jsonRequest<{ id: number; version: string; status: string; published_at: string }>(
    `/admin/config/versions/${id}/publish`,
  )
}

export function rollbackConfigSnapshot(id: number) {
  return jsonRequest<{ id: number; version: string; status: string; rolled_back_at: string }>(
    `/admin/config/versions/${id}/rollback`,
  )
}

export function exportConfigSnapshots() {
  return apiRequest<Blob>('/admin/config/versions/export', {}, { auth: 'admin' } as Record<string, unknown>)
}

export function importConfigSnapshots(data: { data: ConfigSnapshot[] }) {
  return jsonRequest<{ imported: number }>('/admin/config/versions/import', data)
}
