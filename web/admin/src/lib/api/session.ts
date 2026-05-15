import { apiRequest } from '../http'
import type { SessionAdminDashboard } from '../../types/sessionDashboard'

export function fetchSessionDashboard() {
  return apiRequest<SessionAdminDashboard>('/api/v1/admin/dashboard')
}
