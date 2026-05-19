import { apiRequest } from './http'
import type { Channel, ChannelTestResult, CreateChannelRequest, UpdateChannelRequest, ChannelStatus } from '../types/channel'

export async function listChannels(): Promise<Channel[]> {
  const resp = await apiRequest<{ object: string; data: Channel[] }>('/admin/channels')
  return resp.data ?? []
}

export async function getChannel(id: string): Promise<Channel> {
  return apiRequest<Channel>(`/admin/channels/${id}`)
}

export async function createChannel(data: CreateChannelRequest): Promise<Channel> {
  const body: Record<string, unknown> = {
    name: data.name,
    provider: data.provider,
    base_url: data.base_url,
    api_key: data.api_key,
    priority: data.priority,
    weight: data.weight,
    models: data.models,
    tags: data.tags,
    notes: data.notes,
  }
  return apiRequest<Channel>('/admin/channels', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export async function updateChannel(id: string, data: UpdateChannelRequest): Promise<Channel> {
  const body: Record<string, unknown> = {}
  if (data.name !== undefined) body.name = data.name
  if (data.provider !== undefined) body.provider = data.provider
  if (data.base_url !== undefined) body.base_url = data.base_url
  if (data.api_key !== undefined) body.api_key = data.api_key
  if (data.priority !== undefined) body.priority = data.priority
  if (data.weight !== undefined) body.weight = data.weight
  if (data.models !== undefined) body.models = data.models
  if (data.tags !== undefined) body.tags = data.tags
  if (data.notes !== undefined) body.notes = data.notes
  if (data.status !== undefined) body.status = data.status
  return apiRequest<Channel>(`/admin/channels/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export async function deleteChannel(id: string): Promise<void> {
  await apiRequest<void>(`/admin/channels/${id}`, { method: 'DELETE' })
}

export async function batchDeleteChannels(ids: string[]): Promise<void> {
  await apiRequest<void>('/admin/channels/batch-delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  })
}

export async function batchUpdateChannelsStatus(ids: string[], status: ChannelStatus): Promise<void> {
  await apiRequest<void>('/admin/channels/batch-status', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids, status }),
  })
}

export async function testChannel(id: string): Promise<ChannelTestResult> {
  return apiRequest<ChannelTestResult>(`/admin/channels/${id}/test`, { method: 'POST' })
}
