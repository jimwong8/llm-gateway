/** 候选事实 — 从对话中提取的待审事实 */
export type MemoryCandidateFact = {
  id: number              // 记录 ID
  tenant_id: string       // 租户 ID
  user_id: string         // 用户 ID
  fact_key: string        // 事实键
  fact_value: string      // 事实值
  source_text: string     // 来源摘录文本
  status: string          // 状态: pending / confirmed / promoted / rejected
  source_message_seq: number  // 来源消息序号
  confirmation_count: number  // 确认次数
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

/** 项目事实 — 已提升到项目中的持久化事实 */
export type MemoryProjectFact = {
  id: number                // 记录 ID
  tenant_id: string         // 租户 ID
  user_id: string           // 用户 ID
  fact_key: string          // 事实键
  fact_value: string        // 事实值
  source_text: string       // 来源摘录文本
  status: string            // 状态: active / superseded
  superseded_by?: number    // 被哪个新事实取代
  source_message_seq: number    // 来源消息序号
  last_verified_at?: string     // 最后验证时间
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

/** Hybrid Search 请求 */
export type MemorySearchRequest = {
  query: string
  tenant_id?: string
  user_id?: string
  limit?: number
}

/** Hybrid Search 单条结果 */
export type MemorySearchResult = {
  content: string
  score: number
  source: string
  rank: number
  fact_key?: string
  tenant_id?: string
  user_id?: string
}

/** Hybrid Search 响应 */
export type MemorySearchResponse = {
  query: string
  results: MemorySearchResult[]
}
