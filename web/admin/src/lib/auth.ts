export const ADMIN_TOKEN_KEY = 'llm_gateway_admin_token'

export function getToken(): string {
  if (typeof window === 'undefined') {
    return ''
  }
  return window.sessionStorage.getItem(ADMIN_TOKEN_KEY) ?? window.sessionStorage.getItem('llm_gateway_user_token') ?? ''
}

export function setToken(token: string) {
  if (typeof window === 'undefined') {
    return
  }
  window.localStorage.setItem(ADMIN_TOKEN_KEY, token)
}

export function clearToken() {
  if (typeof window === 'undefined') {
    return
  }
  window.localStorage.removeItem(ADMIN_TOKEN_KEY)
  window.localStorage.removeItem('llm_gateway_user_token')
}

export function hasToken(): boolean {
  return getToken().trim().length > 0
}
