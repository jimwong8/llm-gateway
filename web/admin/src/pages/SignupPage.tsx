import { FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router-dom'
import { signup, setUserToken } from '../lib/api/identity'
import { Button, Input, PasswordInput } from '../components/ui'

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const USERNAME_REGEX = /^[a-zA-Z0-9_\-\u4e00-\u9fa5]{2,32}$/

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

export function SignupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const passwordStrength = password ? checkPasswordStrength(password) : null

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')

    if (!email.trim()) {
      setError(t('signup.emailRequired'))
      return
    }
    if (!EMAIL_REGEX.test(email.trim())) {
      setError(t('signup.emailInvalid'))
      return
    }
    if (!username.trim()) {
      setError(t('signup.usernameRequired'))
      return
    }
    if (!USERNAME_REGEX.test(username.trim())) {
      setError(t('signup.usernameInvalid'))
      return
    }
    if (!password) {
      setError(t('signup.passwordRequired'))
      return
    }
    if (password.length < 8) {
      setError(t('signup.passwordMinLength'))
      return
    }
    if (password !== confirmPassword) {
      setError(t('signup.passwordMismatch'))
      return
    }

    setLoading(true)
    try {
      const res = await signup({ email: email.trim(), username: username.trim(), password })
      setUserToken(res.token)
      navigate('/dashboard', { replace: true })
    } catch (err: unknown) {
      setError((err as Error)?.message ?? t('signup.signupFailed'))
    } finally {
      setLoading(false)
    }
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
      <section className="login-card" aria-label={t('signup.title')}>
        <div className="login-card__header">
          <span className="login-badge">{t('app.brand')}</span>
          <h1>{t('signup.title')}</h1>
          <p>{t('signup.subtitle')}</p>
        </div>

        <form className="login-form" onSubmit={handleSubmit} noValidate>
          <Input
            id="email"
            label={t('signup.email')}
            type="email"
            placeholder={t('signup.emailPlaceholder')}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            required
          />

          <Input
            id="username"
            label={t('signup.username')}
            type="text"
            placeholder={t('signup.usernamePlaceholder')}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
          />

          <PasswordInput
            id="password"
            label={t('signup.password')}
            placeholder={t('signup.passwordPlaceholder')}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
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
            label={t('signup.confirmPassword')}
            placeholder={t('signup.confirmPasswordPlaceholder')}
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            autoComplete="new-password"
            required
          />

          {confirmPassword && confirmPassword !== password && (
            <p style={{ fontSize: '0.82rem', color: '#ef4444', marginTop: '-0.5rem' }} role="alert">
              {t('signup.passwordMismatchInline')}
            </p>
          )}

          {error ? <div className="login-error" role="alert" aria-live="assertive">{error}</div> : null}

          <Button type="submit" variant="default" size="lg" loading={loading} disabled={loading}>
            {loading ? t('signup.signingUp') : t('signup.signup')}
          </Button>
        </form>

        <p style={{ textAlign: 'center', marginTop: '1rem', color: '#94a3b8', fontSize: '0.85rem' }}>
          {t('signup.hasAccount')}<Link to="/login">{t('auth.login')}</Link>
        </p>
      </section>
    </main>
  )
}
