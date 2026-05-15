import { jsonRequest } from '../http'
import type { ConfigVersion } from '../../types/admin'

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
