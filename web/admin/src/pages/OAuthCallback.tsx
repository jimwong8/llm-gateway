import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { setUserToken } from '../lib/api/identity'

export function OAuthCallbackPage() {
  const { t } = useTranslation()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()

  useEffect(() => {
    const token = searchParams.get('token')
    if (token) {
      setUserToken(token)
      navigate('/dashboard', { replace: true })
    } else {
      navigate('/login', { replace: true })
    }
  }, [searchParams, navigate])

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <h1>{t('oauth.loggingIn')}</h1>
          <p>{t('oauth.redirecting')}</p>
        </div>
      </section>
    </main>
  )
}
