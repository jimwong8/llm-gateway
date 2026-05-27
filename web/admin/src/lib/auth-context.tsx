import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react'

const ADMIN_TOKEN_KEY = 'llm_gateway_admin_token'
const USER_TOKEN_KEY = 'llm_gateway_user_token'

type AuthState = {
  adminToken: string
  userToken: string
  isAuthenticated: boolean
}

type AuthContextValue = AuthState & {
  setAdminToken: (token: string) => void
  setUserToken: (token: string) => void
  clearAllTokens: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [adminToken, setAdminTokenState] = useState(() =>
    typeof window !== 'undefined' ? sessionStorage.getItem(ADMIN_TOKEN_KEY) ?? '' : ''
  )
  const [userToken, setUserTokenState] = useState(() =>
    typeof window !== 'undefined' ? sessionStorage.getItem(USER_TOKEN_KEY) ?? '' : ''
  )

  const setAdminToken = useCallback((token: string) => {
    sessionStorage.setItem(ADMIN_TOKEN_KEY, token)
    setAdminTokenState(token)
  }, [])

  const setUserToken = useCallback((token: string) => {
    sessionStorage.setItem(USER_TOKEN_KEY, token)
    setUserTokenState(token)
  }, [])

  const clearAllTokens = useCallback(() => {
    sessionStorage.removeItem(ADMIN_TOKEN_KEY)
    sessionStorage.removeItem(USER_TOKEN_KEY)
    setAdminTokenState('')
    setUserTokenState('')
  }, [])

  useEffect(() => {
    const handler = () => {
      clearAllTokens()
      window.location.href = '/admin/ui/login'
    }
    window.addEventListener('auth:expired', handler)
    return () => window.removeEventListener('auth:expired', handler)
  }, [clearAllTokens])

  const isAuthenticated = adminToken.length > 0 || userToken.length > 0

  return (
    <AuthContext.Provider value={{ adminToken, userToken, isAuthenticated, setAdminToken, setUserToken, clearAllTokens }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
