import { FormEvent, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { apiRequest } from '../lib/http'

export function ResetPasswordPage() {
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
      setError('密码至少 8 个字符')
      return
    }
    if (password !== confirmPassword) {
      setError('两次输入的密码不一致')
      return
    }
    const token = searchParams.get('token')
    if (!token) {
      setError('无效的重置链接')
      return
    }
    setLoading(true)
    try {
      await apiRequest('/api/auth/reset-password', { token, password }, { auth: 'none', method: 'POST' })
      setSuccess(true)
      setTimeout(() => navigate('/login'), 2000)
    } catch (err: any) {
      setError(err?.message ?? '重置失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }

  if (success) {
    return (
      <main className="login-page">
        <section className="login-card">
          <div className="login-card__header">
            <h1>密码重置成功</h1>
            <p>您的密码已更新，即将跳转到登录页面...</p>
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
          <h1>重置密码</h1>
          <p>输入新密码</p>
        </div>
        <form className="login-form" onSubmit={handleSubmit}>
          <label>
            新密码
            <input type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="至少 8 个字符" />
          </label>
          <label>
            确认密码
            <input type="password" value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} placeholder="再次输入新密码" />
          </label>
          {error ? <div className="login-error" role="alert">{error}</div> : null}
          <button type="submit" className="button-primary" disabled={loading}>
            {loading ? '重置中...' : '重置密码'}
          </button>
        </form>
      </section>
    </main>
  )
}
