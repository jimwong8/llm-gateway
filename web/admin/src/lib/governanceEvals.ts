import { apiRequest, jsonRequest } from './http'
import type { ListEvaluationRunsResponse, StartEvaluationRunInput } from '../types/governanceEval'

export function listEvaluationRuns(limit = 50) {
  return apiRequest<ListEvaluationRunsResponse>(`/admin/governance/evaluations?limit=${limit}`)
}

export function startEvaluationRun(input: StartEvaluationRunInput) {
  return jsonRequest<unknown>('/admin/governance/evaluations', input, { method: 'POST' })
}
