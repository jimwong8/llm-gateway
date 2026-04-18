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
      setError('请输入 Admin Token')
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
          <h1>Admin Console Login</h1>
          <p>输入管理员 Bearer Token 后进入控制台与在线测试台。</p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          <label htmlFor="admin-token">Admin Token</label>
          <textarea
            id="admin-token"
            name="admin-token"
            rows={5}
            placeholder="sk-admin-..."
            value={token}
            onChange={(event) => setTokenValue(event.target.value)}
          />
          {error ? <div className="login-error">{error}</div> : null}
          <button type="submit">进入控制台</button>
        </form>
      </section>
    </main>
  )
}
