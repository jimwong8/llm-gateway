import { FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'
import { apiRequest } from '../lib/http'

export function ForgotPasswordPage() {
  const { t } = useTranslation()
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    if (!email.trim()) {
      setError(t('password.emailRequired'))
      return
    }
    setLoading(true)
    try {
      await apiRequest('/api/auth/forgot-password', { email: email.trim() }, { auth: 'none', method: 'POST' })
      setSuccess(true)
      toast.success(t('password.resetLinkSentToast'))
    } catch (err: any) {
      setError(err?.message ?? t('password.sendFailed'))
      toast.error(err?.message ?? t('password.sendFailed'))
    } finally {
      setLoading(false)
    }
  }

  if (success) {
    return (
      <main className="login-page">
        <section className="login-card">
          <div className="login-card__header">
            <h1>{t('password.resetLinkSentTitle')}</h1>
            <p>{t('password.resetLinkSentDescription')}</p>
          </div>
          <Link to="/login" className="button-primary" style={{ display: 'block', textAlign: 'center', marginTop: '1rem' }}>
            {t('password.backToLogin')}
          </Link>
        </section>
      </main>
    )
  }

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <span className="login-badge">LLM Gateway</span>
          <h1>{t('password.forgotTitle')}</h1>
          <p>{t('password.forgotDescription')}</p>
        </div>
        <form className="login-form" onSubmit={handleSubmit}>
          <label>
            {t('auth.email')}
            <input type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder={t('auth.emailPlaceholder')} />
          </label>
          {error ? <div className="login-error" role="alert">{error}</div> : null}
          <button type="submit" className="button-primary" disabled={loading}>
            {loading ? t('password.sending') : t('password.sendResetLink')}
          </button>
        </form>
        <p style={{ textAlign: 'center', marginTop: '1rem' }}>
          <Link to="/login">{t('password.backToLogin')}</Link>
        </p>
      </section>
    </main>
  )
}
