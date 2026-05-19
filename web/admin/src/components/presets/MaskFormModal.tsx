import { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

type MaskFormState = {
  name: string
  pattern: string
  replacement: string
}

type MaskFormModalProps = {
  open: boolean
  onClose: () => void
  onSubmit: (e: FormEvent) => void
  form: MaskFormState
  onFormChange: (form: MaskFormState) => void
  loading: boolean
  error?: Error | null
}

export function MaskFormModal({
  open,
  onClose,
  onSubmit,
  form,
  onFormChange,
  loading,
  error,
}: MaskFormModalProps) {
  const { t } = useTranslation()

  if (!open) return null

  return (
    <form className="page-surface" onSubmit={onSubmit} style={{ marginBottom: '1.5rem' }}>
      <h3 style={{ margin: '0 0 1rem', fontSize: '1rem' }}>{t('presets.newMaskRule')}</h3>
      <div className="system-config-grid">
        <label>
          {t('presets.name')}
          <input
            value={form.name}
            onChange={(e) => onFormChange({ ...form, name: e.target.value })}
            placeholder={t('presets.maskNamePlaceholder')}
            required
          />
        </label>
        <label>
          {t('presets.replacement')}
          <input
            value={form.replacement}
            onChange={(e) => onFormChange({ ...form, replacement: e.target.value })}
            placeholder="***"
          />
        </label>
        <label style={{ gridColumn: '1 / -1' }}>
          {t('presets.pattern')}
          <input
            value={form.pattern}
            onChange={(e) => onFormChange({ ...form, pattern: e.target.value })}
            placeholder={t('presets.patternPlaceholder')}
            required
          />
        </label>
      </div>
      <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1rem' }}>
        <button type="submit" className="btn btn--primary" disabled={loading}>
          {loading ? t('presets.creating') : t('common.create')}
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
