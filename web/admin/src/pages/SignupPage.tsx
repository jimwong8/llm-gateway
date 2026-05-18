import { FormEvent, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { signup, setUserToken } from '../lib/api/identity'

export function SignupPage() {
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
      setError('请填写所有字段')
      return
    }
    if (password.length < 8) {
      setError('密码至少 8 个字符')
      return
    }

    setLoading(true)
    try {
      const res = await signup({ email: email.trim(), username: username.trim(), password })
      setUserToken(res.token)
      navigate('/dashboard', { replace: true })
    } catch (err: any) {
      setError(err?.message ?? '注册失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <span className="login-badge">LLM Gateway</span>
          <h1>创建账号</h1>
          <p>注册后可使用 API Key 管理功能。</p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          <label htmlFor="email">邮箱</label>
          <input id="email" type="email" placeholder="user@example.com" value={email} onChange={(e) => setEmail(e.target.value)} />

          <label htmlFor="username">用户名</label>
          <input id="username" type="text" placeholder="my-username" value={username} onChange={(e) => setUsername(e.target.value)} />

          <label htmlFor="password">密码</label>
          <input id="password" type="password" placeholder="至少 8 个字符" value={password} onChange={(e) => setPassword(e.target.value)} />

          {error ? <div className="login-error" role="alert">{error}</div> : null}
          <button type="submit" disabled={loading}>{loading ? '注册中...' : '注册'}</button>
        </form>

        <p style={{ textAlign: 'center', marginTop: '1rem', color: '#94a3b8', fontSize: '0.85rem' }}>
          已有账号？<Link to="/login">登录</Link>
        </p>
      </section>
    </main>
  )
}
