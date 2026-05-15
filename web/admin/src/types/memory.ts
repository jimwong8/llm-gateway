export type MemoryCandidateFact = {
  id: number
  tenant_id: string
  user_id: string
  fact_key: string
  fact_value: string
  source_text: string
  status: string
  source_message_seq: number
  confirmation_count: number
  created_at?: string
  updated_at?: string
}

export type MemoryCandidateFactWire = {
  ID?: number
  TenantID?: string
  UserID?: string
  Key?: string
  Value?: string
  SourceText?: string
  Status?: string
  SourceMessageSeq?: number
  ConfirmationCount?: number
  CreatedAt?: string
  UpdatedAt?: string
  id?: number
  tenant_id?: string
  user_id?: string
  fact_key?: string
  fact_value?: string
  source_text?: string
  status?: string
  source_message_seq?: number
  confirmation_count?: number
  created_at?: string
  updated_at?: string
}

export type MemoryProjectFact = {
  id: number
  tenant_id: string
  user_id: string
  fact_key: string
  fact_value: string
  source_text: string
  status: string
  superseded_by?: number
  source_message_seq: number
  last_verified_at?: string
  created_at?: string
  updated_at?: string
}

export type MemoryProjectFactWire = {
  ID?: number
  TenantID?: string
  UserID?: string
  Key?: string
  Value?: string
  SourceText?: string
  Status?: string
  SupersededBy?: number
  SourceMessageSeq?: number
  LastVerifiedAt?: string
  CreatedAt?: string
  UpdatedAt?: string
  id?: number
  tenant_id?: string
  user_id?: string
  fact_key?: string
  fact_value?: string
  source_text?: string
  status?: string
  superseded_by?: number
  source_message_seq?: number
  last_verified_at?: string
  created_at?: string
  updated_at?: string
}

export type MemoryFactFilters = {
  tenant_id: string
  user_id: string
}

export type MemoryCandidateFactFilters = MemoryFactFilters & {
  status: string
}

export type MemoryProjectFactFilters = MemoryFactFilters & {
  status: string
}

export type MemoryFactAction = 'confirm' | 'reject' | 'promote'

export type MemoryCandidateFactActionRequest = {
  tenant_id: string
  user_id: string
}

export type MemoryCandidateFactBatchActionItem = {
  tenant_id: string
  user_id: string
  fact_key: string
}

export type MemoryCandidateFactBatchActionRequest = {
  items: MemoryCandidateFactBatchActionItem[]
}

export type MemoryCandidateFactBatchActionResultWire = {
  fact_key?: string
  tenant_id?: string
  user_id?: string
  status?: string
  fact?: MemoryCandidateFactWire
  error?: {
    message?: string
    type?: string
  }
}

export type MemoryCandidateFactBatchActionResult = {
  fact_key: string
  tenant_id: string
  user_id: string
  status?: string
  fact?: MemoryCandidateFact
  error?: {
    message: string
    type: string
  }
}

export type MemoryCandidateFactBatchActionResponseWire = {
  action?: string
  success_count?: number
  failure_count?: number
  results?: MemoryCandidateFactBatchActionResultWire[]
}

export type MemoryCandidateFactBatchActionResponse = {
  action: string
  success_count: number
  failure_count: number
  results: MemoryCandidateFactBatchActionResult[]
}

export type MemoryListResponse<T> = {
  object: string
  tenant_id: string
  user_id: string
  status: string
  data: T[]
}
