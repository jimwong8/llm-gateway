import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createChannel, updateChannel } from '../lib/channels'
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
    baseUrl: channel?.baseUrl ?? '',
    apiKey: channel?.apiKey ?? '',
    priority: channel?.priority ?? 'medium',
    weight: channel?.weight ?? 1,
    models: channel?.models ?? [],
    tags: channel?.tags ?? [],
    notes: channel?.notes ?? '',
  })

  const [errors, setErrors] = useState<Record<string, string>>({})
  const [modelInput, setModelInput] = useState('')

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
    if (!form.baseUrl.trim()) errs.baseUrl = 'Base URL 不能为空'
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
                  <select
                    value={form.provider}
                    onChange={(e) => updateField('provider', e.target.value as ChannelProvider)}
                  >
                    {PROVIDERS.map((p) => (
                      <option key={p.value} value={p.value}>
                        {p.label}
                      </option>
                    ))}
                  </select>
                </label>

                <label>
                  Base URL *
                  <input
                    type="text"
                    value={form.baseUrl}
                    onChange={(e) => updateField('baseUrl', e.target.value)}
                    placeholder="https://api.openai.com/v1"
                  />
                  {errors.baseUrl ? <span className="field-error">{errors.baseUrl}</span> : null}
                </label>

                <label>
                  API Key
                  <input
                    type="password"
                    value={form.apiKey ?? ''}
                    onChange={(e) => updateField('apiKey', e.target.value)}
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
