import { FormEvent, useState } from 'react'
import { jsonRequest } from '../../lib/http'
import type { ConfigVersion } from '../../types/admin'

type PromotionPanelProps = {
  onPromoted?: (version: ConfigVersion) => void
}

type PromotionState = {
  module: string
  tenantID: string
  sourceEnvironment: string
  targetEnvironment: string
  scope: string
  projectID: string
  actor: string
  reason: string
}

const initialState: PromotionState = {
  module: '',
  tenantID: '',
  sourceEnvironment: '',
  targetEnvironment: '',
  scope: 'tenant',
  projectID: '',
  actor: '',
  reason: '',
}

export function PromotionPanel({ onPromoted }: PromotionPanelProps) {
  const [form, setForm] = useState<PromotionState>(initialState)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [successMessage, setSuccessMessage] = useState('')

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!form.module.trim() || !form.tenantID.trim() || !form.sourceEnvironment.trim() || !form.targetEnvironment.trim() || !form.scope.trim()) {
      setError('请填写推广必填字段')
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
      const promoted = await jsonRequest<ConfigVersion>('/admin/promotions', {
        module: form.module.trim(),
        tenant_id: form.tenantID.trim(),
        source_environment: form.sourceEnvironment.trim(),
        target_environment: form.targetEnvironment.trim(),
        scope: form.scope.trim(),
        project_id: form.projectID.trim(),
        actor: form.actor.trim(),
        reason: form.reason.trim(),
      })

      setSuccessMessage(`已推广 ${promoted.version_id}`)
      onPromoted?.(promoted)
    } catch (unknownError) {
      const message = unknownError instanceof Error ? unknownError.message : '推广失败'
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="release-panel" aria-label="推广表单" onSubmit={handleSubmit}>
      <div className="release-panel__header">
        <div>
          <h2>跨环境推广</h2>
          <p>从已发布环境向目标环境生成新的已发布版本，用于跨环境推广。</p>
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? '推广中…' : '执行推广'}
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
            来源环境
            <input value={form.sourceEnvironment} onChange={(event) => setForm((prev) => ({ ...prev, sourceEnvironment: event.target.value }))} placeholder="预发布环境" />
          </label>
          <label>
            目标环境
            <input value={form.targetEnvironment} onChange={(event) => setForm((prev) => ({ ...prev, targetEnvironment: event.target.value }))} placeholder="生产环境" />
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
            执行人
            <input value={form.actor} onChange={(event) => setForm((prev) => ({ ...prev, actor: event.target.value }))} placeholder="发布机器人" />
          </label>
          <label>
            原因
            <input value={form.reason} onChange={(event) => setForm((prev) => ({ ...prev, reason: event.target.value }))} placeholder="将 staging 推广到 prod" />
          </label>
      </div>

      {error ? <div className="config-error">{error}</div> : null}
      {successMessage ? <div className="config-success">{successMessage}</div> : null}
    </form>
  )
}
