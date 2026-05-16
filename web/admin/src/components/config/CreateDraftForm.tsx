import { FormEvent, useState } from 'react'
import { jsonRequest } from '../../lib/http'
import type { ConfigVersion } from '../../types/admin'

type CreateDraftFormProps = {
  onCreated?: (version: ConfigVersion) => void
}

type DraftFormState = {
  module: string
  tenantID: string
  scope: string
  projectID: string
  sourceEnvironment: string
  targetEnvironment: string
  actor: string
  reason: string
}

const initialState: DraftFormState = {
  module: '',
  tenantID: '',
  scope: 'tenant',
  projectID: '',
  sourceEnvironment: '',
  targetEnvironment: '',
  actor: '',
  reason: '',
}

export function CreateDraftForm({ onCreated }: CreateDraftFormProps) {
  const [form, setForm] = useState<DraftFormState>(initialState)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [successMessage, setSuccessMessage] = useState('')

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!form.module.trim() || !form.tenantID.trim() || !form.scope.trim() || !form.sourceEnvironment.trim() || !form.targetEnvironment.trim()) {
      setError('请填写必填字段')
      setSuccessMessage('')
      return
    }

    if (form.sourceEnvironment.trim() === form.targetEnvironment.trim()) {
      setError('源环境和目标环境不能相同')
      setSuccessMessage('')
      return
    }

    setSubmitting(true)
    setError('')
    setSuccessMessage('')

    try {
      const created = await jsonRequest<ConfigVersion>('/admin/inheritance-drafts', {
        module: form.module.trim(),
        tenant_id: form.tenantID.trim(),
        scope: form.scope.trim(),
        project_id: form.projectID.trim(),
        source_environment: form.sourceEnvironment.trim(),
        target_environment: form.targetEnvironment.trim(),
        actor: form.actor.trim(),
        reason: form.reason.trim(),
      })

      setSuccessMessage(`已创建 Draft ${created.version_id}`)
      onCreated?.(created)
    } catch (unknownError) {
      const message = unknownError instanceof Error ? unknownError.message : '创建 Draft 失败'
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="draft-form" aria-label="创建 Draft 表单" onSubmit={handleSubmit}>
      <div className="draft-form__header">
        <div>
          <h2>创建继承 Draft</h2>
          <p>从已发布环境派生一个目标环境 Draft，作为后续发布/推广的入口。</p>
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? '提交中…' : '创建 Draft'}
        </button>
      </div>

      <div className="draft-form__grid">
          <label>
            模块
            <input value={form.module} onChange={(event) => setForm((prev) => ({ ...prev, module: event.target.value }))} placeholder="router" />
          </label>
          <label>
            租户 ID
            <input value={form.tenantID} onChange={(event) => setForm((prev) => ({ ...prev, tenantID: event.target.value }))} placeholder="tenant-a" />
          </label>
          <label>
            作用域
            <input value={form.scope} onChange={(event) => setForm((prev) => ({ ...prev, scope: event.target.value }))} placeholder="tenant" />
          </label>
          <label>
            项目 ID
            <input value={form.projectID} onChange={(event) => setForm((prev) => ({ ...prev, projectID: event.target.value }))} placeholder="project-x" />
          </label>
          <label>
            来源环境
            <input value={form.sourceEnvironment} onChange={(event) => setForm((prev) => ({ ...prev, sourceEnvironment: event.target.value }))} placeholder="staging" />
          </label>
          <label>
            目标环境
            <input value={form.targetEnvironment} onChange={(event) => setForm((prev) => ({ ...prev, targetEnvironment: event.target.value }))} placeholder="prod" />
          </label>
          <label>
            执行人
            <input value={form.actor} onChange={(event) => setForm((prev) => ({ ...prev, actor: event.target.value }))} placeholder="release-bot" />
          </label>
          <label>
            原因
            <input value={form.reason} onChange={(event) => setForm((prev) => ({ ...prev, reason: event.target.value }))} placeholder="prepare prod draft" />
          </label>
      </div>

      {error ? <div className="config-error">{error}</div> : null}
      {successMessage ? <div className="config-success">{successMessage}</div> : null}
    </form>
  )
}
