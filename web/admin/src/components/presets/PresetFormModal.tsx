import { FormEvent, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

type PresetFormState = {
  name: string
  template: string
  description: string
  variables: string
}

type PresetFormModalProps = {
  open: boolean
  onClose: () => void
  onSubmit: (e: FormEvent) => void
  form: PresetFormState
  onFormChange: (form: PresetFormState) => void
  loading: boolean
  isEditing: boolean
  error?: Error | null
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

export function PresetFormModal({
  open,
  onClose,
  onSubmit,
  form,
  onFormChange,
  loading,
  isEditing,
  error,
}: PresetFormModalProps) {
  const { t } = useTranslation()

  const variablePreview = useMemo(
    () => previewTemplate(form.template, form.variables),
    [form.template, form.variables],
  )

  if (!open) return null

  return (
    <form
      className="page-surface"
      onSubmit={onSubmit}
      style={{ marginBottom: '1.5rem' }}
    >
      <h3 style={{ margin: '0 0 1rem', fontSize: '1rem' }}>
        {isEditing ? t('presets.editPreset') : t('presets.newPreset')}
      </h3>
      <div className="system-config-grid">
        <label>
          {t('presets.name')}
          <input
            value={form.name}
            onChange={(e) => onFormChange({ ...form, name: e.target.value })}
            placeholder={t('presets.namePlaceholder')}
            required
          />
        </label>
        <label>
          {t('presets.description')}
          <input
            value={form.description}
            onChange={(e) => onFormChange({ ...form, description: e.target.value })}
            placeholder={t('presets.descriptionPlaceholder')}
          />
        </label>
        <label style={{ gridColumn: '1 / -1' }}>
          {t('presets.systemPrompt')}
          <textarea
            rows={4}
            value={form.template}
            onChange={(e) => onFormChange({ ...form, template: e.target.value })}
            placeholder={t('presets.systemPromptPlaceholder')}
            required
          />
        </label>
        <label style={{ gridColumn: '1 / -1' }}>
          {t('presets.variables')}
          <input
            value={form.variables}
            onChange={(e) => onFormChange({ ...form, variables: e.target.value })}
            placeholder={t('presets.variablesPlaceholder')}
          />
        </label>
        {form.variables.trim() && form.template.trim() ? (
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
          disabled={loading}
        >
          {isEditing
            ? (loading ? t('presets.updating') : t('common.save'))
            : (loading ? t('presets.creating') : t('common.create'))}
        </button>
        <button
          type="button"
          className="btn btn--outline"
          onClick={onClose}
        >
          {t('common.cancel')}
        </button>
      </div>
      {error ? (
        <div className="config-error" style={{ marginTop: '0.75rem' }}>
          {error.message}
        </div>
      ) : null}
    </form>
  )
}
