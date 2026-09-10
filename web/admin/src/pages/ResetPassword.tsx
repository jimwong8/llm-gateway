import { FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { apiRequest } from '../lib/http'
import { Button, PasswordInput } from '../components/ui'

type PasswordStrength = 'weak' | 'medium' | 'strong'

function checkPasswordStrength(password: string): PasswordStrength {
  if (password.length < 8) return 'weak'
  let score = 0
  if (password.length >= 12) score++
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) score++
  if (/\d/.test(password)) score++
  if (/[^a-zA-Z0-9]/.test(password)) score++
  if (score <= 1) return 'weak'
  if (score <= 2) return 'medium'
  return 'strong'
}

export function ResetPasswordPage() {
  const { t } = useTranslation()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)
  const [countdown, setCountdown] = useState(2)

  const passwordStrength = password ? checkPasswordStrength(password) : null

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')

    if (!password) {
      setError(t('auth.passwordRequired'))
      return
    }
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
      await apiRequest('/api/auth/reset-password', {
        method: 'POST',
        body: JSON.stringify({ token, password }),
        headers: { 'Content-Type': 'application/json' },
      }, { auth: 'none' })
      setSuccess(true)
      toast.success(t('password.resetSuccessToast'))

      // 倒计时跳转
      let remaining = 2
      setCountdown(remaining)
      const timer = setInterval(() => {
        remaining--
        if (remaining <= 0) {
          clearInterval(timer)
          navigate('/login')
        } else {
          setCountdown(remaining)
        }
      }, 1000)
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
        <section className="login-card" aria-label={t('password.resetSuccessTitle')}>
          <div className="login-card__header">
            <h1>{t('password.resetSuccessTitle')}</h1>
            <p>{t('password.resetSuccessDescription')} ({countdown}s)</p>
          </div>
        </section>
      </main>
    )
  }

  const strengthLabels: Record<PasswordStrength, string> = {
    weak: t('signup.strengthWeak'),
    medium: t('signup.strengthMedium'),
    strong: t('signup.strengthStrong'),
  }
  const strengthColors: Record<PasswordStrength, string> = {
    weak: '#ef4444',
    medium: '#f59e0b',
    strong: '#22c55e',
  }

  return (
    <main className="login-page">
      <section className="login-card" aria-label={t('password.resetTitle')}>
        <div className="login-card__header">
          <span className="login-badge">LLM Gateway</span>
          <h1>{t('password.resetTitle')}</h1>
          <p>{t('password.resetDescription')}</p>
        </div>
        <form className="login-form" onSubmit={handleSubmit} noValidate>
          <PasswordInput
            id="new-password"
            label={t('password.newPassword')}
            placeholder={t('password.newPasswordPlaceholder')}
            value={password}
            onChange={e => setPassword(e.target.value)}
            autoComplete="new-password"
            required
          />

          {passwordStrength && password.length >= 8 && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginTop: '-0.5rem' }}>
              <div style={{ flex: 1, height: '4px', background: '#e2e8f0', borderRadius: '2px', overflow: 'hidden' }}>
                <div
                  style={{
                    height: '100%',
                    width: passwordStrength === 'weak' ? '33%' : passwordStrength === 'medium' ? '66%' : '100%',
                    background: strengthColors[passwordStrength],
                    borderRadius: '2px',
                    transition: 'width 0.3s ease, background 0.3s ease',
                  }}
                />
              </div>
              <span style={{ fontSize: '0.78rem', color: strengthColors[passwordStrength], fontWeight: 500 }}>
                {strengthLabels[passwordStrength]}
              </span>
            </div>
          )}

          <PasswordInput
            id="confirm-password"
            label={t('password.confirmPassword')}
            placeholder={t('password.confirmPasswordPlaceholder')}
            value={confirmPassword}
            onChange={e => setConfirmPassword(e.target.value)}
            autoComplete="new-password"
            required
          />

          {confirmPassword && confirmPassword !== password && (
            <p style={{ fontSize: '0.82rem', color: '#ef4444', marginTop: '-0.5rem' }} role="alert">
              {t('password.mismatch')}
            </p>
          )}

          {error ? <div className="login-error" role="alert" aria-live="assertive">{error}</div> : null}

          <Button type="submit" variant="default" size="lg" loading={loading} disabled={loading}>
            {loading ? t('password.resetting') : t('password.resetAction')}
          </Button>
        </form>
      </section>
    </main>
  )
}
