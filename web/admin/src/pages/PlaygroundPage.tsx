import { FormEvent, useMemo, useState } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { sendPlaygroundRequest, type PlaygroundResult } from '../lib/playground'
import type { PlaygroundMessage, PlaygroundRequest } from '../types/playground'

type RequestState = {
  model: string
  tenantID: string
  taskHint: string
  messages: PlaygroundMessage[]
}

const initialRequestState: RequestState = {
  model: 'gpt-4o-mini',
  tenantID: 'tenant-a',
  taskHint: '',
  messages: [{ role: 'user', content: '请解释一下当前配置会如何影响路由决策。' }],
}

type RecentRequest = {
  id: string
  model: string
  tenantID: string
  messagePreview: string
  status: number
  elapsedMs: number
}

export function PlaygroundPage() {
  const [requestState, setRequestState] = useState<RequestState>(initialRequestState)
  const [result, setResult] = useState<PlaygroundResult | null>(null)
  const [recentRequests, setRecentRequests] = useState<RecentRequest[]>([])
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const requestPreview = useMemo<PlaygroundRequest>(
    () => ({
      model: requestState.model,
      tenant_id: requestState.tenantID,
      task_hint: requestState.taskHint || undefined,
      messages: requestState.messages.filter((message) => message.content.trim() !== ''),
    }),
    [requestState],
  )

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!requestState.model.trim() || !requestState.tenantID.trim() || requestPreview.messages.length === 0) {
      setError('请填写 model、tenant_id，并至少提供一条消息')
      return
    }

    setSubmitting(true)
    setError('')

    try {
      const response = await sendPlaygroundRequest(requestPreview)
      setResult(response)
      setRecentRequests((previous) => [
        {
          id: `${Date.now()}`,
          model: requestPreview.model,
          tenantID: requestPreview.tenant_id,
          messagePreview: requestPreview.messages[0]?.content ?? '—',
          status: response.status,
          elapsedMs: response.elapsedMs,
        },
        ...previous,
      ].slice(0, 5))
    } catch (unknownError) {
      setError(unknownError instanceof Error ? unknownError.message : '请求发送失败')
    } finally {
      setSubmitting(false)
    }
  }

  function updateMessage(index: number, field: keyof PlaygroundMessage, value: string) {
    setRequestState((prev) => ({
      ...prev,
      messages: prev.messages.map((message, currentIndex) =>
        currentIndex === index ? { ...message, [field]: value } : message,
      ),
    }))
  }

  function addMessage() {
    setRequestState((prev) => ({
      ...prev,
      messages: [...prev.messages, { role: 'user', content: '' }],
    }))
  }

  return (
    <AppShell
      title="Playground"
      description="直接在浏览器里发起 /v1/chat/completions 请求，查看响应内容、状态码、耗时与关键元信息。"
    >
      <div className="playground-page">
        <form className="playground-form" onSubmit={handleSubmit}>
          <div className="playground-form__header">
            <div>
              <h2>Request Editor</h2>
              <p>在 Web 里直接调试网关请求，不需要切回 curl 或 verify 脚本。</p>
            </div>
            <div className="playground-form__actions">
              <button type="button" onClick={addMessage}>添加消息</button>
              <button type="submit" disabled={submitting}>{submitting ? '发送中…' : '发送请求'}</button>
            </div>
          </div>

          <div className="playground-form__grid">
            <label>
              Model
              <input value={requestState.model} onChange={(event) => setRequestState((prev) => ({ ...prev, model: event.target.value }))} />
            </label>
            <label>
              Tenant ID
              <input value={requestState.tenantID} onChange={(event) => setRequestState((prev) => ({ ...prev, tenantID: event.target.value }))} />
            </label>
            <label>
              Task Hint
              <input value={requestState.taskHint} onChange={(event) => setRequestState((prev) => ({ ...prev, taskHint: event.target.value }))} placeholder="analysis / code / chat" />
            </label>
          </div>

          <div className="playground-messages">
            {requestState.messages.map((message, index) => (
              <div key={index} className="playground-message-row">
                <label>
                  Role
                  <input value={message.role} onChange={(event) => updateMessage(index, 'role', event.target.value)} />
                </label>
                <label className="playground-message-row__content">
                  Content
                  <textarea value={message.content} rows={4} onChange={(event) => updateMessage(index, 'content', event.target.value)} />
                </label>
              </div>
            ))}
          </div>

          {error ? <div className="config-error">{error}</div> : null}
        </form>

        <section className="playground-response">
          <div className="playground-response__header">
            <div>
              <h2>Response Panel</h2>
              <p>查看响应正文、状态码、耗时与关键响应头。</p>
            </div>
          </div>

          {result ? (
            <>
              <div className="summary-card-grid playground-summary-grid">
                <div className="summary-card">
                  <span>Status</span>
                  <strong>{result.status}</strong>
                </div>
                <div className="summary-card">
                  <span>Elapsed</span>
                  <strong>{result.elapsedMs} ms</strong>
                </div>
                <div className="summary-card">
                  <span>X-Cache</span>
                  <strong>{result.headers['x-cache'] ?? '—'}</strong>
                </div>
                <div className="summary-card">
                  <span>X-Semantic-Score</span>
                  <strong>{result.headers['x-semantic-score'] ?? '—'}</strong>
                </div>
              </div>

              <div className="playground-panels">
                <div className="playground-panel-card">
                  <h3>Response JSON</h3>
                  <pre>{JSON.stringify(result.body, null, 2)}</pre>
                </div>
                <div className="playground-panel-card">
                  <h3>Request Preview</h3>
                  <pre>{JSON.stringify(requestPreview, null, 2)}</pre>
                </div>
                <div className="playground-panel-card">
                  <h3>最近请求</h3>
                  {recentRequests.length > 0 ? (
                    <div className="playground-history">
                      {recentRequests.map((item) => (
                        <button
                          key={item.id}
                          type="button"
                          className="playground-history__item"
                          onClick={() => {
                            setRequestState((current) => ({
                              ...current,
                              model: item.model,
                              tenantID: item.tenantID,
                            }))
                          }}
                        >
                          <strong>{item.model} / {item.tenantID}</strong>
                          <span>Status {item.status} · {item.elapsedMs} ms</span>
                          <small>{item.messagePreview}</small>
                        </button>
                      ))}
                    </div>
                  ) : (
                    <div className="event-state">发送过的请求会出现在这里，支持快速回填基础字段。</div>
                  )}
                </div>
              </div>
            </>
          ) : (
            <div className="event-state">发送请求后，这里会显示返回内容、响应头和关键元信息。</div>
          )}
        </section>
      </div>
    </AppShell>
  )
}
