import { FormEvent, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { EmptyState } from '../components/ui/EmptyState'
import { TableSkeleton } from '../components/ui/Skeleton'
import { Tabs } from '../components/ui/Tabs'
import {
  createPreset,
  createMaskRule,
  deleteMaskRule,
  deletePreset,
  fetchMaskRules,
  fetchPresets,
  updateMaskRule,
  updatePreset,
} from '../lib/api/presets'
import type { MaskRule, MaskRuleInput, PromptPreset, PromptPresetInput } from '../types/preset'

type PresetFormState = {
  name: string
  system_prompt: string
  model: string
  temperature: string
  max_tokens: string
  variables: string
}

type MaskFormState = {
  name: string
  pattern: string
  replacement: string
}

const emptyPresetForm: PresetFormState = {
  name: '',
  system_prompt: '',
  model: '',
  temperature: '0.7',
  max_tokens: '4096',
  variables: '',
}

const emptyMaskForm: MaskFormState = {
  name: '',
  pattern: '',
  replacement: '***',
}

function buildPresetInput(form: PresetFormState): PromptPresetInput {
  return {
    tenant_id: 'default',
    name: form.name.trim(),
    system_prompt: form.system_prompt.trim(),
    model: form.model.trim(),
    temperature: parseFloat(form.temperature) || 0.7,
    max_tokens: parseInt(form.max_tokens, 10) || 4096,
  }
}

function previewTemplate(template: string, variablesStr: string): string {
  const variables = variablesStr
    .split(',')
    .map((v) => v.trim())
    .filter(Boolean)
  if (variables.length === 0 || !template.trim()) return template
  let result = template
  for (const v of variables) {
    result = result.replace(new RegExp(`\\{\\{\\s*${v}\\s*\\}\\}`, 'g'), `[${v}]`)
  }
  return result
}

export function PresetsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<'presets' | 'masks'>('presets')
  const [showPresetForm, setShowPresetForm] = useState(false)
  const [showMaskForm, setShowMaskForm] = useState(false)
  const [editingPresetId, setEditingPresetId] = useState<number | null>(null)
  const [presetForm, setPresetForm] = useState<PresetFormState>(emptyPresetForm)
  const [maskForm, setMaskForm] = useState<MaskFormState>(emptyMaskForm)

  // ── Queries ──────────────────────────────────────────
  const presetsQuery = useQuery({
    queryKey: ['prompt-presets'],
    queryFn: fetchPresets,
  })

  const masksQuery = useQuery({
    queryKey: ['mask-rules'],
    queryFn: fetchMaskRules,
  })

  // ── Mutations ────────────────────────────────────────
  const createPresetMutation = useMutation({
    mutationFn: (input: PromptPresetInput) => createPreset(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-presets'] })
      setShowPresetForm(false)
      setPresetForm(emptyPresetForm)
    },
  })

  const updatePresetMutation = useMutation({
    mutationFn: ({ id, input }: { id: number; input: Partial<PromptPresetInput> }) =>
      updatePreset(id, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-presets'] })
      setEditingPresetId(null)
      setPresetForm(emptyPresetForm)
    },
  })

  const deletePresetMutation = useMutation({
    mutationFn: (id: number) => deletePreset(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['prompt-presets'] }),
  })

  const createMaskMutation = useMutation({
    mutationFn: (input: MaskRuleInput) => createMaskRule(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mask-rules'] })
      setShowMaskForm(false)
      setMaskForm(emptyMaskForm)
    },
  })

  const toggleMaskMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      updateMaskRule(id, { enabled }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['mask-rules'] }),
  })

  const deleteMaskMutation = useMutation({
    mutationFn: (id: number) => deleteMaskRule(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['mask-rules'] }),
  })

  // ── Handlers ─────────────────────────────────────────
  function handleCreatePreset(e: FormEvent) {
    e.preventDefault()
    createPresetMutation.mutate(buildPresetInput(presetForm))
  }

  function handleUpdatePreset(e: FormEvent) {
    e.preventDefault()
    if (editingPresetId === null) return
    updatePresetMutation.mutate({
      id: editingPresetId,
      input: buildPresetInput(presetForm),
    })
  }

  function handleEditPreset(preset: PromptPreset) {
    setEditingPresetId(preset.id)
    setPresetForm({
      name: preset.name,
      system_prompt: preset.system_prompt,
      model: preset.model,
      temperature: String(preset.temperature),
      max_tokens: String(preset.max_tokens),
      variables: '',
    })
    setShowPresetForm(false)
  }

  function handleCancelPresetForm() {
    setShowPresetForm(false)
    setEditingPresetId(null)
    setPresetForm(emptyPresetForm)
  }

  function handleCreateMask(e: FormEvent) {
    e.preventDefault()
    createMaskMutation.mutate({
      tenant_id: 'default',
      name: maskForm.name.trim(),
      pattern: maskForm.pattern.trim(),
      replacement: maskForm.replacement.trim() || '***',
      enabled: true,
    })
  }

  // ── Variable preview ─────────────────────────────────
  const variablePreview = useMemo(
    () => previewTemplate(presetForm.system_prompt, presetForm.variables),
    [presetForm.system_prompt, presetForm.variables],
  )

  const isEditing = editingPresetId !== null
  const presets = presetsQuery.data ?? []
  const masks = masksQuery.data ?? []

  const tabItems = [
    { key: 'presets' as const, label: t('presets.tabPresets') },
    { key: 'masks' as const, label: t('presets.tabMasks') },
  ]

  return (
    <AppShell title={t('presets.pageTitle')} description={t('presets.pageDescription')}>
      <Tabs items={tabItems} activeKey={activeTab} onChange={(k) => setActiveTab(k as 'presets' | 'masks')} />

      {activeTab === 'presets' ? (
        <div className="presets-section">
          <div className="presets-toolbar">
            <button
              type="button"
              className="btn btn--primary"
              onClick={() => {
                setEditingPresetId(null)
                setPresetForm(emptyPresetForm)
                setShowPresetForm(true)
              }}
            >
              + {t('presets.newPreset')}
            </button>
          </div>

          {showPresetForm || isEditing ? (
            <form
              className="page-surface"
              onSubmit={isEditing ? handleUpdatePreset : handleCreatePreset}
              style={{ marginBottom: '1.5rem' }}
            >
              <h3 style={{ margin: '0 0 1rem', fontSize: '1rem' }}>
                {isEditing ? t('presets.editPreset') : t('presets.newPreset')}
              </h3>
              <div className="system-config-grid">
                <label>
                  {t('presets.name')}
                  <input
                    value={presetForm.name}
                    onChange={(e) => setPresetForm((p) => ({ ...p, name: e.target.value }))}
                    placeholder={t('presets.namePlaceholder')}
                    required
                  />
                </label>
                <label>
                  {t('presets.model')}
                  <input
                    value={presetForm.model}
                    onChange={(e) => setPresetForm((p) => ({ ...p, model: e.target.value }))}
                    placeholder={t('presets.modelPlaceholder')}
                    required
                  />
                </label>
                <label>
                  {t('presets.temperature')}
                  <input
                    type="number"
                    step="0.1"
                    min="0"
                    max="2"
                    value={presetForm.temperature}
                    onChange={(e) => setPresetForm((p) => ({ ...p, temperature: e.target.value }))}
                  />
                </label>
                <label>
                  {t('presets.maxTokens')}
                  <input
                    type="number"
                    min="1"
                    value={presetForm.max_tokens}
                    onChange={(e) => setPresetForm((p) => ({ ...p, max_tokens: e.target.value }))}
                  />
                </label>
                <label style={{ gridColumn: '1 / -1' }}>
                  {t('presets.systemPrompt')}
                  <textarea
                    rows={4}
                    value={presetForm.system_prompt}
                    onChange={(e) => setPresetForm((p) => ({ ...p, system_prompt: e.target.value }))}
                    placeholder={t('presets.systemPromptPlaceholder')}
                    required
                  />
                </label>
                <label style={{ gridColumn: '1 / -1' }}>
                  {t('presets.variables')}
                  <input
                    value={presetForm.variables}
                    onChange={(e) => setPresetForm((p) => ({ ...p, variables: e.target.value }))}
                    placeholder={t('presets.variablesPlaceholder')}
                  />
                </label>
                {presetForm.variables.trim() && presetForm.system_prompt.trim() ? (
                  <div style={{ gridColumn: '1 / -1' }}>
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
                      {t('presets.preview')}
                    </div>
                    <div
                      className="page-surface"
                      style={{
                        padding: '0.75rem',
                        fontFamily: 'var(--font-mono, monospace)',
                        fontSize: '0.85rem',
                        whiteSpace: 'pre-wrap',
                        background: 'var(--surface-alt, #f8f9fa)',
                        border: '1px solid var(--border, #e5e7eb)',
                        borderRadius: '6px',
                        minHeight: '2.5rem',
                      }}
                    >
                      {variablePreview}
                    </div>
                  </div>
                ) : null}
              </div>
              <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1rem' }}>
                <button
                  type="submit"
                  className="btn btn--primary"
                  disabled={createPresetMutation.isPending || updatePresetMutation.isPending}
                >
                  {isEditing
                    ? (updatePresetMutation.isPending ? t('presets.updating') : t('common.save'))
                    : (createPresetMutation.isPending ? t('presets.creating') : t('common.create'))}
                </button>
                <button
                  type="button"
                  className="btn btn--outline"
                  onClick={handleCancelPresetForm}
                >
                  {t('common.cancel')}
                </button>
              </div>
              {(createPresetMutation.error || updatePresetMutation.error) ? (
                <div className="config-error" style={{ marginTop: '0.75rem' }}>
                  {((createPresetMutation.error || updatePresetMutation.error) as Error).message}
                </div>
              ) : null}
            </form>
          ) : null}

          {presetsQuery.isLoading ? (
            <TableSkeleton rows={5} cols={5} />
          ) : presets.length === 0 ? (
            <EmptyState
              title={t('presets.emptyTitle')}
              description={t('presets.emptyDescription')}
              action={{ label: t('presets.newPreset'), onClick: () => setShowPresetForm(true) }}
            />
          ) : (
            <div className="presets-table">
              <table>
                <thead>
                  <tr>
                    <th>{t('presets.name')}</th>
                    <th>{t('presets.model')}</th>
                    <th>{t('presets.temperature')}</th>
                    <th>{t('presets.maxTokens')}</th>
                    <th>{t('presets.createdAt')}</th>
                    <th>{t('presets.actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {presets.map((p) => (
                    <tr key={p.id}>
                      <td>{p.name}</td>
                      <td>{p.model}</td>
                      <td>{p.temperature}</td>
                      <td>{p.max_tokens}</td>
                      <td>{p.created_at ?? '-'}</td>
                      <td>
                        <div style={{ display: 'flex', gap: '0.5rem' }}>
                          <button
                            type="button"
                            className="btn btn--sm btn--outline"
                            onClick={() => handleEditPreset(p)}
                          >
                            {t('common.edit')}
                          </button>
                          <button
                            type="button"
                            className="btn btn--sm btn--danger-ghost"
                            onClick={() => {
                              if (confirm(t('presets.confirmDelete', { name: p.name }))) {
                                deletePresetMutation.mutate(p.id)
                              }
                            }}
                          >
                            {t('common.delete')}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : (
        <div className="masks-section">
          <div className="presets-toolbar">
            <button
              type="button"
              className="btn btn--primary"
              onClick={() => setShowMaskForm(true)}
            >
              + {t('presets.newMaskRule')}
            </button>
          </div>

          {showMaskForm ? (
            <form className="page-surface" onSubmit={handleCreateMask} style={{ marginBottom: '1.5rem' }}>
              <h3 style={{ margin: '0 0 1rem', fontSize: '1rem' }}>{t('presets.newMaskRule')}</h3>
              <div className="system-config-grid">
                <label>
                  {t('presets.name')}
                  <input
                    value={maskForm.name}
                    onChange={(e) => setMaskForm((f) => ({ ...f, name: e.target.value }))}
                    placeholder={t('presets.maskNamePlaceholder')}
                    required
                  />
                </label>
                <label>
                  {t('presets.replacement')}
                  <input
                    value={maskForm.replacement}
                    onChange={(e) => setMaskForm((f) => ({ ...f, replacement: e.target.value }))}
                    placeholder="***"
                  />
                </label>
                <label style={{ gridColumn: '1 / -1' }}>
                  {t('presets.pattern')}
                  <input
                    value={maskForm.pattern}
                    onChange={(e) => setMaskForm((f) => ({ ...f, pattern: e.target.value }))}
                    placeholder={t('presets.patternPlaceholder')}
                    required
                  />
                </label>
              </div>
              <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1rem' }}>
                <button type="submit" className="btn btn--primary" disabled={createMaskMutation.isPending}>
                  {createMaskMutation.isPending ? t('presets.creating') : t('common.create')}
                </button>
                <button
                  type="button"
                  className="btn btn--outline"
                  onClick={() => {
                    setShowMaskForm(false)
                    setMaskForm(emptyMaskForm)
                  }}
                >
                  {t('common.cancel')}
                </button>
              </div>
              {createMaskMutation.error ? (
                <div className="config-error" style={{ marginTop: '0.75rem' }}>
                  {(createMaskMutation.error as Error).message}
                </div>
              ) : null}
            </form>
          ) : null}

          {masksQuery.isLoading ? (
            <TableSkeleton rows={5} cols={4} />
          ) : masks.length === 0 ? (
            <EmptyState
              title={t('presets.maskEmptyTitle')}
              description={t('presets.maskEmptyDescription')}
              action={{ label: t('presets.newMaskRule'), onClick: () => setShowMaskForm(true) }}
            />
          ) : (
            <div className="presets-table">
              <table>
                <thead>
                  <tr>
                    <th>{t('presets.name')}</th>
                    <th>{t('presets.pattern')}</th>
                    <th>{t('presets.replacement')}</th>
                    <th>{t('presets.status')}</th>
                    <th>{t('presets.createdAt')}</th>
                    <th>{t('presets.actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {masks.map((m) => (
                    <tr key={m.id}>
                      <td>{m.name}</td>
                      <td><code>{m.pattern}</code></td>
                      <td>{m.replacement}</td>
                      <td>
                        <button
                          type="button"
                          className={m.enabled ? 'badge badge--success' : 'badge badge--warning'}
                          onClick={() => {
                            const action = m.enabled ? 'disable' : 'enable'
                            const confirmKey = m.enabled
                              ? 'presets.confirmToggleDisable'
                              : 'presets.confirmToggleEnable'
                            if (confirm(t(confirmKey, { name: m.name }))) {
                              toggleMaskMutation.mutate({ id: m.id, enabled: !m.enabled })
                            }
                          }}
                          style={{ cursor: 'pointer', border: 'none', font: 'inherit' }}
                        >
                          {m.enabled ? t('presets.enabled') : t('presets.disabled')}
                        </button>
                      </td>
                      <td>{m.created_at ?? '-'}</td>
                      <td>
                        <button
                          type="button"
                          className="btn btn--sm btn--danger-ghost"
                          onClick={() => {
                            if (confirm(t('presets.confirmDeleteMask', { name: m.name }))) {
                              deleteMaskMutation.mutate(m.id)
                            }
                          }}
                        >
                          {t('common.delete')}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </AppShell>
  )
}
