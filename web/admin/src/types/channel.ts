export type ChannelProvider = 'openai' | 'anthropic' | 'google' | 'azure' | 'aws' | 'custom'

export type ChannelStatus = 'active' | 'inactive' | 'error' | 'maintenance'

export type ChannelPriority = 'highest' | 'high' | 'medium' | 'low' | 'lowest'

export interface Channel {
  id: string
  name: string
  provider: ChannelProvider
  status: ChannelStatus
  base_url: string
  api_key: string
  priority: ChannelPriority
  weight: number
  models: string[]
  tags: string[]
  notes?: string
  latency_ms?: number
  total_requests?: number
  created_at: string
  updated_at: string
}

export interface CreateChannelRequest {
  name: string
  provider: ChannelProvider
  base_url: string
  api_key?: string
  priority?: ChannelPriority
  weight?: number
  models?: string[]
  tags?: string[]
  notes?: string
}

export interface UpdateChannelRequest extends Partial<CreateChannelRequest> {
  status?: ChannelStatus
}

export interface ChannelTestResult {
  success: boolean
  latency_ms?: number
  error?: string
  model?: string
}
