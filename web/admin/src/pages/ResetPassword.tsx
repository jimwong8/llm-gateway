import { FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { apiRequest } from '../lib/http'

export function ResetPasswordPage() {
  const { t } = useTranslation()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    if (password.length < 8) {
      setError(t('password.minLength'))
      return
    }
    if (password !== confirmPassword) {
      setError(t('password.mismatch'))
      return
    }
    const token = searchParams.get('token')
    if (!token) {
      setError(t('password.invalidLink'))
      return
    }
    setLoading(true)
    try {
      await apiRequest('/api/auth/reset-password', { method: 'POST', body: JSON.stringify({ token, password }), headers: { 'Content-Type': 'application/json' } }, { auth: 'none' })
      setSuccess(true)
      toast.success(t('password.resetSuccessToast'))
      setTimeout(() => navigate('/login'), 2000)
    } catch (err: unknown) {
      const message = (err as Error)?.message ?? t('password.resetFailed')
      setError(message)
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  if (success) {
    return (
      <main className="login-page">
        <section className="login-card">
          <div className="login-card__header">
            <h1>{t('password.resetSuccessTitle')}</h1>
            <p>{t('password.resetSuccessDescription')}</p>
          </div>
        </section>
      </main>
    )
  }

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <span className="login-badge">LLM Gateway</span>
          <h1>{t('password.resetTitle')}</h1>
          <p>{t('password.resetDescription')}</p>
        </div>
        <form className="login-form" onSubmit={handleSubmit}>
          <label>
            {t('password.newPassword')}
            <input type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder={t('password.newPasswordPlaceholder')} />
          </label>
          <label>
            {t('password.confirmPassword')}
            <input type="password" value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} placeholder={t('password.confirmPasswordPlaceholder')} />
          </label>
          {error ? <div className="login-error" role="alert">{error}</div> : null}
          <button type="submit" className="button-primary" disabled={loading}>
            {loading ? t('password.resetting') : t('password.resetAction')}
          </button>
        </form>
      </section>
    </main>
  )
}
