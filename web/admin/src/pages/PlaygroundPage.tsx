import { FormEvent, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
      setError(t('playground.fillRequired'))
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
      setError(unknownError instanceof Error ? unknownError.message : t('playground.requestFailed'))
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
      title={t('playground.pageTitle')}
      description={t('playground.pageDescription')}
    >
      <div className="playground-page">
        <form className="playground-form" onSubmit={handleSubmit}>
          <div className="playground-form__header">
            <div>
              <h2>{t('playground.requestEditor')}</h2>
              <p>{t('playground.requestEditorDesc')}</p>
            </div>
            <div className="playground-form__actions">
              <button type="button" onClick={addMessage}>{t('playground.addMessage')}</button>
              <button type="submit" disabled={submitting}>{submitting ? t('playground.sending') : t('playground.sendRequest')}</button>
            </div>
          </div>

          <div className="playground-form__grid">
            <label>
              {t('playground.model')}
              <input value={requestState.model} onChange={(event) => setRequestState((prev) => ({ ...prev, model: event.target.value }))} />
            </label>
            <label>
              {t('playground.tenantId')}
              <input value={requestState.tenantID} onChange={(event) => setRequestState((prev) => ({ ...prev, tenantID: event.target.value }))} />
            </label>
            <label>
              {t('playground.taskHint')}
              <input value={requestState.taskHint} onChange={(event) => setRequestState((prev) => ({ ...prev, taskHint: event.target.value }))} placeholder={t('playground.taskHintPlaceholder')} />
            </label>
          </div>

          <div className="playground-messages">
            {requestState.messages.map((message, index) => (
              <div key={index} className="playground-message-row">
                  <label>
                    {t('playground.role')}
                    <input value={message.role} onChange={(event) => updateMessage(index, 'role', event.target.value)} />
                  </label>
                  <label className="playground-message-row__content">
                    {t('playground.content')}
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
              <h2>{t('playground.responsePanel')}</h2>
              <p>{t('playground.responsePanelDesc')}</p>
            </div>
          </div>

          {result ? (
            <>
              <div className="summary-card-grid playground-summary-grid">
                <div className="summary-card">
                  <span>{t('playground.statusCode')}</span>
                  <strong>{result.status}</strong>
                </div>
                <div className="summary-card">
                  <span>{t('playground.elapsed')}</span>
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
                  <h3>{t('playground.responseJson')}</h3>
                  <pre>{JSON.stringify(result.body, null, 2)}</pre>
                </div>
                <div className="playground-panel-card">
                  <h3>{t('playground.requestPreview')}</h3>
                  <pre>{JSON.stringify(requestPreview, null, 2)}</pre>
                </div>
                <div className="playground-panel-card">
                  <h3>{t('playground.recentRequests')}</h3>
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
                          <span>{t('playground.status')} {item.status} · {item.elapsedMs} ms</span>
                          <small>{item.messagePreview}</small>
                        </button>
                      ))}
                    </div>
                  ) : (
                    <div className="event-state">{t('playground.noRecentRequests')}</div>
                  )}
                </div>
              </div>
            </>
          ) : (
            <div className="event-state">{t('playground.noResponse')}</div>
          )}
        </section>
      </div>
    </AppShell>
  )
}
