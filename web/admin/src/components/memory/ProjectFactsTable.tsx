import { formatDate } from '../../lib/format'
import type { MemoryProjectFact } from '../../types/memory'
import type { CandidateSortField, SortDirection } from './memoryUtils'
import { pageSizeOptions, sortIndicator } from './memoryUtils'


interface ProjectFactsTableProps {
  data: MemoryProjectFact[]
  selectedFact: { kind: 'project'; fact: MemoryProjectFact } | null
  projectSortField: CandidateSortField
  projectSortDirection: SortDirection
  projectLocalQuery: string
  setProjectLocalQuery: (q: string) => void
  projectPageSize: number
  setProjectPageSize: (s: number) => void
  projectPage: number
  setProjectPage: React.Dispatch<React.SetStateAction<number>>
  projectPageCount: number
  projectPagination: { start: number; end: number; total: number }
  t: (key: string, options?: Record<string, unknown>) => string
  onSort: (field: CandidateSortField) => void
  onRowClick: (row: MemoryProjectFact) => void
}

export function ProjectFactsTable({
  data, selectedFact,
  projectSortField, projectSortDirection,
  projectLocalQuery, setProjectLocalQuery,
  projectPageSize, setProjectPageSize,
  projectPage, setProjectPage,
  projectPageCount, projectPagination,
  t, onSort, onRowClick,
}: ProjectFactsTableProps) {
  return (
    <>
      <section className="event-state memory-governance__table-toolbar" aria-label={t('memory.projectLocalControl')}>
        <div className="memory-governance__table-toolbar-fields">
          <label>
            {t('memory.localSearch')}
            <input
              value={projectLocalQuery}
              onChange={(event) => setProjectLocalQuery(event.target.value)}
              placeholder={t('memory.localSearchPlaceholder')}
            />
          </label>
          <label>
            {t('memory.pageSize')}
            <select value={String(projectPageSize)} onChange={(event) => setProjectPageSize(Number(event.target.value))}>
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
              start: projectPagination.start,
              end: projectPagination.end,
              total: projectPagination.total,
              fetched: data.length,
            })}
          </span>
        </div>
      </section>

      <section className="event-table" aria-label={t('memory.projectTable')}>
        <table>
          <thead>
            <tr>
              <th>{t('memory.factKeyHeader')}</th>
              <th>{t('memory.value')}</th>
              <th>{t('memory.tenant')}</th>
              <th>{t('memory.user')}</th>
              <th>
                <button
                  type="button"
                  className="policy-select"
                  aria-sort={projectSortField === 'status' ? (projectSortDirection === 'desc' ? 'descending' : 'ascending') : 'none'}
                  onClick={() => onSort('status')}
                >
                  {t('memory.status')} {sortIndicator(projectSortField, projectSortDirection, 'status')}
                </button>
              </th>
              <th>{t('memory.sourceSeq')}</th>
              <th>{t('memory.lastVerified')}</th>
              <th>
                <button
                  type="button"
                  className="policy-select"
                  aria-sort={projectSortField === 'updated_at' ? (projectSortDirection === 'desc' ? 'descending' : 'ascending') : 'none'}
                  onClick={() => onSort('updated_at')}
                >
                  {t('memory.updatedAt')} {sortIndicator(projectSortField, projectSortDirection, 'updated_at')}
                </button>
              </th>
            </tr>
          </thead>
          <tbody>
            {data.map((row) => (
              <tr
                key={`${row.fact_key}:${row.tenant_id}:${row.user_id}:${row.updated_at ?? row.id}`}
                className={selectedFact?.kind === 'project' && selectedFact.fact.id === row.id ? 'memory-row memory-row--selected' : 'memory-row'}
                onClick={() => onRowClick(row)}
              >
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
                    <small>{row.superseded_by ? t('memory.supersededBy', { id: row.superseded_by }) : t('memory.currentlyActive')}</small>
                  </div>
                </td>
                <td>
                  <div className="memory-fact-cell memory-fact-cell--compact">
                    <strong>{row.source_message_seq}</strong>
                    <small>{t('memory.verifiedAt', { date: formatDate(row.last_verified_at) })}</small>
                  </div>
                </td>
                <td>{formatDate(row.last_verified_at)}</td>
                <td>{formatDate(row.updated_at)}</td>
              </tr>
            ))}
            {data.length === 0 ? (
              <tr>
                <td colSpan={8}>{t('memory.noProjectFacts')}</td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </section>

      <div className="memory-governance__pager" role="group" aria-label={t('memory.projectPagination')}>
        <span>
          {t('memory.paginationSummary', {
            page: projectPage, totalPages: projectPageCount,
            start: projectPagination.start, end: projectPagination.end, total: projectPagination.total,
          })}
        </span>
        <div className="policy-actions">
          <button
            type="button" className="rollouts-action"
            onClick={() => setProjectPage((value) => Math.max(1, value - 1))}
            disabled={projectPage === 1}
          >
            {t('memory.prevPage')}
          </button>
          <button
            type="button" className="rollouts-action"
            onClick={() => setProjectPage((value) => Math.min(projectPageCount, value + 1))}
            disabled={projectPage === projectPageCount}
          >
            {t('memory.nextPage')}
          </button>
        </div>
      </div>
    </>
  )
}
