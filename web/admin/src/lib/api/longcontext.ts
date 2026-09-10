import { apiRequest } from '../http'
import { buildQuery } from '../format'
import type {
  AdminRunLongContextInput,
  CreateLongContextTaskInput,
  LongContextTask,
  LongContextTaskListResponse,
} from '../../types/longcontext'

export function fetchLongContextTasks(params?: { tenant_id?: string; limit?: number; offset?: number }) {
  return apiRequest<LongContextTaskListResponse>(
    buildQuery('/v1/long-context/tasks', {
      ...(params?.tenant_id ? { tenant_id: params.tenant_id } : {}),
      ...(params?.limit !== undefined ? { limit: String(params.limit) } : {}),
      ...(params?.offset !== undefined ? { offset: String(params.offset) } : {}),
    }),
  )
}

export function fetchLongContextTask(id: string, tenantID: string) {
  return apiRequest<LongContextTask>(buildQuery(`/v1/long-context/tasks/${id}`, { tenant_id: tenantID }))
}

export function createLongContextTask(input: CreateLongContextTaskInput) {
  return apiRequest<LongContextTask>('/v1/long-context/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function cancelLongContextTask(id: string, tenantID: string) {
  return apiRequest<{ status: string }>(`/v1/long-context/tasks/${id}/cancel`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tenant_id: tenantID }),
  })
}

export function runLongContextTask(input: AdminRunLongContextInput) {
  return apiRequest<LongContextTask>('/admin/long-context/run', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function fetchLongContextHealth() {
  return apiRequest<{ status: string; db: string }>('/admin/long-context/health')
}
