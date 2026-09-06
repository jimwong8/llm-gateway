export interface LongContextHealth {
  status: 'healthy' | 'unhealthy'
  db: string
}

export interface StuckTask {
  id: string
  tenant_id: string
  status: string
  phase: string
  completed_chunks: number
  chunk_count: number
  pending_chunks: number
  failed_chunks: number
  lease_owner: string
  lease_until: string
  last_error: string
  updated_at: string
  minutes_since_update: number
}

export interface StuckTasksResponse {
  tasks: StuckTask[]
  count: number
}

export interface RunLongContextInput {
  task_id: string
  tenant_id: string
  channel: string
  model: string
  max_chunks?: number
  reader_model?: string
  reader_channel?: string
  reader_top_k?: number
}

export interface RunLongContextResponse {
  processed: number
  successes: number
  [key: string]: unknown
}
