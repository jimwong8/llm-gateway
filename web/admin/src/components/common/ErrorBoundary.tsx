import { Component, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'

type Props = { children: ReactNode }
type State = { hasError: boolean; error: Error | null; errorInfo: string | null }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null, errorInfo: null }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    this.setState({ errorInfo: info.componentStack || null })
    console.error('ErrorBoundary caught:', error, info)
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null, errorInfo: null })
  }

  handleCopyError = () => {
    const text = `${this.state.error?.name}: ${this.state.error?.message}\n\n${this.state.errorInfo || ''}`
    navigator.clipboard.writeText(text).catch(() => {})
  }

  render() {
    if (this.state.hasError) {
      return (
        <ErrorFallback 
          error={this.state.error} 
          errorInfo={this.state.errorInfo}
          onReset={this.handleReset}
          onCopy={this.handleCopyError}
        />
      )
    }
    return this.props.children
  }
}

function getErrorGuidance(error: Error | null): { title: string; suggestion: string; actions: string[] } {
  if (!error) return { title: '未知错误', suggestion: '发生了意外错误，请重试。', actions: [] }
  
  const msg = error.message.toLowerCase()
  
  if (msg.includes('401') || msg.includes('unauthorized')) {
    return {
      title: '登录已过期',
      suggestion: '您的登录凭证已失效，请重新登录。',
      actions: ['login']
    }
  }
  
  if (msg.includes('403') || msg.includes('forbidden')) {
    return {
      title: '权限不足',
      suggestion: '您没有执行此操作的权限，请联系管理员。',
      actions: []
    }
  }
  
  if (msg.includes('404') || msg.includes('not found')) {
    return {
      title: '资源不存在',
      suggestion: '请求的资源可能已被删除或不存在。',
      actions: ['back']
    }
  }
  
  if (msg.includes('500') || msg.includes('internal')) {
    return {
      title: '服务器错误',
      suggestion: '服务器暂时无法处理请求，请稍后重试。',
      actions: ['retry']
    }
  }
  
  if (msg.includes('network') || msg.includes('failed to fetch')) {
    return {
      title: '网络连接失败',
      suggestion: '请检查您的网络连接，确保可以访问服务器。',
      actions: ['retry']
    }
  }
  
  if (msg.includes('timeout')) {
    return {
      title: '请求超时',
      suggestion: '服务器响应时间过长，请稍后重试。',
      actions: ['retry']
    }
  }
  
  return {
    title: '发生错误',
    suggestion: error.message || '发生了意外错误',
    actions: ['retry']
  }
}

function ErrorFallback({ 
  error, 
  errorInfo,
  onReset, 
  onCopy 
}: { 
  error: Error | null
  errorInfo: string | null
  onReset: () => void
  onCopy: () => void
}) {
  const navigate = useNavigate()
  const guidance = getErrorGuidance(error)
  const [showDetails, setShowDetails] = useState(false)

  return (
    <div 
      className="error-boundary" 
      style={{ 
        display: 'grid', 
        placeItems: 'center', 
        minHeight: '60vh', 
        padding: '2rem', 
        textAlign: 'center' 
      }}
      role="alert"
      aria-live="assertive"
    >
      <div style={{ maxWidth: '500px' }}>
        <div style={{ fontSize: '3rem', marginBottom: '1rem' }} aria-hidden="true">
          {guidance.actions.includes('login') ? '🔒' : 
           guidance.actions.includes('retry') ? '⚠️' : '❌'}
        </div>
        
        <h1 style={{ fontSize: '1.25rem', fontWeight: 600, marginBottom: '0.75rem', color: 'var(--text-primary)' }}>
          {guidance.title}
        </h1>
        
        <p style={{ color: 'var(--text-secondary)', marginBottom: '1.5rem', lineHeight: 1.6 }}>
          {guidance.suggestion}
        </p>
        
        <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'center', flexWrap: 'wrap' }}>
          {guidance.actions.includes('retry') && (
            <button 
              type="button" 
              onClick={onReset} 
              className="btn btn-primary"
              style={{
                padding: '0.6rem 1.5rem',
                borderRadius: 'var(--radius-md)',
                background: 'var(--color-primary)',
                color: '#fff',
                border: 'none',
                cursor: 'pointer',
                fontWeight: 500
              }}
            >
              重试
            </button>
          )}
          
          {guidance.actions.includes('login') && (
            <button 
              type="button" 
              onClick={() => navigate('/login')} 
              className="btn btn-primary"
              style={{
                padding: '0.6rem 1.5rem',
                borderRadius: 'var(--radius-md)',
                background: 'var(--color-primary)',
                color: '#fff',
                border: 'none',
                cursor: 'pointer',
                fontWeight: 500
              }}
            >
              前往登录
            </button>
          )}
          
          <button 
            type="button" 
            onClick={() => navigate('/dashboard')} 
            className="btn btn-secondary"
            style={{
              padding: '0.6rem 1.5rem',
              borderRadius: 'var(--radius-md)',
              background: 'var(--bg-tertiary)',
              color: 'var(--text-primary)',
              border: '1px solid var(--border-default)',
              cursor: 'pointer'
            }}
          >
            返回首页
          </button>
        </div>

        {/* 错误详情展开 */}
        <div style={{ marginTop: '2rem', textAlign: 'left' }}>
          <button
            type="button"
            onClick={() => setShowDetails(!showDetails)}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--text-tertiary)',
              cursor: 'pointer',
              fontSize: '0.8rem',
              display: 'flex',
              alignItems: 'center',
              gap: '0.5rem',
              margin: '0 auto'
            }}
            aria-expanded={showDetails}
          >
            {showDetails ? '▼' : '▶'} 技术详情
          </button>
          
          {showDetails && (
            <div 
              style={{ 
                marginTop: '0.75rem',
                padding: '1rem',
                background: 'var(--bg-tertiary)',
                borderRadius: 'var(--radius-md)',
                fontSize: '0.75rem',
                fontFamily: 'var(--font-mono)',
                color: 'var(--text-tertiary)',
                maxHeight: '200px',
                overflow: 'auto',
                position: 'relative'
              }}
            >
              <button
                type="button"
                onClick={onCopy}
                style={{
                  position: 'absolute',
                  top: '0.5rem',
                  right: '0.5rem',
                  background: 'var(--bg-secondary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-sm)',
                  padding: '0.25rem 0.5rem',
                  fontSize: '0.7rem',
                  cursor: 'pointer',
                  color: 'var(--text-secondary)'
                }}
              >
                复制
              </button>
              <strong>{error?.name}:</strong> {error?.message}
              {errorInfo && (
                <pre style={{ marginTop: '0.75rem', whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                  {errorInfo}
                </pre>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

import { useState } from 'react'
