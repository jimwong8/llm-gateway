import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { setUserToken } from '../lib/api/identity'

export function OAuthCallbackPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()

  useEffect(() => {
    const token = searchParams.get('token')
    if (token) {
      setUserToken(token)
      navigate('/dashboard', { replace: true })
    } else {
      navigate('/login', { replace: true })
    }
  }, [searchParams, navigate])

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-card__header">
          <h1>登录中...</h1>
          <p>正在完成 GitHub 授权，请稍候。</p>
        </div>
      </section>
    </main>
  )
}
