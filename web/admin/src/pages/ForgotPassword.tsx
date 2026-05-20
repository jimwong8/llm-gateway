import { FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'
import { apiRequest } from '../lib/http'
import { Button, Input } from '../components/ui'

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

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
    if (!EMAIL_REGEX.test(email.trim())) {
      setError(t('password.emailInvalid'))
      return
    }

    setLoading(true)
    try {
      await apiRequest('/api/auth/forgot-password', {
        method: 'POST',
        body: JSON.stringify({ email: email.trim() }),
        headers: { 'Content-Type': 'application/json' },
      }, { auth: 'none' })
      setSuccess(true)
      toast.success(t('password.resetLinkSentToast'))
    } catch (err: unknown) {
      const message = (err as Error)?.message ?? t('password.sendFailed')
      setError(message)
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  if (success) {
    return (
      <main className="login-page">
        <section className="login-card" aria-label={t('password.resetLinkSentTitle')}>
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
      <section className="login-card" aria-label={t('password.forgotTitle')}>
        <div className="login-card__header">
          <span className="login-badge">LLM Gateway</span>
          <h1>{t('password.forgotTitle')}</h1>
          <p>{t('password.forgotDescription')}</p>
        </div>
        <form className="login-form" onSubmit={handleSubmit} noValidate>
          <Input
            id="email"
            label={t('auth.email')}
            type="email"
            placeholder={t('auth.emailPlaceholder')}
            value={email}
            onChange={e => setEmail(e.target.value)}
            error={error}
            autoComplete="email"
            required
          />
          <Button type="submit" variant="primary" size="lg" loading={loading} disabled={loading}>
            {loading ? t('password.sending') : t('password.sendResetLink')}
          </Button>
        </form>
        <p style={{ textAlign: 'center', marginTop: '1rem' }}>
          <Link to="/login">{t('password.backToLogin')}</Link>
        </p>
      </section>
    </main>
  )
}
