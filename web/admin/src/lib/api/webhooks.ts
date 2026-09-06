import { apiRequest, jsonRequest } from './http'
import type { Webhook } from '../types/webhook'

export async function listWebhooks(): Promise<{ data: Webhook[] }> {
  return apiRequest<{ data: Webhook[] }>('/api/webhooks')
}

export async function createWebhook(data: {
  url: string
  events: string[]
  secret?: string
  enabled?: boolean
}): Promise<Webhook> {
  return jsonRequest<Webhook>('/api/webhooks', data)
}

export async function deleteWebhook(id: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/api/webhooks/${id}`, { method: 'DELETE' })
}
