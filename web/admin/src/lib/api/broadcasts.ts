import { apiRequest, jsonRequest } from '../http'
import type { Broadcast } from '../../types/broadcast'

type BroadcastListResponse = {
  object: string
  data: Broadcast[]
}

type BroadcastActiveResponse = {
  object: string
  data: Broadcast[]
  read_ids: number[]
}

type BroadcastInput = {
  title: string
  content: string
  type: string
  start_at: string
  end_at: string
}

export function listBroadcasts() {
  return apiRequest<BroadcastListResponse>('/admin/broadcasts')
}

export function createBroadcast(input: BroadcastInput) {
  return jsonRequest<Broadcast>('/admin/broadcasts', input)
}

export function updateBroadcast(id: number, input: BroadcastInput) {
  return jsonRequest<Broadcast>(`/admin/broadcasts/${id}`, input, { method: 'PUT' })
}

export function deleteBroadcast(id: number) {
  return apiRequest<{ status: string }>(`/admin/broadcasts/${id}`, { method: 'DELETE' })
}

export function listActiveBroadcasts() {
  return apiRequest<BroadcastActiveResponse>('/api/user/broadcasts')
}

export function markBroadcastRead(id: number) {
  return apiRequest<{ status: string }>(`/api/user/broadcasts/${id}/read`, { method: 'POST' })
}
