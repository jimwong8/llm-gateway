import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

type RollbackDialogProps = {
  open: boolean
  onClose: () => void
  onSubmit: (e: FormEvent<HTMLFormElement>) => void
  rolloutID: string
  environment: string
  actor: string
  onActorChange: (value: string) => void
  reason: string
  onReasonChange: (value: string) => void
  loading: boolean
  error: string
}

export function RollbackDialog({
  open,
  onClose,
  onSubmit,
  rolloutID,
  environment,
  actor,
  onActorChange,
  reason,
  onReasonChange,
  loading,
  error,
}: RollbackDialogProps) {
  const { t } = useTranslation()

  if (!open) return null

  return (
    <div className="dialog-backdrop" role="presentation">
      <section
        className="dialog-card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="rollback-dialog-title"
      >
        <div className="dialog-card__header">
          <div>
            <h2 id="rollback-dialog-title">{t('rollouts.rollbackTitle')}</h2>
            <p>Rollout ID: {rolloutID} · Environment: {environment}</p>
          </div>
          <button type="button" onClick={onClose}>
            {t('common.close')}
          </button>
        </div>

        <form className="release-panel__grid" aria-label={t('rollouts.rollbackFormLabel')} onSubmit={onSubmit}>
          <label>
            {t('rollouts.actor')}
            <input value={actor} onChange={(event) => onActorChange(event.target.value)} />
          </label>
          <label>
            {t('rollouts.reason')}
            <input value={reason} onChange={(event) => onReasonChange(event.target.value)} />
          </label>
          {error ? <div className="config-error">{error}</div> : null}
          <div className="dialog-card__actions">
            <button type="button" onClick={onClose}>{t('common.cancel')}</button>
            <button type="submit" disabled={loading}>{loading ? t('rollouts.rollingBack') : t('rollouts.confirmRollback')}</button>
          </div>
        </form>
      </section>
    </div>
  )
}
