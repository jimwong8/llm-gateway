import { FormEvent, useState } from 'react'
import { jsonRequest } from '../../lib/http'
import type { ConfigVersion } from '../../types/admin'

type ReleaseDraftPanelProps = {
  onReleased?: (version: ConfigVersion) => void
}

type ReleaseDraftState = {
  module: string
  tenantID: string
  environment: string
  scope: string
  projectID: string
  versionID: string
  actor: string
  reason: string
}

const initialState: ReleaseDraftState = {
  module: '',
  tenantID: '',
  environment: '',
  scope: 'tenant',
  projectID: '',
  versionID: '',
  actor: '',
  reason: '',
}

export function ReleaseDraftPanel({ onReleased }: ReleaseDraftPanelProps) {
  const [form, setForm] = useState<ReleaseDraftState>(initialState)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [successMessage, setSuccessMessage] = useState('')

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!form.module.trim() || !form.tenantID.trim() || !form.environment.trim() || !form.scope.trim() || !form.versionID.trim()) {
      setError('请填写发布必填字段')
      setSuccessMessage('')
      return
    }

    setSubmitting(true)
    setError('')
    setSuccessMessage('')

    try {
      const released = await jsonRequest<ConfigVersion>('/admin/releases', {
        module: form.module.trim(),
        tenant_id: form.tenantID.trim(),
        environment: form.environment.trim(),
        scope: form.scope.trim(),
        project_id: form.projectID.trim(),
        version_id: form.versionID.trim(),
        actor: form.actor.trim(),
        reason: form.reason.trim(),
      })

      setSuccessMessage(`已发布 ${released.version_id}`)
      onReleased?.(released)
    } catch (unknownError) {
      const message = unknownError instanceof Error ? unknownError.message : '发布草稿 失败'
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="release-panel" aria-label="发布草稿表单" onSubmit={handleSubmit}>
      <div className="release-panel__header">
        <div>
          <h2>发布草稿</h2>
          <p>将目标环境的 Draft 发布为 Released，触发后续运行时生效链路。</p>
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? '发布中…' : '发布草稿'}
        </button>
      </div>

      <div className="release-panel__grid">
          <label>
            模块
            <input value={form.module} onChange={(event) => setForm((prev) => ({ ...prev, module: event.target.value }))} placeholder="路由模块" />
          </label>
          <label>
            租户 ID
            <input value={form.tenantID} onChange={(event) => setForm((prev) => ({ ...prev, tenantID: event.target.value }))} placeholder="租户-a" />
          </label>
          <label>
            环境
            <input value={form.environment} onChange={(event) => setForm((prev) => ({ ...prev, environment: event.target.value }))} placeholder="生产环境" />
          </label>
          <label>
            作用域
            <input value={form.scope} onChange={(event) => setForm((prev) => ({ ...prev, scope: event.target.value }))} placeholder="租户" />
          </label>
          <label>
            项目 ID
            <input value={form.projectID} onChange={(event) => setForm((prev) => ({ ...prev, projectID: event.target.value }))} placeholder="项目-x" />
          </label>
          <label>
            版本 ID
            <input value={form.versionID} onChange={(event) => setForm((prev) => ({ ...prev, versionID: event.target.value }))} placeholder="配置版本-101" />
          </label>
          <label>
            执行人
            <input value={form.actor} onChange={(event) => setForm((prev) => ({ ...prev, actor: event.target.value }))} placeholder="发布机器人" />
          </label>
          <label>
            原因
            <input value={form.reason} onChange={(event) => setForm((prev) => ({ ...prev, reason: event.target.value }))} placeholder="批准生产环境草稿" />
          </label>
      </div>

      {error ? <div className="config-error">{error}</div> : null}
      {successMessage ? <div className="config-success">{successMessage}</div> : null}
    </form>
  )
}
