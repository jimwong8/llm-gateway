import { apiRequest, jsonRequest } from './http'
import type { LongContextHealth, RunLongContextInput, RunLongContextResponse, StuckTasksResponse } from '../types/longContext'

export function getLongContextHealth() {
  return apiRequest<LongContextHealth>('/admin/long-context/health')
}

export function listStuckTasks() {
  return apiRequest<StuckTasksResponse>('/admin/long-context/tasks/stuck')
}

export function runLongContext(input: RunLongContextInput) {
  return jsonRequest<RunLongContextResponse>('/admin/long-context/run', input, { method: 'POST' })
}
