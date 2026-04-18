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
      setError('请填写 Release 必填字段')
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
      const message = unknownError instanceof Error ? unknownError.message : '发布 Draft 失败'
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="release-panel" aria-label="Release Draft Form" onSubmit={handleSubmit}>
      <div className="release-panel__header">
        <div>
          <h2>Release Draft</h2>
          <p>将目标环境的 Draft 明确发布为 Released，触发后续运行时生效链路。</p>
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? '发布中…' : '发布 Draft'}
        </button>
      </div>

      <div className="release-panel__grid">
        <label>
          Module
          <input value={form.module} onChange={(event) => setForm((prev) => ({ ...prev, module: event.target.value }))} placeholder="router" />
        </label>
        <label>
          Tenant ID
          <input value={form.tenantID} onChange={(event) => setForm((prev) => ({ ...prev, tenantID: event.target.value }))} placeholder="tenant-a" />
        </label>
        <label>
          Environment
          <input value={form.environment} onChange={(event) => setForm((prev) => ({ ...prev, environment: event.target.value }))} placeholder="prod" />
        </label>
        <label>
          Scope
          <input value={form.scope} onChange={(event) => setForm((prev) => ({ ...prev, scope: event.target.value }))} placeholder="tenant" />
        </label>
        <label>
          Project ID
          <input value={form.projectID} onChange={(event) => setForm((prev) => ({ ...prev, projectID: event.target.value }))} placeholder="project-x" />
        </label>
        <label>
          Version ID
          <input value={form.versionID} onChange={(event) => setForm((prev) => ({ ...prev, versionID: event.target.value }))} placeholder="cfg_101" />
        </label>
        <label>
          Actor
          <input value={form.actor} onChange={(event) => setForm((prev) => ({ ...prev, actor: event.target.value }))} placeholder="release-bot" />
        </label>
        <label>
          Reason
          <input value={form.reason} onChange={(event) => setForm((prev) => ({ ...prev, reason: event.target.value }))} placeholder="approve prod draft" />
        </label>
      </div>

      {error ? <div className="config-error">{error}</div> : null}
      {successMessage ? <div className="config-success">{successMessage}</div> : null}
    </form>
  )
}
