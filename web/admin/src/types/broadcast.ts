export type BroadcastType = 'info' | 'warning' | 'critical'

export type Broadcast = {
  id: number
  title: string
  content: string
  type: BroadcastType
  start_at: string
  end_at: string
  created_by: string
  created_at: string
  updated_at: string
}

export type BroadcastInput = {
  title: string
  content: string
  type: BroadcastType
  start_at: string
  end_at: string
}
