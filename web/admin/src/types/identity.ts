export interface User {
  id: number
  email: string
  username: string
  role: number
  status: string
  created_at: string
}

export interface UsageStats {
  total_requests: number
  total_tokens: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_cost: number
  avg_latency_ms: number
}

export interface ApiKey {
  id: number
  name: string
  key_prefix: string
  status: string
  rpm_limit: number
  last_used_at?: string
  created_at: string
  usage?: UsageStats
}

export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  token: string
  user: User
}

export interface SignupRequest {
  email: string
  username: string
  password: string
}

export interface CreateApiKeyRequest {
  name?: string
}
