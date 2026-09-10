import type { RuntimeObserverResponse } from '../types/runtimeObserver'
import { apiRequest } from './http'

export function getRuntimeObserver(environment: string, limit = 20) {
  const params = new URLSearchParams()
  if (environment.trim()) {
    params.set('environment', environment.trim())
  }
  params.set('limit', String(limit))
  return apiRequest<RuntimeObserverResponse>(`/admin/governance/runtime-observer?${params.toString()}`)
}
