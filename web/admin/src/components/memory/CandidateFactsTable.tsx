import { formatDate } from '../../lib/format'
import type {
  MemoryCandidateFact,
  MemoryFactAction,
} from '../../types/memory'
import type { CandidateSortField, SortDirection } from './memoryUtils'
import {
  actionLabel,
  candidateRowKey,
  isActionAllowed,
  pageSizeOptions,
  sortIndicator,
} from './memoryUtils'


interface CandidateFactsTableProps {
  data: MemoryCandidateFact[]
  selectedFact: { kind: 'candidate'; fact: MemoryCandidateFact } | null
  selectedCandidateKeySet: Set<string>
  actionSubmitting: Record<string, boolean>
  batchActionSubmitting: MemoryFactAction | null
  candidateSortField: CandidateSortField
  candidateSortDirection: SortDirection
  candidateLocalQuery: string
  setCandidateLocalQuery: (q: string) => void
  candidatePageSize: number
  setCandidatePageSize: (s: number) => void
  candidatePage: number
  setCandidatePage: React.Dispatch<React.SetStateAction<number>>
  candidatePageCount: number
  candidatePagination: { start: number; end: number; total: number }
  allVisibleSelected: boolean
  selectAllVisibleRef: React.RefObject<HTMLInputElement>
  t: (key: string, options?: Record<string, unknown>) => string
  onSort: (field: CandidateSortField) => void
  onSelect: (row: MemoryCandidateFact, checked: boolean) => void
  onToggleSelectAll: (checked: boolean) => void
  onAction: (row: MemoryCandidateFact, action: MemoryFactAction) => void
  onRowClick: (row: MemoryCandidateFact) => void
}

export function CandidateFactsTable({
  data, selectedFact, selectedCandidateKeySet, actionSubmitting, batchActionSubmitting,
  candidateSortField, candidateSortDirection,
  candidateLocalQuery, setCandidateLocalQuery,
  candidatePageSize, setCandidatePageSize,
  candidatePage, setCandidatePage,
  candidatePageCount, candidatePagination,
  allVisibleSelected, selectAllVisibleRef,
  t, onSort, onSelect, onToggleSelectAll, onAction, onRowClick,
}: CandidateFactsTableProps) {
  return (
    <>
      <section className="event-state memory-governance__table-toolbar" aria-label={t('memory.candidateLocalControl')}>
        <div className="memory-governance__table-toolbar-fields">
          <label>
            {t('memory.localSearch')}
            <input
              value={candidateLocalQuery}
              onChange={(event) => setCandidateLocalQuery(event.target.value)}
              placeholder={t('memory.localSearchPlaceholder')}
            />
          </label>
          <label>
            {t('memory.pageSize')}
            <select value={String(candidatePageSize)} onChange={(event) => setCandidatePageSize(Number(event.target.value))}>
              {pageSizeOptions.map((size) => (
                <option key={size} value={size}>{size}</option>
              ))}
            </select>
          </label>
        </div>
        <div className="memory-governance__scope-summary">
          <strong>{t('memory.visibleScope')}</strong>
          <span>
            {t('memory.scopeSummary', {
              start: candidatePagination.start,
              end: candidatePagination.end,
              total: candidatePagination.total,
              fetched: data.length,
            })}
          </span>
        </div>
      </section>

      <section className="event-table" aria-label={t('memory.candidateTable')}>
        <table>
          <thead>
            <tr>
              <th className="memory-governance__selection-cell">
                <input
                  ref={selectAllVisibleRef}
                  type="checkbox"
                  checked={allVisibleSelected}
                  onChange={(event) => onToggleSelectAll(event.target.checked)}
                  aria-label={t('memory.selectAllVisible')}
                />
              </th>
              <th>{t('memory.factKeyHeader')}</th>
              <th>{t('memory.value')}</th>
              <th>{t('memory.tenant')}</th>
              <th>{t('memory.user')}</th>
              <th>
                <button
                  type="button"
                  className="policy-select"
                  aria-sort={candidateSortField === 'status' ? (candidateSortDirection === 'desc' ? 'descending' : 'ascending') : 'none'}
                  onClick={() => onSort('status')}
                >
                  {t('memory.status')} {sortIndicator(candidateSortField, candidateSortDirection, 'status')}
                </button>
              </th>
              <th>{t('memory.confirmCount')}</th>
              <th>{t('memory.sourceSeq')}</th>
              <th>
                <button
                  type="button"
                  className="policy-select"
                  aria-sort={candidateSortField === 'updated_at' ? (candidateSortDirection === 'desc' ? 'descending' : 'ascending') : 'none'}
                  onClick={() => onSort('updated_at')}
                >
                  {t('memory.updatedAt')} {sortIndicator(candidateSortField, candidateSortDirection, 'updated_at')}
                </button>
              </th>
              <th>{t('memory.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {data.map((row) => {
              const confirmKey = `confirm:${row.fact_key}:${row.tenant_id}:${row.user_id}`
              const rejectKey = `reject:${row.fact_key}:${row.tenant_id}:${row.user_id}`
              const promoteKey = `promote:${row.fact_key}:${row.tenant_id}:${row.user_id}`
              const rowKey = candidateRowKey(row)

              return (
                <tr
                  key={`${row.fact_key}:${row.tenant_id}:${row.user_id}:${row.updated_at ?? row.id}`}
                  className={selectedFact?.kind === 'candidate' && selectedFact.fact.id === row.id ? 'memory-row memory-row--selected' : 'memory-row'}
                  onClick={() => onRowClick(row)}
                >
                  <td className="memory-governance__selection-cell">
                    <input
                      type="checkbox"
                      checked={selectedCandidateKeySet.has(rowKey)}
                      onChange={(event) => onSelect(row, event.target.checked)}
                      onClick={(event) => event.stopPropagation()}
                      aria-label={t('memory.selectCandidate', { key: row.fact_key })}
                    />
                  </td>
                  <td>{row.fact_key}</td>
                  <td>
                    <div className="memory-fact-cell">
                      <strong>{row.fact_value}</strong>
                      <small>{row.source_text || t('memory.noSourceExcerpt')}</small>
                    </div>
                  </td>
                  <td>{row.tenant_id || '—'}</td>
                  <td>{row.user_id}</td>
                  <td>
                    <div className="memory-fact-cell">
                      <span className={`status-pill ${row.status}`}>{row.status}</span>
                      <small>{t('memory.lastUpdated', { date: formatDate(row.updated_at) })}</small>
                    </div>
                  </td>
                  <td>
                    <div className="memory-fact-cell memory-fact-cell--compact">
                      <strong>{row.confirmation_count}</strong>
                      <small>{t('memory.sourceSeqLabel', { seq: row.source_message_seq })}</small>
                    </div>
                  </td>
                  <td>{row.source_message_seq}</td>
                  <td>{formatDate(row.updated_at)}</td>
                  <td>
                    <div className="policy-actions">
                      <button
                        type="button"
                        className="rollouts-action"
                        onClick={(event) => { event.stopPropagation(); void onAction(row, 'confirm') }}
                        disabled={!isActionAllowed(row.status, 'confirm') || Boolean(actionSubmitting[confirmKey]) || batchActionSubmitting !== null}
                        title={isActionAllowed(row.status, 'confirm') ? '' : t('memory.onlyPendingConfirm')}
                      >
                        {actionSubmitting[confirmKey] ? t('memory.confirming') : t('memory.confirm')}
                      </button>
                      <button
                        type="button"
                        className="rollouts-action"
                        onClick={(event) => { event.stopPropagation(); void onAction(row, 'reject') }}
                        disabled={!isActionAllowed(row.status, 'reject') || Boolean(actionSubmitting[rejectKey]) || batchActionSubmitting !== null}
                        title={isActionAllowed(row.status, 'reject') ? '' : t('memory.onlyPendingConfirmedReject')}
                      >
                        {actionSubmitting[rejectKey] ? t('memory.rejecting') : t('memory.reject')}
                      </button>
                      <button
                        type="button"
                        className="rollouts-action"
                        onClick={(event) => { event.stopPropagation(); void onAction(row, 'promote') }}
                        disabled={!isActionAllowed(row.status, 'promote') || Boolean(actionSubmitting[promoteKey]) || batchActionSubmitting !== null}
                        title={isActionAllowed(row.status, 'promote') ? '' : t('memory.onlyConfirmedPromote')}
                      >
                        {actionSubmitting[promoteKey] ? t('memory.promoting') : t('memory.promote')}
                      </button>
                    </div>
                  </td>
                </tr>
              )
            })}
            {data.length === 0 ? (
              <tr>
                <td colSpan={10}>{t('memory.noCandidateFacts')}</td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </section>

      <div className="memory-governance__pager" role="group" aria-label={t('memory.candidatePagination')}>
        <span>
          {t('memory.paginationSummary', {
            page: candidatePage, totalPages: candidatePageCount,
            start: candidatePagination.start, end: candidatePagination.end, total: candidatePagination.total,
          })}
        </span>
        <div className="policy-actions">
          <button
            type="button" className="rollouts-action"
            onClick={() => setCandidatePage((value) => Math.max(1, value - 1))}
            disabled={candidatePage === 1}
          >
            {t('memory.prevPage')}
          </button>
          <button
            type="button" className="rollouts-action"
            onClick={() => setCandidatePage((value) => Math.min(candidatePageCount, value + 1))}
            disabled={candidatePage === candidatePageCount}
          >
            {t('memory.nextPage')}
          </button>
        </div>
      </div>
    </>
  )
}
