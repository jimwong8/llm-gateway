import type { FormEvent } from 'react'
import type { MemoryFactFilters } from '../../types/memory'

const candidateStatuses = ['', 'pending', 'confirmed', 'promoted', 'rejected']
const projectStatuses = ['', 'active', 'superseded']

interface MemoryFiltersProps {
  draftFilters: MemoryFactFilters
  setDraftFilters: React.Dispatch<React.SetStateAction<MemoryFactFilters>>
  draftCandidateStatus: string
  setDraftCandidateStatus: (s: string) => void
  draftProjectStatus: string
  setDraftProjectStatus: (s: string) => void
  t: (key: string, options?: Record<string, unknown>) => string
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onReset: () => void
}

export function MemoryFilters({
  draftFilters, setDraftFilters,
  draftCandidateStatus, setDraftCandidateStatus,
  draftProjectStatus, setDraftProjectStatus,
  t, onSubmit, onReset,
}: MemoryFiltersProps) {
  return (
    <form className="config-filters" aria-label={t('memory.filterLabel')} onSubmit={onSubmit}>
      <label>
        {t('memory.tenantId')}
        <input
          value={draftFilters.tenant_id}
          onChange={(event) => setDraftFilters((previous) => ({ ...previous, tenant_id: event.target.value }))}
          placeholder={t('memory.tenantPlaceholder')}
        />
      </label>
      <label>
        {t('memory.userId')}
        <input
          value={draftFilters.user_id}
          onChange={(event) => setDraftFilters((previous) => ({ ...previous, user_id: event.target.value }))}
          placeholder={t('memory.userPlaceholder')}
        />
      </label>
      <label>
        {t('memory.candidateStatus')}
        <select value={draftCandidateStatus} onChange={(event) => setDraftCandidateStatus(event.target.value)}>
          {candidateStatuses.map((status) => (
            <option key={status || 'all'} value={status}>
              {status || 'all'}
            </option>
          ))}
        </select>
      </label>
      <label>
        {t('memory.projectStatus')}
        <select value={draftProjectStatus} onChange={(event) => setDraftProjectStatus(event.target.value)}>
          {projectStatuses.map((status) => (
            <option key={status || 'all'} value={status}>
              {status || 'all'}
            </option>
          ))}
        </select>
      </label>
      <div className="config-filters__actions">
        <button type="submit">{t('memory.refreshFacts')}</button>
        <button type="button" className="rollouts-action" onClick={onReset}>
          {t('memory.resetFilters')}
        </button>
      </div>
    </form>
  )
}
