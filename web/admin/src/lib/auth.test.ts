import { ADMIN_TOKEN_KEY, getToken, setToken, clearToken, hasToken } from './auth'

describe('auth', () => {
  beforeEach(() => {
    window.sessionStorage.clear()
  })

  describe('getToken', () => {
    it('returns empty string when no token is set', () => {
      expect(getToken()).toBe('')
    })

    it('returns token from sessionStorage', () => {
      window.sessionStorage.setItem(ADMIN_TOKEN_KEY, 'test-token')
      expect(getToken()).toBe('test-token')
    })
  })

  describe('setToken', () => {
    it('stores token in sessionStorage', () => {
      setToken('my-token')
      expect(window.sessionStorage.getItem(ADMIN_TOKEN_KEY)).toBe('my-token')
    })

    it('overwrites existing token', () => {
      setToken('old-token')
      setToken('new-token')
      expect(getToken()).toBe('new-token')
    })
  })

  describe('clearToken', () => {
    it('removes token from sessionStorage', () => {
      setToken('some-token')
      clearToken()
      expect(getToken()).toBe('')
    })

    it('does nothing when no token exists', () => {
      clearToken()
      expect(getToken()).toBe('')
    })
  })

  describe('hasToken', () => {
    it('returns false when no token is set', () => {
      expect(hasToken()).toBe(false)
    })

    it('returns false for whitespace-only token', () => {
      setToken('   ')
      expect(hasToken()).toBe(false)
    })

    it('returns true when token is set', () => {
      setToken('valid-token')
      expect(hasToken()).toBe(true)
    })
  })
})
