export type SessionAdminDashboard = {
  overall_status?: string
  health?: {
    status?: string
  }
  continuation?: {
    status?: string
  }
  duplicates?: {
    duplicate_group_count?: number
  }
  alerts?: {
    issues?: Array<string | { [key: string]: unknown }>
  }
  shared_memory?: {
    project_session_count?: number
  }
  continuation_metrics?: {
    pending?: number
  }
  kg?: {
    kg_success?: number
    kg_fail_json_extract?: number
    kg_fail_429?: number
  }
  operation_history?: Array<{
    action: string
    target_type?: string | null
    target_id?: string | null
    status: string
    message?: string | null
    operator?: string | null
    created_at?: string | null
  }>
  ai_ops_advice?: {
    summary?: string
    risk_level?: string
    recommended_actions?: Array<{
      priority?: string
      area?: string
      action: string
      risk?: string | null
    }>
    model_used?: string | null
    error?: string | null
  }
}
