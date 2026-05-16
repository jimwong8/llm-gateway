import { FormEvent, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { setToken } from '../lib/auth'

type LocationState = {
  from?: {
    pathname?: string
  }
}

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const [token, setTokenValue] = useState('')
  const [error, setError] = useState('')

  const state = location.state as LocationState | null
  const nextPath = state?.from?.pathname ?? '/dashboard'

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
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

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <span className="login-badge">LLM Gateway</span>
          <h1>管理员控制台登录</h1>
          <p>输入管理员 Bearer Token 后进入控制台与在线测试台。</p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
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
      </section>
    </main>
  )
}
