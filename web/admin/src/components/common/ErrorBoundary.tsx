import { Component, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'

type Props = { children: ReactNode }
type State = { hasError: boolean; error: Error | null }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      return <ErrorFallback error={this.state.error} onReset={this.handleReset} />
    }
    return this.props.children
  }
}

function ErrorFallback({ error, onReset }: { error: Error | null; onReset: () => void }) {
  const navigate = useNavigate()
  return (
    <div className="error-boundary" style={{ display: 'grid', placeItems: 'center', minHeight: '60vh', padding: '2rem', textAlign: 'center' }}>
      <div>
        <h1 style={{ fontSize: '1.5rem', marginBottom: '1rem' }}>页面出现错误</h1>
        <p style={{ color: 'var(--slate-500)', marginBottom: '1.5rem' }}>
          {error?.message ?? '未知错误'}
        </p>
        <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'center' }}>
          <button type="button" onClick={onReset} className="btn btn--primary">
            重试
          </button>
          <button type="button" onClick={() => navigate('/dashboard')} className="btn">
            返回首页
          </button>
        </div>
      </div>
    </div>
  )
}
