export const ADMIN_TOKEN_KEY = 'llm_gateway_admin_token'

export function getToken(): string {
  if (typeof window === 'undefined') {
    return ''
  }
  return window.sessionStorage.getItem(ADMIN_TOKEN_KEY) ?? ''
}

export function setToken(token: string) {
  if (typeof window === 'undefined') {
    return
  }
  window.sessionStorage.setItem(ADMIN_TOKEN_KEY, token)
}

export function clearToken() {
  if (typeof window === 'undefined') {
    return
  }
  window.sessionStorage.removeItem(ADMIN_TOKEN_KEY)
}

export function hasToken(): boolean {
  return getToken().trim().length > 0
}
