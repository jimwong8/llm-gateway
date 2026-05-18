import { FormEvent, useState } from 'react'
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
} from '../lib/api/presets'
import type { MaskRule, MaskRuleInput, PromptPreset, PromptPresetInput } from '../types/preset'

type PresetFormState = {
  name: string
  system_prompt: string
  model: string
  temperature: string
  max_tokens: string
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
}

const emptyMaskForm: MaskFormState = {
  name: '',
  pattern: '',
  replacement: '***',
}

export function PresetsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<'presets' | 'masks'>('presets')
  const [showPresetForm, setShowPresetForm] = useState(false)
  const [showMaskForm, setShowMaskForm] = useState(false)
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

  const deleteMaskMutation = useMutation({
    mutationFn: (id: number) => deleteMaskRule(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['mask-rules'] }),
  })

  // ── Handlers ─────────────────────────────────────────
  function handleCreatePreset(e: FormEvent) {
    e.preventDefault()
    createPresetMutation.mutate({
      tenant_id: 'default',
      name: presetForm.name.trim(),
      system_prompt: presetForm.system_prompt.trim(),
      model: presetForm.model.trim(),
      temperature: parseFloat(presetForm.temperature) || 0.7,
      max_tokens: parseInt(presetForm.max_tokens, 10) || 4096,
    })
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
              onClick={() => setShowPresetForm(true)}
            >
              + {t('presets.newPreset')}
            </button>
          </div>

          {showPresetForm ? (
            <form className="page-surface" onSubmit={handleCreatePreset} style={{ marginBottom: '1.5rem' }}>
              <h3 style={{ margin: '0 0 1rem', fontSize: '1rem' }}>{t('presets.newPreset')}</h3>
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
              </div>
              <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1rem' }}>
                <button type="submit" className="btn btn--primary" disabled={createPresetMutation.isPending}>
                  {createPresetMutation.isPending ? t('presets.creating') : t('common.create')}
                </button>
                <button
                  type="button"
                  className="btn btn--outline"
                  onClick={() => {
                    setShowPresetForm(false)
                    setPresetForm(emptyPresetForm)
                  }}
                >
                   {t('common.cancel')}
                 </button>
               </div>
               {createPresetMutation.error ? (
                 <div className="config-error" style={{ marginTop: '0.75rem' }}>
                   {(createPresetMutation.error as Error).message}
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
                         <span className={m.enabled ? 'badge badge--success' : 'badge badge--warning'}>
                           {m.enabled ? t('presets.enabled') : t('presets.disabled')}
                         </span>
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
