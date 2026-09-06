export type EvaluationRunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'

export interface EvaluationRun {
  id: string
  dataset_id: string
  formula_id: string
  agent_id: string
  task_type: string
  environment: string
  status: EvaluationRunStatus
  started_at: string
  completed_at: string
  created_at: string
}

export interface ListEvaluationRunsResponse {
  object: 'list'
  data: EvaluationRun[]
}

export interface StartEvaluationRunInput {
  dataset_id: string
  agent_id: string
  task_type: string
  environment: string
}
