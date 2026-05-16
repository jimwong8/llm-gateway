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
  return apiRequest<Channel>('/admin/channels', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

export async function updateChannel(id: string, data: UpdateChannelRequest): Promise<Channel> {
  return apiRequest<Channel>(`/admin/channels/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
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
