import { useState, useMemo } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createChannel, updateChannel, fetchProviderModels } from '../lib/channels'
import type { Channel, CreateChannelRequest, ChannelProvider, ChannelPriority } from '../types/channel'

const PROVIDERS: { value: ChannelProvider; label: string }[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'google', label: 'Google AI' },
  { value: 'azure', label: 'Azure OpenAI' },
  { value: 'aws', label: 'AWS Bedrock' },
  { value: 'custom', label: '自定义' },
]

const PRIORITIES: { value: ChannelPriority; label: string }[] = [
  { value: 'highest', label: '最高' },
  { value: 'high', label: '高' },
  { value: 'medium', label: '中' },
  { value: 'low', label: '低' },
  { value: 'lowest', label: '最低' },
]

type ChannelFormModalProps = {
  channel: Channel | null
  onClose: () => void
}

export function ChannelFormModal({ channel, onClose }: ChannelFormModalProps) {
  const queryClient = useQueryClient()
  const isEditing = !!channel

  const [form, setForm] = useState<CreateChannelRequest>({
    name: channel?.name ?? '',
    provider: channel?.provider ?? 'openai',
    base_url: channel?.base_url ?? '',
    api_key: channel?.api_key ?? '',
    priority: channel?.priority ?? 'medium',
    weight: channel?.weight ?? 1,
    models: channel?.models ?? [],
    tags: channel?.tags ?? [],
    notes: channel?.notes ?? '',
  })

  const [errors, setErrors] = useState<Record<string, string>>({})
  const [modelInput, setModelInput] = useState('')
  const [providerInput, setProviderInput] = useState<string>(channel?.provider ?? 'openai')

  // ── 读取模型 ────────────────────────────
  const [fetchedModels, setFetchedModels] = useState<string[]>([])
  const [fetchingModels, setFetchingModels] = useState(false)
  const [fetchModelsError, setFetchModelsError] = useState('')
  const [selectedFetchedModels, setSelectedFetchedModels] = useState<Set<string>>(new Set())

  const handleFetchModels = async () => {
    if (!form.base_url.trim()) {
      setFetchModelsError('请先填写 Base URL')
      return
    }
    setFetchingModels(true)
    setFetchModelsError('')
    try {
      const models = await fetchProviderModels({
        provider: form.provider,
        base_url: form.base_url,
        api_key: form.api_key ?? '',
      })
      setFetchedModels(models)
      setSelectedFetchedModels(new Set())
    } catch (err) {
      setFetchModelsError((err as Error).message ?? '读取模型失败')
    } finally {
      setFetchingModels(false)
    }
  }

  const toggleFetchedModel = (model: string) => {
    setSelectedFetchedModels(prev => {
      const next = new Set(prev)
      if (next.has(model)) next.delete(model)
      else next.add(model)
      return next
    })
  }

  const addSelectedModels = () => {
    const existing = new Set(form.models ?? [])
    selectedFetchedModels.forEach(m => existing.add(m))
    setForm({ ...form, models: Array.from(existing) })
    setSelectedFetchedModels(new Set())
  }

  const availableToAdd = useMemo(() => {
    const existing = new Set(form.models ?? [])
    return fetchedModels.filter(m => !existing.has(m))
  }, [fetchedModels, form.models])

  const mutation = useMutation({
    mutationFn: isEditing
      ? () => updateChannel(channel!.id, form)
      : () => createChannel(form),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      onClose()
    },
  })

  const validate = (): boolean => {
    const errs: Record<string, string> = {}
    if (!form.name.trim()) errs.name = '渠道名称不能为空'
    if (!form.base_url.trim()) errs.base_url = 'Base URL 不能为空'
    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!validate()) return
    mutation.mutate()
  }

  const addModel = () => {
    const model = modelInput.trim()
    if (model && !form.models!.includes(model)) {
      setForm({ ...form, models: [...(form.models ?? []), model] })
      setModelInput('')
    }
  }

  const removeModel = (model: string) => {
    setForm({ ...form, models: form.models!.filter((m) => m !== model) })
  }

  const addTag = (tag: string) => {
    const t = tag.trim()
    if (t && !form.tags!.includes(t)) {
      setForm({ ...form, tags: [...form.tags!, t] })
    }
  }

  const removeTag = (tag: string) => {
    setForm({ ...form, tags: form.tags!.filter((t) => t !== tag) })
  }

  const updateField = <K extends keyof CreateChannelRequest>(
    key: K,
    value: CreateChannelRequest[K],
  ) => {
    setForm({ ...form, [key]: value })
    if (errors[key]) {
      setErrors({ ...errors, [key]: '' })
    }
  }

  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <div className="dialog-card channel-form-modal" onClick={(e) => e.stopPropagation()}>
        <div className="dialog-card__header">
          <div>
            <h2>{isEditing ? '编辑渠道' : '添加渠道'}</h2>
            <p>配置 LLM 供应商连接信息</p>
          </div>
          <button type="button" onClick={onClose}>
            关闭
          </button>
        </div>

        <form onSubmit={handleSubmit}>
          {/* Tabs: Basic / Routing / Advanced */}
          <div className="channel-form__tabs">
            <TabPanel label="基础信息">
              <div className="channel-form__grid">
                <label>
                  名称 *
                  <input
                    type="text"
                    value={form.name}
                    onChange={(e) => updateField('name', e.target.value)}
                    placeholder="例如: OpenAI-Prod"
                  />
                  {errors.name ? <span className="field-error">{errors.name}</span> : null}
                </label>

                <label>
                  供应商
                  <div style={{ display: 'flex', gap: '4px' }}>
                    <input
                      type="text"
                      list="provider-options"
                      value={providerInput}
                      onChange={(e) => {
                        setProviderInput(e.target.value)
                        updateField('provider', e.target.value as ChannelProvider)
                      }}
                      placeholder="选择或输入供应商名称"
                      style={{ flex: 1 }}
                    />
                    <datalist id="provider-options">
                      {PROVIDERS.map((p) => (
                        <option key={p.value} value={p.value}>{p.label}</option>
                      ))}
                    </datalist>
                  </div>
                </label>

                <label>
                  Base URL *
                  <input
                    type="text"
                    value={form.base_url}
                    onChange={(e) => updateField('base_url', e.target.value)}
                    placeholder="https://api.openai.com/v1"
                  />
                  {errors.base_url ? <span className="field-error">{errors.base_url}</span> : null}
                </label>

                <label>
                  API Key
                  <input
                    type="password"
                    value={form.api_key ?? ''}
                    onChange={(e) => updateField('api_key', e.target.value)}
                    placeholder={isEditing ? '留空则不修改' : 'sk-...'}
                  />
                </label>
              </div>
            </TabPanel>

            <TabPanel label="路由配置">
              <div className="channel-form__grid">
                <label>
                  优先级
                  <select
                    value={form.priority ?? 'medium'}
                    onChange={(e) => updateField('priority', e.target.value as ChannelPriority)}
                  >
                    {PRIORITIES.map((p) => (
                      <option key={p.value} value={p.value}>
                        {p.label}
                      </option>
                    ))}
                  </select>
                </label>

                <label>
                  权重
                  <input
                    type="number"
                    min={1}
                    max={100}
                    value={form.weight ?? 1}
                    onChange={(e) => updateField('weight', parseInt(e.target.value) || 1)}
                  />
                </label>

                <label className="channel-form__full-row">
                  模型列表
                  {/* 读取模型按钮 */}
                  <div style={{ display: 'flex', gap: '8px', marginBottom: '8px' }}>
                    <button
                      type="button"
                      className="btn btn--outline btn--sm"
                      onClick={handleFetchModels}
                      disabled={fetchingModels}
                    >
                      {fetchingModels ? '读取中...' : '📡 读取模型'}
                    </button>
                    {fetchModelsError && (
                      <span style={{ color: 'var(--danger-color)', fontSize: '12px', alignSelf: 'center' }}>
                        {fetchModelsError}
                      </span>
                    )}
                  </div>

                  {/* 读取到的模型列表（多选） */}
                  {fetchedModels.length > 0 && (
                    <div style={{
                      border: '1px solid var(--border-color)',
                      borderRadius: '6px',
                      padding: '8px',
                      maxHeight: '200px',
                      overflowY: 'auto',
                      marginBottom: '8px',
                      background: 'var(--surface-color)',
                    }}>
                      <div style={{ fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '6px' }}>
                        共 {fetchedModels.length} 个模型，已选 {selectedFetchedModels.size} 个
                      </div>
                      {availableToAdd.map((m) => (
                        <label
                          key={m}
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: '6px',
                            padding: '2px 0',
                            cursor: 'pointer',
                            fontSize: '13px',
                          }}
                        >
                          <input
                            type="checkbox"
                            checked={selectedFetchedModels.has(m)}
                            onChange={() => toggleFetchedModel(m)}
                            style={{ margin: 0 }}
                          />
                          {m}
                        </label>
                      ))}
                      {selectedFetchedModels.size > 0 && (
                        <button
                          type="button"
                          className="btn btn--sm btn--primary"
                          onClick={addSelectedModels}
                          style={{ marginTop: '8px', width: '100%' }}
                        >
                          添加选中的 {selectedFetchedModels.size} 个模型
                        </button>
                      )}
                    </div>
                  )}

                  {/* 手动添加模型 */}
                  <div className="channel-form__tag-input">
                    <input
                      type="text"
                      value={modelInput}
                      onChange={(e) => setModelInput(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault()
                          addModel()
                        }
                      }}
                      placeholder="输入模型 ID 后回车添加"
                    />
                    <button type="button" className="btn btn--sm" onClick={addModel}>
                      添加
                    </button>
                  </div>
                  <div className="channel-form__tags">
                    {form.models?.map((m) => (
                      <span key={m} className="channel-form__tag">
                        {m}
                        <button
                          type="button"
                          className="channel-form__tag-remove"
                          onClick={() => removeModel(m)}
                        >
                          ×
                        </button>
                      </span>
                    ))}
                  </div>
                </label>
              </div>
            </TabPanel>

            <TabPanel label="高级配置">
              <div className="channel-form__grid">
                <label>
                  标签
                  <ChannelTagInput
                    tags={form.tags ?? []}
                    onAdd={addTag}
                    onRemove={removeTag}
                  />
                </label>

                <label className="channel-form__full-row">
                  备注
                  <textarea
                    value={form.notes ?? ''}
                    onChange={(e) => updateField('notes', e.target.value)}
                    rows={3}
                    placeholder="可选备注信息"
                  />
                </label>
              </div>
            </TabPanel>
          </div>

          <div className="dialog-card__actions">
            <button type="button" onClick={onClose}>
              取消
            </button>
            <button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? '保存中...' : isEditing ? '保存修改' : '创建渠道'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function TabPanel({ label, children }: { label: string; children: React.ReactNode }) {
  // For now render all tabs. The parent will show/hide via CSS.
  return (
    <div className="channel-form__tab-panel" data-tab-label={label}>
      {children}
    </div>
  )
}

function ChannelTagInput({
  tags,
  onAdd,
  onRemove,
}: {
  tags: string[]
  onAdd: (tag: string) => void
  onRemove: (tag: string) => void
}) {
  const [input, setInput] = useState('')

  const handleAdd = () => {
    const v = input.trim()
    if (v) {
      onAdd(v)
      setInput('')
    }
  }

  return (
    <div>
      <div className="channel-form__tag-input">
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              handleAdd()
            }
          }}
          placeholder="输入标签后回车"
        />
        <button type="button" className="btn btn--sm" onClick={handleAdd}>
          添加
        </button>
      </div>
      <div className="channel-form__tags">
        {tags.map((t) => (
          <span key={t} className="channel-form__tag">
            {t}
            <button type="button" className="channel-form__tag-remove" onClick={() => onRemove(t)}>
              ×
            </button>
          </span>
        ))}
      </div>
    </div>
  )
}
