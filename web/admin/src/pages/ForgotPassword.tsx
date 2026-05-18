import { FormEvent, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiRequest } from '../lib/http'

export function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    if (!email.trim()) {
      setError('请输入邮箱地址')
      return
    }
    setLoading(true)
    try {
      await apiRequest('/api/auth/forgot-password', { email: email.trim() }, { auth: 'none', method: 'POST' })
      setSuccess(true)
    } catch (err: any) {
      setError(err?.message ?? '发送失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }

  if (success) {
    return (
      <main className="login-page">
        <section className="login-card">
          <div className="login-card__header">
            <h1>重置链接已发送</h1>
            <p>如果您的邮箱已注册，我们将发送密码重置链接。请检查您的收件箱。</p>
          </div>
          <Link to="/login" className="button-primary" style={{ display: 'block', textAlign: 'center', marginTop: '1rem' }}>
            返回登录
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
          <h1>找回密码</h1>
          <p>输入您的邮箱地址，我们将发送重置链接</p>
        </div>
        <form className="login-form" onSubmit={handleSubmit}>
          <label>
            邮箱
            <input type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="your@email.com" />
          </label>
          {error ? <div className="login-error" role="alert">{error}</div> : null}
          <button type="submit" className="button-primary" disabled={loading}>
            {loading ? '发送中...' : '发送重置链接'}
          </button>
        </form>
        <p style={{ textAlign: 'center', marginTop: '1rem' }}>
          <Link to="/login">返回登录</Link>
        </p>
      </section>
    </main>
  )
}
