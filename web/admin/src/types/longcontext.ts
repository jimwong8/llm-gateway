export type LongContextTaskStatus =
  | 'queued'
  | 'ingesting'
  | 'mapping'
  | 'indexing'
  | 'retrieving'
  | 'synthesizing'
  | 'verifying'
  | 'succeeded'
  | 'failed'
  | 'cancelled'

export type LongContextTaskPhase =
  | 'queued'
  | 'ingesting'
  | 'mapping'
  | 'indexing'
  | 'retrieving'
  | 'synthesizing'
  | 'verifying'

export type LongContextTaskResult = {
  query: string
  answer: string
  evidence_count: number
  citation_coverage: number
  cited_evidence_ids: string[]
}

export type LongContextTask = {
  id: string
  tenant_id: string
  model: string
  mode: 'qa' | 'summary'
  query: string
  status: LongContextTaskStatus
  phase: LongContextTaskPhase
  chunk_count: number
  completed_chunks: number
  evidence_count?: number
  retrieved_count?: number
  conflict_count?: number
  citation_coverage?: number
  retry_count: number
  used_tokens: number
  estimated_cost: number
  last_error: string
  result: LongContextTaskResult | null
  created_at: string
  updated_at: string
}

export type LongContextTaskListResponse = {
  tasks: LongContextTask[]
  total: number
}

export type CreateLongContextTaskInput = {
  model: string
  mode: 'qa' | 'summary'
  query: string
  input: { text: string }
  tenant_id: string
  options?: {
    worker_count?: number
    retrieval_top_k?: number
    final_context_budget?: number
    chunk_tokens?: number
    chunk_overlap_tokens?: number
  }
}

export type AdminRunLongContextInput = {
  task_id: string
  tenant_id: string
  channel: string
  model: string
  reader_channel?: string
  reader_model?: string
  reader_top_k?: number
}
