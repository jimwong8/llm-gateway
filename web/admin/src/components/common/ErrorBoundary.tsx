import { Component, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { translateError } from '../../lib/error-translator'

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
  const translated = translateError(error)
  const toneColor =
    translated.tone === 'error'
      ? 'var(--color-danger)'
      : translated.tone === 'warning'
        ? 'var(--color-warning)'
        : 'var(--slate-500)'

  return (
    <div
      className="error-boundary"
      style={{ display: 'grid', placeItems: 'center', minHeight: '60vh', padding: '2rem', textAlign: 'center' }}
    >
      <div>
        <h1 style={{ fontSize: '1.5rem', marginBottom: '0.5rem', color: toneColor }}>
          {translated.title}
        </h1>
        <p style={{ color: 'var(--slate-500)', marginBottom: '1.5rem', fontSize: '0.95rem' }}>
          {translated.body}
        </p>
        {error && (
          <details style={{ marginBottom: '1.5rem', textAlign: 'left', maxWidth: '640px' }}>
            <summary style={{ cursor: 'pointer', color: 'var(--slate-400)', fontSize: '0.8rem' }}>
              原始错误信息
            </summary>
            <pre
              style={{
                background: 'var(--gray-900)',
                color: 'var(--gray-300)',
                padding: '0.75rem',
                borderRadius: '0.5rem',
                fontSize: '0.75rem',
                overflow: 'auto',
                marginTop: '0.5rem',
              }}
            >
              {error.message}
            </pre>
          </details>
        )}
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
