import { FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { EmptyState } from '../components/ui/EmptyState'
import { TableSkeleton } from '../components/ui/Skeleton'
import { Tabs } from '../components/ui/Tabs'
import { PresetFormModal } from '../components/presets/PresetFormModal'
import { MaskFormModal } from '../components/presets/MaskFormModal'
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
import type { MaskRuleInput, PromptPreset, PromptPresetInput } from '../types/preset'

type PresetFormState = {
  name: string
  template: string
  description: string
  variables: string
}

type MaskFormState = {
  name: string
  pattern: string
  replacement: string
}

const emptyPresetForm: PresetFormState = {
  name: '',
  template: '',
  description: '',
  variables: '',
}

const emptyMaskForm: MaskFormState = {
  name: '',
  pattern: '',
  replacement: '***',
}

function buildPresetInput(form: PresetFormState): PromptPresetInput {
  const variables = form.variables
    .split(',')
    .map((v) => v.trim())
    .filter(Boolean)
  return {
    name: form.name.trim(),
    template: form.template.trim(),
    description: form.description.trim(),
    variables,
  }
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
    mutationFn: ({ id, enabled, name, pattern, replace }: { id: number; enabled: boolean; name: string; pattern: string; replace: string }) =>
      updateMaskRule(id, { is_active: enabled, name, pattern, replacement: replace }),
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
      template: preset.template,
      description: preset.description ?? '',
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

  function handleCancelMaskForm() {
    setShowMaskForm(false)
    setMaskForm(emptyMaskForm)
  }

  const isEditing = editingPresetId !== null
  const presets = presetsQuery.data ?? []
  const masks = masksQuery.data ?? []

  const tabItems = [
    { key: 'presets' as const, label: t('presets.tabPresets') },
    { key: 'masks' as const, label: t('presets.tabMasks') },
  ]

  return (
    <AppShell title={t('presets.pageTitle')} description={t('presets.pageDescription')}>
      <Tabs tabs={tabItems} activeKey={activeTab} onChange={(k) => setActiveTab(k as 'presets' | 'masks')} />

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

          <PresetFormModal
            open={showPresetForm || isEditing}
            onClose={handleCancelPresetForm}
            onSubmit={isEditing ? handleUpdatePreset : handleCreatePreset}
            form={presetForm}
            onFormChange={setPresetForm}
            loading={createPresetMutation.isPending || updatePresetMutation.isPending}
            isEditing={isEditing}
            error={createPresetMutation.error || updatePresetMutation.error}
          />

          {presetsQuery.isLoading ? (
            <TableSkeleton rows={5} cols={3} />
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
                    <th>{t('presets.description')}</th>
                    <th>{t('presets.createdAt')}</th>
                    <th>{t('presets.actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {presets.map((p) => (
                    <tr key={p.id}>
                      <td>{p.name}</td>
                      <td>{p.description ?? '-'}</td>
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

          <MaskFormModal
            open={showMaskForm}
            onClose={handleCancelMaskForm}
            onSubmit={handleCreateMask}
            form={maskForm}
            onFormChange={setMaskForm}
            loading={createMaskMutation.isPending}
            error={createMaskMutation.error}
          />

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
                      <td>{m.replace}</td>
                      <td>
                        <button
                          type="button"
                          className={m.is_active ? 'badge badge--success' : 'badge badge--warning'}
                          onClick={() => {
                            const confirmKey = m.is_active
                              ? 'presets.confirmToggleDisable'
                              : 'presets.confirmToggleEnable'
                            if (confirm(t(confirmKey, { name: m.name }))) {
                              toggleMaskMutation.mutate({ id: m.id, enabled: !m.is_active, name: m.name, pattern: m.pattern, replace: m.replace })
                            }
                          }}
                          style={{ cursor: 'pointer', border: 'none', font: 'inherit' }}
                        >
                          {m.is_active ? t('presets.enabled') : t('presets.disabled')}
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
