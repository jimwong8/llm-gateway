import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { AppShell } from '../components/layout/AppShell'
import { getGitHubLoginUrl, listOAuthBindings, deleteOAuthBinding } from '../lib/api/identity'
import type { OAuthBinding } from '../types/identity'

export function AccountPage() {
  const { t } = useTranslation()
  const [bindings, setBindings] = useState<OAuthBinding[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  async function fetchBindings() {
    setLoading(true)
    try {
      const res = await listOAuthBindings()
      setBindings(res.data)
    } catch (err: unknown) {
      setError((err as Error)?.message ?? t('account.loadError'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchBindings() }, [])

  async function handleUnlink(provider: string) {
    if (!confirm(t('account.confirmUnlink', { provider }))) return
    try {
      await deleteOAuthBinding(provider)
      setBindings(prev => prev.filter(b => b.provider !== provider))
    } catch (err: unknown) {
      setError((err as Error)?.message ?? t('account.unlinkFailed'))
    }
  }

  return (
    <AppShell title={t('account.title')} description={t('account.description')}>
      <div className="page account-page">
        <h2>{t('account.title')}</h2>

        <section className="card" style={{ marginTop: '1.5rem' }}>
          <h3>{t('account.oauthBindings')}</h3>
          {error && <div className="login-error" role="alert">{error}</div>}
          {loading ? (
            <p>{t('common.loading')}</p>
          ) : bindings.length === 0 ? (
            <p style={{ color: '#94a3b8' }}>{t('account.noBindings')}</p>
          ) : (
            <table className="admin-table" style={{ marginTop: '1rem' }}>
              <thead>
                <tr>
                   <th>{t('account.colProvider')}</th>
                   <th>{t('account.colBoundAt')}</th>
                   <th>{t('account.colActions')}</th>
                </tr>
              </thead>
              <tbody>
                {bindings.map(b => (
                  <tr key={b.id}>
                    <td style={{ textTransform: 'capitalize' }}>{b.provider}</td>
                    <td>{new Date(b.created_at).toLocaleString()}</td>
                    <td>
                      <button
                        className="btn btn--danger btn--sm"
                        onClick={() => handleUnlink(b.provider)}
                      >
                        {t('account.unlink')}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          <div style={{ marginTop: '1.5rem' }}>
            <a
              href={getGitHubLoginUrl()}
              className="btn"
              style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.438 9.8 8.205 11.387.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.73.083-.73 1.205.085 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 21.795 24 17.295 24 12 24 5.37 18.63 0 12 0z"/>
              </svg>
              {t('account.bindGitHub')}
            </a>
          </div>
        </section>
      </div>
    </AppShell>
  )
}
