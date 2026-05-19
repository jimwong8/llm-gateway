import { FormEvent, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { setToken } from '../lib/auth'
import { apiRequest } from '../lib/http'
import { getGitHubLoginUrl, login, setUserToken } from '../lib/api/identity'
import { Button, Input, Tabs } from '../components/ui'

type LocationState = {
  from?: {
    pathname?: string
  }
}

type LoginMode = 'admin' | 'user'



export function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const [mode, setMode] = useState<LoginMode>('admin')
  const [token, setTokenValue] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [githubEnabled, setGitHubEnabled] = useState(false)

  const MODE_TABS = [
    { key: 'admin', label: t('auth.admin') },
    { key: 'user', label: t('auth.user') },
  ]

  const state = location.state as LocationState | null
  const nextPath = state?.from?.pathname ?? '/dashboard'

  useEffect(() => {
    const params = new URLSearchParams(location.search)
    const tokenParam = params.get('token')
    if (tokenParam) {
      setUserToken(tokenParam)
      navigate(nextPath, { replace: true })
      return
    }

    apiRequest<{ github_enabled: boolean }>('/api/auth/oauth/config', {}, { auth: 'none' })
      .then(cfg => setGitHubEnabled(cfg.github_enabled))
      .catch(() => {})
  }, [])

  async function handleAdminSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const normalized = token.trim()
    if (!normalized) {
      setError(t('auth.adminTokenRequired'))
      return
    }
    if (normalized.length < 4) {
      setError(t('auth.tokenFormatInvalid'))
      return
    }
    setToken(normalized)
    setError('')
    navigate(nextPath, { replace: true })
  }

  async function handleUserSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    if (!email.trim() || !password) {
      setError(t('auth.emailPasswordRequired'))
      return
    }
    setLoading(true)
    try {
      const res = await login({ email: email.trim(), password })
      setUserToken(res.token)
      navigate(nextPath, { replace: true })
    } catch (err: unknown) {
      setError((err as Error)?.message ?? t('auth.loginFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <span className="login-badge">LLM Gateway</span>
          <h1>{t('auth.loginTitle')}</h1>
          <p>{t('auth.loginSubtitle')}</p>
        </div>

        <div style={{ marginBottom: '1.5rem' }}>
          <Tabs tabs={MODE_TABS} activeKey={mode} onChange={(key) => { setMode(key as LoginMode); setError('') }} />
        </div>

        {mode === 'admin' ? (
          <form className="login-form" onSubmit={handleAdminSubmit}>
            <Input
              id="admin-token"
              label={t('auth.adminToken')}
              type="password"
              placeholder={t('auth.adminTokenPlaceholder')}
              value={token}
              onChange={(event) => setTokenValue(event.target.value)}
              error={error}
            />
            <Button type="submit" variant="primary" size="lg">{t('auth.enterConsole')}</Button>
          </form>
        ) : (
          <>
            <form className="login-form" onSubmit={handleUserSubmit}>
              <Input label={t('auth.email')} id="email" type="email" placeholder={t('auth.emailPlaceholder')} value={email} onChange={(e) => setEmail(e.target.value)} />
              <Input label={t('auth.password')} id="password" type="password" placeholder={t('auth.passwordPlaceholder')} value={password} onChange={(e) => setPassword(e.target.value)} />
              {error ? (
                <div className="login-error" role="alert">{error}</div>
              ) : null}
              <Button type="submit" variant="primary" size="lg" loading={loading} disabled={loading}>
                {loading ? t('auth.loggingIn') : t('auth.login')}
              </Button>
            </form>
            {githubEnabled && (
              <div style={{ marginTop: '1rem', textAlign: 'center' }}>
                <div style={{ color: '#94a3b8', fontSize: '0.85rem', marginBottom: '0.5rem' }}>{t('auth.or')}</div>
                <a
                  href={getGitHubLoginUrl()}
                  style={{
                    display: 'inline-flex', alignItems: 'center', gap: '0.5rem',
                    padding: '0.6rem 1.2rem', borderRadius: '6px', border: '1px solid #d1d5db',
                    color: '#374151', background: '#fff', textDecoration: 'none',
                    fontWeight: 500, fontSize: '0.9rem',
                  }}
                >
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.438 9.8 8.205 11.387.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.73.083-.73 1.205.085 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 21.795 24 17.295 24 12 24 5.37 18.63 0 12 0z"/>
                  </svg>
                  {t('auth.githubLogin')}
                </a>
              </div>
            )}
            <p style={{ textAlign: 'center', marginTop: '0.5rem' }}>
              <Link to="/forgot-password" style={{ color: '#94a3b8', fontSize: '0.85rem' }}>忘记密码？</Link>
            </p>
            <p style={{ textAlign: 'center', marginTop: '0.5rem', color: '#94a3b8', fontSize: '0.85rem' }}>
              {t('auth.noAccount')}<Link to="/signup">{t('auth.register')}</Link>
            </p>
          </>
        )}
      </section>
    </main>
  )
}
