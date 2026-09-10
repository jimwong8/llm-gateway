import type { MemoryFactAction } from '../../types/memory'
import { actionLabel } from './memoryUtils'

interface BatchActionBarProps {
  selectedCount: number
  totalCount: number
  filteredCount: number
  fetchedCount: number
  confirmable: number
  rejectable: number
  promotable: number
  batchActionSubmitting: MemoryFactAction | null
  t: (key: string, options?: Record<string, unknown>) => string
  onBatchAction: (action: MemoryFactAction) => void
}

export function BatchActionBar({
  selectedCount, totalCount, filteredCount, fetchedCount,
  confirmable, rejectable, promotable,
  batchActionSubmitting, t, onBatchAction,
}: BatchActionBarProps) {
  return (
    <section className="event-state memory-governance__batch-toolbar" aria-label={t('memory.batchOperations')}>
      <div>
        <strong>{t('memory.batch')}</strong>
        <div>
          {t('memory.batchSummary', {
            selected: selectedCount,
            total: totalCount,
            filtered: filteredCount,
            fetched: fetchedCount,
            confirmable,
            rejectable,
            promotable,
          })}
        </div>
      </div>
      <div className="policy-actions">
        <button
          type="button"
          className="rollouts-action"
          onClick={() => onBatchAction('confirm')}
          disabled={batchActionSubmitting !== null || confirmable === 0}
        >
          {batchActionSubmitting === 'confirm' ? t('memory.batchConfirming') : t('memory.batchConfirm')}
        </button>
        <button
          type="button"
          className="rollouts-action"
          onClick={() => onBatchAction('reject')}
          disabled={batchActionSubmitting !== null || rejectable === 0}
        >
          {batchActionSubmitting === 'reject' ? t('memory.batchRejecting') : t('memory.batchReject')}
        </button>
        <button
          type="button"
          className="rollouts-action"
          onClick={() => onBatchAction('promote')}
          disabled={batchActionSubmitting !== null || promotable === 0}
        >
          {batchActionSubmitting === 'promote' ? t('memory.batchPromoting') : t('memory.batchPromote')}
        </button>
      </div>
    </section>
  )
}
