import { formatDate } from '../../lib/format'
import type { SelectedFact } from './memoryUtils'
import { actionLabel, isActionAllowed } from './memoryUtils'

interface FactDetailPanelProps {
  selectedFact: SelectedFact | null
  t: (key: string, options?: Record<string, unknown>) => string
}

export function FactDetailPanel({ selectedFact, t }: FactDetailPanelProps) {
  return (
    <section className="memory-governance__detail event-state" aria-label={t('memory.factDetail')}>
      <strong>{selectedFact ? t('memory.factDetail') : t('memory.selectFactForDetail')}</strong>
      {selectedFact ? (
        <div className="memory-governance__detail-grid">
          <div>
            <span>{t('memory.type')}</span>
            <strong>{selectedFact.kind === 'candidate' ? 'Candidate Fact' : 'Project Fact'}</strong>
          </div>
          <div>
            <span>Fact Key</span>
            <strong>{selectedFact.fact.fact_key}</strong>
          </div>
          <div>
            <span>Tenant</span>
            <strong>{selectedFact.fact.tenant_id || '—'}</strong>
          </div>
          <div>
            <span>User</span>
            <strong>{selectedFact.fact.user_id}</strong>
          </div>
          <div>
            <span>Status</span>
            <strong>{selectedFact.fact.status}</strong>
          </div>
          <div>
            <span>Source Seq</span>
            <strong>{selectedFact.fact.source_message_seq}</strong>
          </div>
          <div className="memory-governance__detail-wide">
            <span>Value</span>
            <strong>{selectedFact.fact.fact_value}</strong>
          </div>
          <div className="memory-governance__detail-wide">
            <span>Source Text</span>
            <pre>{selectedFact.fact.source_text || t('memory.noSourceExcerpt')}</pre>
          </div>
          {selectedFact.kind === 'candidate' ? (
            <>
              <div>
                <span>Confirmations</span>
                <strong>{selectedFact.fact.confirmation_count}</strong>
              </div>
              <div>
                <span>Allowed Actions</span>
                <strong>
                  {(['confirm', 'reject', 'promote'] as const)
                    .filter((action) => isActionAllowed(selectedFact.fact.status, action))
                    .map((action) => actionLabel(action, t))
                    .join(' / ') || t('memory.readOnly')}
                </strong>
              </div>
            </>
          ) : (
            <>
              <div>
                <span>Superseded By</span>
                <strong>{selectedFact.fact.superseded_by ? `#${selectedFact.fact.superseded_by}` : t('memory.currentlyActive')}</strong>
              </div>
              <div>
                <span>Last Verified</span>
                <strong>{formatDate(selectedFact.fact.last_verified_at)}</strong>
              </div>
            </>
          )}
          <div>
            <span>Updated At</span>
            <strong>{formatDate(selectedFact.fact.updated_at)}</strong>
          </div>
          <div>
            <span>Created At</span>
            <strong>{formatDate(selectedFact.fact.created_at)}</strong>
          </div>
        </div>
      ) : (
        <div>{t('memory.clickRowHint')}</div>
      )}
    </section>
  )
}
