import { FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router-dom'
import { signup, setUserToken } from '../lib/api/identity'

export function SignupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')

    if (!email.trim() || !username.trim() || !password) {
      setError(t('signup.allFieldsRequired'))
      return
    }
    if (password.length < 8) {
      setError(t('signup.passwordMinLength'))
      return
    }

    setLoading(true)
    try {
      const res = await signup({ email: email.trim(), username: username.trim(), password })
      setUserToken(res.token)
      navigate('/dashboard', { replace: true })
    } catch (err: any) {
      setError(err?.message ?? t('signup.signupFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <span className="login-badge">{t('app.brand')}</span>
          <h1>{t('signup.title')}</h1>
          <p>{t('signup.subtitle')}</p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          <label htmlFor="email">{t('signup.email')}</label>
          <input id="email" type="email" placeholder={t('signup.emailPlaceholder')} value={email} onChange={(e) => setEmail(e.target.value)} />

          <label htmlFor="username">{t('signup.username')}</label>
          <input id="username" type="text" placeholder={t('signup.usernamePlaceholder')} value={username} onChange={(e) => setUsername(e.target.value)} />

          <label htmlFor="password">{t('signup.password')}</label>
          <input id="password" type="password" placeholder={t('signup.passwordPlaceholder')} value={password} onChange={(e) => setPassword(e.target.value)} />

          {error ? <div className="login-error" role="alert">{error}</div> : null}
          <button type="submit" disabled={loading}>{loading ? t('signup.signingUp') : t('signup.signup')}</button>
        </form>

        <p style={{ textAlign: 'center', marginTop: '1rem', color: '#94a3b8', fontSize: '0.85rem' }}>
          {t('signup.hasAccount')}<Link to="/login">{t('auth.login')}</Link>
        </p>
      </section>
    </main>
  )
}
