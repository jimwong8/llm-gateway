import { FormEvent, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { setToken } from '../lib/auth'
import { login, setUserToken } from '../lib/api/identity'

type LocationState = {
  from?: {
    pathname?: string
  }
}

type LoginMode = 'admin' | 'user'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const [mode, setMode] = useState<LoginMode>('admin')
  const [token, setTokenValue] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const state = location.state as LocationState | null
  const nextPath = state?.from?.pathname ?? '/dashboard'

  async function handleAdminSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const normalized = token.trim()
    if (!normalized) {
      setError('请输入管理员 Token')
      return
    }
    if (normalized.length < 4) {
      setError('Token 格式无效，长度至少 4 个字符')
      return
    }
    setToken(normalized)
    setError('')
    navigate(nextPath, { replace: true })
  }

  async function handleUserSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    if (!email.trim() || !password) {
      setError('请填写邮箱和密码')
      return
    }
    setLoading(true)
    try {
      const res = await login({ email: email.trim(), password })
      setUserToken(res.token)
      navigate(nextPath, { replace: true })
    } catch (err: any) {
      setError(err?.message ?? '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <span className="login-badge">LLM Gateway</span>
          <h1>控制台登录</h1>
          <p>使用管理员 Token 或用户账号登录。</p>
        </div>

        <div style={{ display: 'flex', marginBottom: '1.5rem', borderBottom: '2px solid #e2e8f0' }}>
          <button
            type="button"
            style={{
              flex: 1, padding: '0.75rem', border: 'none', background: mode === 'admin' ? '#3b82f6' : 'transparent',
              color: mode === 'admin' ? '#fff' : '#64748b', cursor: 'pointer', fontWeight: 600,
              borderRadius: '4px 4px 0 0',
            }}
            onClick={() => setMode('admin')}
          >
            管理员
          </button>
          <button
            type="button"
            style={{
              flex: 1, padding: '0.75rem', border: 'none', background: mode === 'user' ? '#3b82f6' : 'transparent',
              color: mode === 'user' ? '#fff' : '#64748b', cursor: 'pointer', fontWeight: 600,
              borderRadius: '4px 4px 0 0',
            }}
            onClick={() => setMode('user')}
          >
            用户登录
          </button>
        </div>

        {mode === 'admin' ? (
          <form className="login-form" onSubmit={handleAdminSubmit}>
            <label htmlFor="admin-token">管理员 Token</label>
            <input
              id="admin-token"
              name="admin-token"
              type="password"
              placeholder="sk-admin-..."
              value={token}
              onChange={(event) => setTokenValue(event.target.value)}
            />
            {error ? <div className="login-error" role="alert">{error}</div> : null}
            <button type="submit">进入控制台</button>
          </form>
        ) : (
          <>
            <form className="login-form" onSubmit={handleUserSubmit}>
              <label htmlFor="email">邮箱</label>
              <input id="email" type="email" placeholder="user@example.com" value={email} onChange={(e) => setEmail(e.target.value)} />
              <label htmlFor="password">密码</label>
              <input id="password" type="password" placeholder="输入密码" value={password} onChange={(e) => setPassword(e.target.value)} />
              {error ? <div className="login-error" role="alert">{error}</div> : null}
              <button type="submit" disabled={loading}>{loading ? '登录中...' : '登录'}</button>
            </form>
            <p style={{ textAlign: 'center', marginTop: '1rem', color: '#94a3b8', fontSize: '0.85rem' }}>
              没有账号？<Link to="/signup">注册</Link>
            </p>
          </>
        )}
      </section>
    </main>
  )
}
