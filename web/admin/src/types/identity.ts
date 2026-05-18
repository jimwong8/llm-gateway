export interface User {
  id: number
  email: string
  username: string
  role: number
  status: string
  created_at: string
}

export interface ApiKey {
  id: number
  name: string
  key_prefix: string
  status: string
  last_used_at?: string
  created_at: string
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
