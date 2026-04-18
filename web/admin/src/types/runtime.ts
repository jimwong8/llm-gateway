export type AuditEvent = {
  type: string
  tenant_id: string
  environment: string
  version_id: string
  actor?: string
  reason?: string
  created_at: string
}

export type RuntimeEvent = {
  version: {
    module: string
    tenant_id: string
    environment: string
    scope: string
    project_id: string
    version: string
    actor: string
    source: string
    summary: string
    created_at: string
    source_environment?: string
    source_version?: string
  }
}

export type SummaryResponse = {
  total: number
  by_type: Record<string, number>
  by_environment: Record<string, number>
}
