import type { MemoryCandidateFact, MemoryFactAction } from '../../types/memory'
import { actionLabel, candidateRowKey } from './memoryUtils'

interface BatchConfirmDialogProps {
  pendingBatchAction: MemoryFactAction
  pendingBatchFacts: MemoryCandidateFact[]
  pendingBatchLabel: string
  pendingBatchCount: number
  t: (key: string, options?: Record<string, unknown>) => string
  onCancel: () => void
  onConfirm: () => void
}

export function BatchConfirmDialog({
  pendingBatchAction, pendingBatchFacts, pendingBatchLabel, pendingBatchCount,
  t, onCancel, onConfirm,
}: BatchConfirmDialogProps) {
  return (
    <div className="dialog-backdrop" role="dialog" aria-modal="true" aria-label={t('memory.confirmBatchTitle', { action: pendingBatchLabel })}>
      <div className="dialog-card">
        <div className="dialog-card__header">
          <div>
            <h2>{t('memory.confirmBatchTitle', { action: pendingBatchLabel })}</h2>
            <p>{t('memory.confirmBatchDesc', { count: pendingBatchCount, action: pendingBatchLabel })}</p>
          </div>
          <button type="button" onClick={onCancel}>
            {t('common.close')}
          </button>
        </div>
        <div className="memory-governance__confirm-list">
          {pendingBatchFacts.slice(0, 5).map((row) => (
            <div key={candidateRowKey(row)}>
              <strong>{row.fact_key}</strong>
              <small>{row.fact_value} · {row.status}</small>
            </div>
          ))}
          {pendingBatchFacts.length > 5 ? (
            <div>{t('memory.remainingOmitted', { count: pendingBatchFacts.length - 5 })}</div>
          ) : null}
        </div>
        <div className="dialog-card__actions">
          <button type="button" onClick={onCancel}>
            {t('common.cancel')}
          </button>
          <button type="button" onClick={onConfirm}>
            {t('memory.confirmBatchAction', { action: pendingBatchLabel })}
          </button>
        </div>
      </div>
    </div>
  )
}
