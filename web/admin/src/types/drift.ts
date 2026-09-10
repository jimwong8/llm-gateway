import type { ListResponse } from './common'

export type DriftRow = {
  id: string
  environment: string
  agent_id: string
  active_model: string
  recommended_model: string
  status: 'detected' | 'accepted' | 'resolved' | string
  details?: Record<string, unknown> | string | null
  detected_at?: string
  updated_at?: string
}

export type DriftListResponse = ListResponse<DriftRow>
