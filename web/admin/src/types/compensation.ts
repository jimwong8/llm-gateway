export interface CompensationRecord {
  module: string
  tenant_id: string
  environment: string
  version: string
  failed_stage: string
  error_summary: string
  suggested_action: string
  created_at: string
}

export interface ListCompensationsResponse {
  object: 'list'
  data: CompensationRecord[]
  summary: {
    total: number
    filtered_total: number
    returned: number
    filters: Record<string, string>
  }
}

export interface ReplayCompensationInput {
  tenant_id: string
  environment: string
  version: string
  module: string
}
