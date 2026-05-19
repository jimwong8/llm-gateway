import type { ApiKey, CreateApiKeyRequest, LoginRequest, LoginResponse, OAuthBinding, SignupRequest, User } from '../../types/identity'
import { apiRequest, jsonRequest } from '../http'

const USER_TOKEN_KEY = 'llm_gateway_user_token'

export function getUserToken(): string {
  if (typeof window === 'undefined') return ''
  return window.sessionStorage.getItem(USER_TOKEN_KEY) ?? ''
}

export function setUserToken(token: string) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(USER_TOKEN_KEY, token)
}

export function clearUserToken() {
  if (typeof window === 'undefined') return
  window.sessionStorage.removeItem(USER_TOKEN_KEY)
  window.sessionStorage.removeItem('llm_gateway_admin_token')
}

export function hasUserToken(): boolean {
  return getUserToken().trim().length > 0
}

export async function signup(data: SignupRequest): Promise<LoginResponse> {
  return jsonRequest<LoginResponse>('/api/auth/signup', data, { method: 'POST' }, { auth: 'none' })
}

export async function login(data: LoginRequest): Promise<LoginResponse> {
  return jsonRequest<LoginResponse>('/api/auth/login', data, { method: 'POST' }, { auth: 'none' })
}

function authHeaders(): HeadersInit {
  const token = getUserToken()
  if (token) {
    return { 'Authorization': `Bearer ${token}` }
  }
  return {}
}

export async function getMe(): Promise<User> {
  return apiRequest<User>('/api/auth/me', {
    method: 'GET',
    headers: authHeaders(),
  }, { auth: 'none' })
}

export async function listApiKeys(): Promise<{ object: string; data: ApiKey[] }> {
  return apiRequest<{ object: string; data: ApiKey[] }>('/api/user/api-keys', {
    method: 'GET',
    headers: authHeaders(),
  }, { auth: 'none' })
}

export async function createApiKey(data?: CreateApiKeyRequest): Promise<{ key: string; api_key: ApiKey }> {
  return jsonRequest<{ key: string; api_key: ApiKey }>('/api/user/api-keys', data || {}, { method: 'POST', headers: authHeaders() }, { auth: 'none' })
}

export async function revokeApiKey(id: number): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/api/user/api-keys/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  }, { auth: 'none' })
}

export function getGitHubLoginUrl(): string {
  return '/api/auth/oauth/github'
}

export async function listOAuthBindings(): Promise<{ object: string; data: OAuthBinding[] }> {
  return apiRequest('/api/user/oauth', {
    method: 'GET',
    headers: authHeaders(),
  }, { auth: 'none' })
}

export async function deleteOAuthBinding(provider: string): Promise<{ status: string }> {
  return apiRequest(`/api/user/oauth/${provider}`, {
    method: 'DELETE',
    headers: authHeaders(),
  }, { auth: 'none' })
}
