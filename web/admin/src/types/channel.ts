export type ChannelProvider = 'openai' | 'anthropic' | 'google' | 'azure' | 'aws' | 'custom'

export type ChannelStatus = 'active' | 'inactive' | 'error' | 'maintenance'

export type ChannelPriority = 'highest' | 'high' | 'medium' | 'low' | 'lowest'

export interface Channel {
  id: string
  name: string
  provider: ChannelProvider
  status: ChannelStatus
  baseUrl: string
  apiKey: string
  priority: ChannelPriority
  weight: number
  models: string[]
  tags: string[]
  notes?: string
  latencyMs?: number
  totalRequests?: number
  createdAt: string
  updatedAt: string
}

export interface CreateChannelRequest {
  name: string
  provider: ChannelProvider
  baseUrl: string
  apiKey?: string
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
  latencyMs?: number
  error?: string
  model?: string
}
