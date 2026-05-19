import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { ApiError } from '../lib/http'
import {
  listMemoryCandidateFacts,
  listMemoryProjectFacts,
  searchMemory,
  submitMemoryCandidateFactAction,
  submitMemoryCandidateFactBatchAction,
} from '../lib/memory'
import type {
  MemoryCandidateFact,
  MemoryCandidateFactActionRequest,
  MemoryCandidateFactBatchActionResult,
  MemoryCandidateFactFilters,
  MemoryFactAction,
  MemoryFactFilters,
  MemoryProjectFact,
  MemoryProjectFactFilters,
  MemorySearchResult,
} from '../types/memory'

const initialFilters: MemoryFactFilters = {
  tenant_id: '',
  user_id: '',
}

const initialCandidateFilters: MemoryCandidateFactFilters = {
  ...initialFilters,
  status: '',
}

const initialProjectFilters: MemoryProjectFactFilters = {
  ...initialFilters,
  status: '',
}

const candidateStatuses = ['', 'pending', 'confirmed', 'promoted', 'rejected']
const projectStatuses = ['', 'active', 'superseded']
const pageSizeOptions = [10, 25, 50]

type CandidateSortField = 'status' | 'updated_at'
type SortDirection = 'asc' | 'desc'
type SelectedFact =
  | { kind: 'candidate'; fact: MemoryCandidateFact }
  | { kind: 'project'; fact: MemoryProjectFact }

type PaginationSummary = {
  start: number
  end: number
  total: number
}

function formatDate(value?: string) {
  if (!value) {
    return '—'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

function buildActionPayload(filters: MemoryFactFilters, row: MemoryCandidateFact): MemoryCandidateFactActionRequest {
  return {
    tenant_id: filters.tenant_id.trim() || row.tenant_id,
    user_id: filters.user_id.trim() || row.user_id,
  }
}

function actionLabel(action: MemoryFactAction, t?: (key: string) => string) {
  switch (action) {
    case 'confirm':
      return t ? t('memory.actionConfirm') : '确认'
    case 'reject':
      return t ? t('memory.actionReject') : '拒绝'
    case 'promote':
      return t ? t('memory.actionPromote') : '提升'
    default:
      return action
  }
}

function isActionAllowed(status: string, action: MemoryFactAction) {
  const normalized = status.trim().toLowerCase()
  switch (action) {
    case 'confirm':
      return normalized === 'pending'
    case 'reject':
      return normalized === 'pending' || normalized === 'confirmed'
    case 'promote':
      return normalized === 'confirmed'
    default:
      return true
  }
}

function parseSortTime(value?: string) {
  if (!value) {
    return 0
  }

  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 0 : date.getTime()
}

function candidateSortPriority(status: string) {
  switch (status.trim().toLowerCase()) {
    case 'pending':
      return 0
    case 'confirmed':
      return 1
    case 'promoted':
      return 2
    case 'rejected':
      return 3
    default:
      return 99
  }
}

function projectSortPriority(status: string) {
  switch (status.trim().toLowerCase()) {
    case 'active':
      return 0
    case 'superseded':
      return 1
    default:
      return 99
  }
}

function compareCandidateSortField(
  a: MemoryCandidateFact,
  b: MemoryCandidateFact,
  field: CandidateSortField,
  direction: SortDirection,
) {
  let compareValue = 0

  if (field === 'status') {
    compareValue = candidateSortPriority(a.status) - candidateSortPriority(b.status)
    if (compareValue === 0) {
      compareValue = a.status.localeCompare(b.status)
    }
  } else {
    compareValue = parseSortTime(a.updated_at) - parseSortTime(b.updated_at)
  }

  if (compareValue === 0) {
    compareValue = a.fact_key.localeCompare(b.fact_key)
  }

  return direction === 'asc' ? compareValue : -compareValue
}

function compareProjectSortField(
  a: MemoryProjectFact,
  b: MemoryProjectFact,
  field: CandidateSortField,
  direction: SortDirection,
) {
  let compareValue = 0

  if (field === 'status') {
    compareValue = projectSortPriority(a.status) - projectSortPriority(b.status)
    if (compareValue === 0) {
      compareValue = a.status.localeCompare(b.status)
    }
  } else {
    compareValue = parseSortTime(a.updated_at) - parseSortTime(b.updated_at)
  }

  if (compareValue === 0) {
    compareValue = a.fact_key.localeCompare(b.fact_key)
  }

  return direction === 'asc' ? compareValue : -compareValue
}

function nextSortDirection(
  currentField: CandidateSortField,
  currentDirection: SortDirection,
  nextField: CandidateSortField,
): SortDirection {
  if (currentField !== nextField) {
    return 'desc'
  }
  return currentDirection === 'desc' ? 'asc' : 'desc'
}

function sortIndicator(activeField: CandidateSortField, activeDirection: SortDirection, field: CandidateSortField) {
  if (activeField !== field) {
    return '↕'
  }
  return activeDirection === 'desc' ? '↓' : '↑'
}

function matchesLocalFactQuery(fields: Array<string | number | undefined>, query: string) {
  const normalizedQuery = query.trim().toLowerCase()
  if (!normalizedQuery) {
    return true
  }

  return fields.some((value) => String(value ?? '').toLowerCase().includes(normalizedQuery))
}

function candidateRowKey(row: MemoryCandidateFact) {
  return `${row.id}:${row.fact_key}:${row.tenant_id}:${row.user_id}`
}

function candidateActionKey(row: MemoryCandidateFact, action: MemoryFactAction) {
  return `${action}:${row.fact_key}:${row.tenant_id}:${row.user_id}`
}

function buildBatchActionSummary(result: MemoryCandidateFactBatchActionResult) {
  const status = result.status ?? result.fact?.status ?? 'unknown'
  return `${result.fact_key}→${status}`
}

function buildBatchActionFailure(result: MemoryCandidateFactBatchActionResult, t: (key: string) => string) {
  const message = result.error?.message?.trim() || t('memory.actionFailed')
  return `${result.fact_key}：${message}`
}

function openBatchConfirmation(
  action: MemoryFactAction,
  facts: MemoryCandidateFact[],
  setActionError: (value: string) => void,
  setActionSuccess: (value: string) => void,
  setPendingBatchAction: (value: MemoryFactAction | null) => void,
  setPendingBatchFacts: (value: MemoryCandidateFact[]) => void,
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  if (facts.length === 0) {
    setActionError(t('memory.batchSelectRequired'))
    setActionSuccess('')
    return
  }

  const actionableRows = facts.filter((row) => isActionAllowed(row.status, action))
  if (actionableRows.length === 0) {
    setActionError(t('memory.batchNoActionable', { action: actionLabel(action, t) }))
    setActionSuccess('')
    return
  }

  setActionError('')
  setActionSuccess('')
  setPendingBatchAction(action)
  setPendingBatchFacts(actionableRows)
}

function buildPaginationSummary(total: number, page: number, pageSize: number): PaginationSummary {
  if (total === 0) {
    return {
      start: 0,
      end: 0,
      total,
    }
  }

  const start = (page - 1) * pageSize + 1
  const end = Math.min(page * pageSize, total)

  return {
    start,
    end,
    total,
  }
}

export function MemoryGovernancePage() {
  const { t: _t } = useTranslation()
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const t = _t as (key: string, options?: Record<string, unknown>) => string
  const [activeTab, setActiveTab] = useState<'governance' | 'search'>('governance')
  const [draftFilters, setDraftFilters] = useState<MemoryFactFilters>(initialFilters)
  const [draftCandidateStatus, setDraftCandidateStatus] = useState('')
  const [draftProjectStatus, setDraftProjectStatus] = useState('')
  const [candidateFilters, setCandidateFilters] = useState<MemoryCandidateFactFilters>(initialCandidateFilters)
  const [projectFilters, setProjectFilters] = useState<MemoryProjectFactFilters>(initialProjectFilters)
  const [actionSubmitting, setActionSubmitting] = useState<Record<string, boolean>>({})
  const [actionError, setActionError] = useState('')
  const [actionSuccess, setActionSuccess] = useState('')
  const [selectedFact, setSelectedFact] = useState<SelectedFact | null>(null)
  const [selectedCandidateKeys, setSelectedCandidateKeys] = useState<string[]>([])
  const [batchActionSubmitting, setBatchActionSubmitting] = useState<MemoryFactAction | null>(null)
  const [pendingBatchAction, setPendingBatchAction] = useState<MemoryFactAction | null>(null)
  const [pendingBatchFacts, setPendingBatchFacts] = useState<MemoryCandidateFact[]>([])
  const [candidateSortField, setCandidateSortField] = useState<CandidateSortField>('updated_at')
  const [candidateSortDirection, setCandidateSortDirection] = useState<SortDirection>('desc')
  const [projectSortField, setProjectSortField] = useState<CandidateSortField>('updated_at')
  const [projectSortDirection, setProjectSortDirection] = useState<SortDirection>('desc')
  const [candidateLocalQuery, setCandidateLocalQuery] = useState('')
  const [projectLocalQuery, setProjectLocalQuery] = useState('')
  const [candidatePageSize, setCandidatePageSize] = useState(10)
  const [projectPageSize, setProjectPageSize] = useState(10)
  const [candidatePage, setCandidatePage] = useState(1)
  const [projectPage, setProjectPage] = useState(1)
  const selectAllVisibleRef = useRef<HTMLInputElement | null>(null)

  // Hybrid Search state
  const [searchQuery, setSearchQuery] = useState('')
  const [searchTenantId, setSearchTenantId] = useState('')
  const [searchUserId, setSearchUserId] = useState('')
  const [searchResults, setSearchResults] = useState<MemorySearchResult[]>([])
  const [searchLoading, setSearchLoading] = useState(false)
  const [searchError, setSearchError] = useState('')
  const [searchSubmitted, setSearchSubmitted] = useState(false)

  const candidateFactsQuery = useQuery({
    queryKey: ['memory-candidate-facts', candidateFilters],
    queryFn: () => listMemoryCandidateFacts(candidateFilters),
  })

  const projectFactsQuery = useQuery({
    queryKey: ['memory-project-facts', projectFilters],
    queryFn: () => listMemoryProjectFacts(projectFilters),
  })

  const candidateFacts = useMemo(() => candidateFactsQuery.data?.data ?? [], [candidateFactsQuery.data])
  const projectFacts = useMemo(() => projectFactsQuery.data?.data ?? [], [projectFactsQuery.data])

  const sortedCandidateFacts = useMemo(
    () => [...candidateFacts].sort((left, right) => compareCandidateSortField(left, right, candidateSortField, candidateSortDirection)),
    [candidateFacts, candidateSortField, candidateSortDirection],
  )

  const filteredCandidateFacts = useMemo(
    () =>
      sortedCandidateFacts.filter((row) =>
        matchesLocalFactQuery(
          [row.fact_key, row.fact_value, row.source_text, row.tenant_id, row.user_id, row.status, row.source_message_seq],
          candidateLocalQuery,
        ),
      ),
    [sortedCandidateFacts, candidateLocalQuery],
  )

  const candidatePageCount = Math.max(1, Math.ceil(filteredCandidateFacts.length / candidatePageSize))
  const pagedCandidateFacts = useMemo(() => {
    const start = (candidatePage - 1) * candidatePageSize
    return filteredCandidateFacts.slice(start, start + candidatePageSize)
  }, [filteredCandidateFacts, candidatePage, candidatePageSize])

  const sortedProjectFacts = useMemo(
    () => [...projectFacts].sort((left, right) => compareProjectSortField(left, right, projectSortField, projectSortDirection)),
    [projectFacts, projectSortField, projectSortDirection],
  )

  const filteredProjectFacts = useMemo(
    () =>
      sortedProjectFacts.filter((row) =>
        matchesLocalFactQuery(
          [
            row.fact_key,
            row.fact_value,
            row.source_text,
            row.tenant_id,
            row.user_id,
            row.status,
            row.source_message_seq,
            row.superseded_by,
          ],
          projectLocalQuery,
        ),
      ),
    [sortedProjectFacts, projectLocalQuery],
  )

  const projectPageCount = Math.max(1, Math.ceil(filteredProjectFacts.length / projectPageSize))
  const pagedProjectFacts = useMemo(() => {
    const start = (projectPage - 1) * projectPageSize
    return filteredProjectFacts.slice(start, start + projectPageSize)
  }, [filteredProjectFacts, projectPage, projectPageSize])

  const visibleCandidateKeys = useMemo(() => pagedCandidateFacts.map(candidateRowKey), [pagedCandidateFacts])
  const visibleCandidateKeySet = useMemo(() => new Set(visibleCandidateKeys), [visibleCandidateKeys])
  const selectedCandidateKeySet = useMemo(() => new Set(selectedCandidateKeys), [selectedCandidateKeys])

  const selectedVisibleCandidateFacts = useMemo(
    () => pagedCandidateFacts.filter((row) => selectedCandidateKeySet.has(candidateRowKey(row))),
    [pagedCandidateFacts, selectedCandidateKeySet],
  )

  const selectedCandidateMetrics = useMemo(
    () => ({
      confirm: selectedVisibleCandidateFacts.filter((row) => isActionAllowed(row.status, 'confirm')).length,
      reject: selectedVisibleCandidateFacts.filter((row) => isActionAllowed(row.status, 'reject')).length,
      promote: selectedVisibleCandidateFacts.filter((row) => isActionAllowed(row.status, 'promote')).length,
    }),
    [selectedVisibleCandidateFacts],
  )

  const allVisibleSelected = pagedCandidateFacts.length > 0 && pagedCandidateFacts.every((row) => selectedCandidateKeySet.has(candidateRowKey(row)))
  const someVisibleSelected = pagedCandidateFacts.some((row) => selectedCandidateKeySet.has(candidateRowKey(row)))

  const pendingBatchCount = pendingBatchFacts.length
  const pendingBatchLabel = pendingBatchAction ? actionLabel(pendingBatchAction, t) : ''

  const metrics = useMemo(() => {
    const totalCandidates = candidateFacts.length
    const pendingCandidates = candidateFacts.filter((fact) => fact.status === 'pending').length
    const confirmedCandidates = candidateFacts.filter((fact) => fact.status === 'confirmed').length
    const promotedCandidates = candidateFacts.filter((fact) => fact.status === 'promoted').length
    const rejectedCandidates = candidateFacts.filter((fact) => fact.status === 'rejected').length
    const totalProjectFacts = projectFacts.length
    const activeProjectFacts = projectFacts.filter((fact) => fact.status === 'active').length
    const supersededProjectFacts = projectFacts.filter((fact) => fact.status === 'superseded').length

    return {
      totalCandidates,
      pendingCandidates,
      confirmedCandidates,
      promotedCandidates,
      rejectedCandidates,
      totalProjectFacts,
      activeProjectFacts,
      supersededProjectFacts,
    }
  }, [candidateFacts, projectFacts])

  const candidatePagination = useMemo(
    () => buildPaginationSummary(filteredCandidateFacts.length, candidatePage, candidatePageSize),
    [filteredCandidateFacts.length, candidatePage, candidatePageSize],
  )

  const projectPagination = useMemo(
    () => buildPaginationSummary(filteredProjectFacts.length, projectPage, projectPageSize),
    [filteredProjectFacts.length, projectPage, projectPageSize],
  )

  useEffect(() => {
    setSelectedCandidateKeys((previous) => previous.filter((key) => visibleCandidateKeySet.has(key)))
  }, [visibleCandidateKeySet])

  useEffect(() => {
    setCandidatePage(1)
  }, [candidateLocalQuery, candidatePageSize, candidateSortField, candidateSortDirection, candidateFilters])

  useEffect(() => {
    setProjectPage(1)
  }, [projectLocalQuery, projectPageSize, projectSortField, projectSortDirection, projectFilters])

  useEffect(() => {
    if (candidatePage > candidatePageCount) {
      setCandidatePage(candidatePageCount)
    }
  }, [candidatePage, candidatePageCount])

  useEffect(() => {
    if (projectPage > projectPageCount) {
      setProjectPage(projectPageCount)
    }
  }, [projectPage, projectPageCount])

  useEffect(() => {
    if (selectAllVisibleRef.current) {
      selectAllVisibleRef.current.indeterminate = !allVisibleSelected && someVisibleSelected
    }
  }, [allVisibleSelected, someVisibleSelected])

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const baseFilters = {
      tenant_id: draftFilters.tenant_id.trim(),
      user_id: draftFilters.user_id.trim(),
    }

    setCandidateFilters({
      ...baseFilters,
      status: draftCandidateStatus.trim(),
    })
    setProjectFilters({
      ...baseFilters,
      status: draftProjectStatus.trim(),
    })
    setActionError('')
    setActionSuccess('')
  }

  function handleResetFilters() {
    setDraftFilters(initialFilters)
    setDraftCandidateStatus('')
    setDraftProjectStatus('')
    setCandidateFilters(initialCandidateFilters)
    setProjectFilters(initialProjectFilters)
    setCandidateLocalQuery('')
    setProjectLocalQuery('')
    setCandidatePageSize(pageSizeOptions[0])
    setProjectPageSize(pageSizeOptions[0])
    setCandidatePage(1)
    setProjectPage(1)
    setSelectedCandidateKeys([])
    setActionError('')
    setActionSuccess('')
  }

  async function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!searchQuery.trim()) {
      setSearchError(t('memory.searchRequired'))
      return
    }
    setSearchLoading(true)
    setSearchError('')
    setSearchSubmitted(true)
    try {
      const response = await searchMemory({
        query: searchQuery.trim(),
        tenant_id: searchTenantId.trim() || undefined,
        user_id: searchUserId.trim() || undefined,
        limit: 20,
      })
      setSearchResults(response.results ?? [])
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setSearchError(unknownError.message)
      } else {
        setSearchError(unknownError instanceof Error ? unknownError.message : t('memory.searchFailed'))
      }
      setSearchResults([])
    } finally {
      setSearchLoading(false)
    }
  }

  function handleToggleCandidateSelection(row: MemoryCandidateFact, checked: boolean) {
    const key = candidateRowKey(row)
    setSelectedCandidateKeys((previous) => {
      if (checked) {
        return previous.includes(key) ? previous : [...previous, key]
      }
      return previous.filter((value) => value !== key)
    })
  }

  function handleToggleSelectAllVisible(checked: boolean) {
    setSelectedCandidateKeys((previous) => {
      if (checked) {
        return Array.from(new Set([...previous, ...visibleCandidateKeys]))
      }
      return previous.filter((key) => !visibleCandidateKeySet.has(key))
    })
  }

  async function handleCandidateAction(row: MemoryCandidateFact, action: MemoryFactAction) {
    const actionKey = candidateActionKey(row, action)
    setActionSubmitting((previous) => ({ ...previous, [actionKey]: true }))
    setActionError('')
    setActionSuccess('')

    try {
      const response = await submitMemoryCandidateFactAction(row.fact_key, action, buildActionPayload(candidateFilters, row))
      setActionSuccess(t('memory.actionSuccess', { action: actionLabel(action, t), factKey: response.fact_key, status: response.status }))
      setSelectedFact({ kind: 'candidate', fact: response })
      await Promise.all([candidateFactsQuery.refetch(), projectFactsQuery.refetch()])
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setActionError(t('memory.actionFailedDetail', { message: unknownError.message }))
      } else {
        setActionError(unknownError instanceof Error ? unknownError.message : t('memory.actionFailed'))
      }
    } finally {
      setActionSubmitting((previous) => ({ ...previous, [actionKey]: false }))
    }
  }

  async function handleBatchCandidateAction(
    action: MemoryFactAction,
    actionableRows: MemoryCandidateFact[] = selectedVisibleCandidateFacts.filter((row) => isActionAllowed(row.status, action)),
  ) {
    if (batchActionSubmitting) {
      return
    }

    if (selectedVisibleCandidateFacts.length === 0) {
      setActionError(t('memory.batchSelectRequired'))
      setActionSuccess('')
      return
    }

    if (actionableRows.length === 0) {
      setActionError(t('memory.batchNoActionable', { action: actionLabel(action, t) }))
      setActionSuccess('')
      return
    }

    setBatchActionSubmitting(action)
    setActionError('')
    setActionSuccess('')

    const failedKeys = new Set<string>()

    try {
      const response = await submitMemoryCandidateFactBatchAction(action, {
        items: actionableRows.map((row) => ({
          fact_key: row.fact_key,
          ...buildActionPayload(candidateFilters, row),
        })),
      })

      const successResults = response.results.filter((result) => !result.error)
      const failureResults = response.results.filter((result) => result.error)
      let latestSelectedFact: MemoryCandidateFact | null = null

      for (const row of actionableRows) {
        const matchedFailure = failureResults.find(
          (result) => result.fact_key === row.fact_key && result.user_id === buildActionPayload(candidateFilters, row).user_id,
        )
        if (matchedFailure) {
          failedKeys.add(candidateRowKey(row))
        }
      }

      if (successResults.length > 0) {
        const successPreview = successResults.slice(0, 3).map(buildBatchActionSummary).join('；')
        const hasMoreSuccess = successResults.length > 3 ? ' 等' : ''
        const failurePart = failureResults.length > 0 ? `，失败 ${failureResults.length} 条` : ''
        setActionSuccess(
          t('memory.batchSuccess', {
            action: actionLabel(action, t),
            successCount: successResults.length,
            failureCount: failureResults.length,
            preview: successPreview,
            failurePart,
            hasMore: hasMoreSuccess,
          }),
        )
        latestSelectedFact = successResults[successResults.length - 1]?.fact ?? null
      }

      if (failureResults.length > 0) {
        const failurePreview = failureResults.slice(0, 3).map((r) => buildBatchActionFailure(r, t)).join('；')
        const hasMoreFailure = failureResults.length > 3 ? ' 等' : ''
        setActionError(t('memory.batchPartialFailure', {
          action: actionLabel(action, t),
          preview: failurePreview,
          hasMore: hasMoreFailure,
        }))
      }

      if (latestSelectedFact) {
        setSelectedFact({ kind: 'candidate', fact: latestSelectedFact })
      }

      setSelectedCandidateKeys(Array.from(failedKeys))
      await Promise.all([candidateFactsQuery.refetch(), projectFactsQuery.refetch()])
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setActionError(t('memory.batchFailedDetail', { message: unknownError.message }))
      } else {
        setActionError(unknownError instanceof Error ? unknownError.message : t('memory.batchFailed'))
      }
    } finally {
      setBatchActionSubmitting(null)
    }
  }

  return (
    <AppShell
      title={t('memory.pageTitle')}
      description={t('memory.pageDescription')}
    >
      <div className="events-page">
        <form className="config-filters" aria-label={t('memory.filterLabel')} onSubmit={handleSubmit}>
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
            <button type="button" className="rollouts-action" onClick={handleResetFilters}>
              {t('memory.resetFilters')}
            </button>
          </div>
        </form>

        <div className="memory-governance__tabs" role="tablist" aria-label={t('memory.tabSwitchLabel')}>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'governance'}
            className={`memory-governance__tab ${activeTab === 'governance' ? 'memory-governance__tab--active' : ''}`}
            onClick={() => setActiveTab('governance')}
          >
            {t('memory.tabGovernance')}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'search'}
            className={`memory-governance__tab ${activeTab === 'search' ? 'memory-governance__tab--active' : ''}`}
            onClick={() => setActiveTab('search')}
          >
            {t('memory.tabSearch')}
          </button>
        </div>

        {activeTab === 'governance' ? (
        <>
        {candidateFactsQuery.isLoading || projectFactsQuery.isLoading ? <div className="event-state">{t('memory.loadingFacts')}</div> : null}
        {candidateFactsQuery.error ? <div className="config-error">{t('memory.candidateLoadError')}</div> : null}
        {projectFactsQuery.error ? <div className="config-error">{t('memory.projectLoadError')}</div> : null}
        {actionError ? <div className="config-error">{actionError}</div> : null}
        {actionSuccess ? (
          <div className="event-state memory-governance__feedback" role="status" aria-live="polite">
            <strong>{t('memory.recentAction')}</strong>
            <div>{actionSuccess}</div>
            <div>{t('memory.autoRefreshed')}</div>
          </div>
        ) : null}
        </>
        ) : null}

        {activeTab === 'search' ? (
        <div className="memory-governance__search-panel">
          <form className="config-filters" aria-label={t('memory.searchFormLabel')} onSubmit={handleSearch}>
            <label>
              {t('memory.searchContent')}
              <input
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder={t('memory.searchPlaceholder')}
              />
            </label>
            <label>
              {t('memory.tenantId')}
              <input
                value={searchTenantId}
                onChange={(event) => setSearchTenantId(event.target.value)}
                placeholder={t('memory.optionalTenantScope')}
              />
            </label>
            <label>
              {t('memory.userId')}
              <input
                value={searchUserId}
                onChange={(event) => setSearchUserId(event.target.value)}
                placeholder={t('memory.optionalUserScope')}
              />
            </label>
            <div className="config-filters__actions">
              <button type="submit" disabled={searchLoading}>
                {searchLoading ? t('memory.searching') : t('memory.searchAction')}
              </button>
              <button
                type="button"
                className="rollouts-action"
                onClick={() => {
                  setSearchQuery('')
                  setSearchTenantId('')
                  setSearchUserId('')
                  setSearchResults([])
                  setSearchError('')
                  setSearchSubmitted(false)
                }}
              >
                {t('memory.clear')}
              </button>
            </div>
          </form>

          {searchError ? <div className="config-error">{searchError}</div> : null}

          {searchLoading ? (
            <div className="event-state">{t('memory.hybridSearching')}</div>
          ) : null}

          {!searchLoading && searchSubmitted && searchResults.length === 0 && !searchError ? (
            <div className="event-state">{t('memory.noSearchResults')}</div>
          ) : null}

          {!searchLoading && searchResults.length > 0 ? (
            <section className="event-table" aria-label={t('memory.searchResultsLabel')}>
              <div className="memory-governance__search-summary">
                {t('memory.searchResultCount', { count: searchResults.length })}
              </div>
              <table>
                <thead>
                  <tr>
                    <th className="memory-governance__rank-cell">{t('memory.rank')}</th>
                    <th>{t('memory.content')}</th>
                    <th className="memory-governance__score-cell">{t('memory.score')}</th>
                    <th>{t('memory.source')}</th>
                    <th>{t('memory.factKey')}</th>
                    <th>{t('memory.tenant')}</th>
                    <th>{t('memory.user')}</th>
                  </tr>
                </thead>
                <tbody>
                  {searchResults.map((result, index) => (
                    <tr key={`${result.rank}-${result.source}-${index}`}>
                      <td className="memory-governance__rank-cell">
                        <span className="memory-governance__rank-badge">{result.rank}</span>
                      </td>
                      <td>
                        <div className="memory-fact-cell">
                          <strong>{result.content}</strong>
                        </div>
                      </td>
                      <td className="memory-governance__score-cell">
                        <span className="memory-governance__score-pill">{result.score.toFixed(4)}</span>
                      </td>
                      <td>{result.source}</td>
                      <td>{result.fact_key || '—'}</td>
                      <td>{result.tenant_id || '—'}</td>
                      <td>{result.user_id || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          ) : null}
        </div>
        ) : null}

        {activeTab === 'governance' && !candidateFactsQuery.isLoading && !projectFactsQuery.isLoading && !candidateFactsQuery.error && !projectFactsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('memory.candidateFacts')}</span>
                <strong>{metrics.totalCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.pending')}</span>
                <strong>{metrics.pendingCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.confirmed')}</span>
                <strong>{metrics.confirmedCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.promoted')}</span>
                <strong>{metrics.promotedCandidates}</strong>
                <small>{t('memory.rejectedCount', { count: metrics.rejectedCandidates })}</small>
              </section>
            </div>

            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('memory.projectFacts')}</span>
                <strong>{metrics.totalProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.activeProjectFacts')}</span>
                <strong>{metrics.activeProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.supersededFacts')}</span>
                <strong>{metrics.supersededProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.currentFilter')}</span>
                <strong>{candidateFilters.tenant_id || t('memory.allTenants')}</strong>
                <small>
                  {t('memory.filterSummary', {
                    user: candidateFilters.user_id || t('memory.allUsers'),
                    candidate: candidateFilters.status || t('memory.all'),
                    project: projectFilters.status || t('memory.all'),
                  })}
                </small>
              </section>
            </div>

            <div className="memory-governance__content">
              <div className="memory-governance__candidate-panel">
                <section className="event-state memory-governance__batch-toolbar" aria-label={t('memory.batchOperations')}>
                      <div>
                        <strong>{t('memory.batch')}</strong>
                        <div>
                          {t('memory.batchSummary', {
                            selected: selectedVisibleCandidateFacts.length,
                            total: pagedCandidateFacts.length,
                            filtered: filteredCandidateFacts.length,
                            fetched: candidateFacts.length,
                            confirmable: selectedCandidateMetrics.confirm,
                            rejectable: selectedCandidateMetrics.reject,
                            promotable: selectedCandidateMetrics.promote,
                          })}
                    </div>
                  </div>
                  <div className="policy-actions">
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() =>
                        openBatchConfirmation(
                          'confirm',
                          selectedVisibleCandidateFacts,
                          setActionError,
                          setActionSuccess,
                          setPendingBatchAction,
                          setPendingBatchFacts,
                          t,
                        )
                      }
                      disabled={batchActionSubmitting !== null || selectedCandidateMetrics.confirm === 0}
                    >
                      {batchActionSubmitting === 'confirm' ? t('memory.batchConfirming') : t('memory.batchConfirm')}
                    </button>
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() =>
                        openBatchConfirmation(
                          'reject',
                          selectedVisibleCandidateFacts,
                          setActionError,
                          setActionSuccess,
                          setPendingBatchAction,
                          setPendingBatchFacts,
                          t,
                        )
                      }
                      disabled={batchActionSubmitting !== null || selectedCandidateMetrics.reject === 0}
                    >
                      {batchActionSubmitting === 'reject' ? t('memory.batchRejecting') : t('memory.batchReject')}
                    </button>
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() =>
                        openBatchConfirmation(
                          'promote',
                          selectedVisibleCandidateFacts,
                          setActionError,
                          setActionSuccess,
                          setPendingBatchAction,
                          setPendingBatchFacts,
                          t,
                        )
                      }
                      disabled={batchActionSubmitting !== null || selectedCandidateMetrics.promote === 0}
                    >
                      {batchActionSubmitting === 'promote' ? t('memory.batchPromoting') : t('memory.batchPromote')}
                    </button>
                  </div>
                </section>

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
                          <option key={size} value={size}>
                            {size}
                          </option>
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
                        fetched: candidateFacts.length,
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
                            onChange={(event) => handleToggleSelectAllVisible(event.target.checked)}
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
                            onClick={() => {
                              setCandidateSortDirection((previous) => nextSortDirection(candidateSortField, previous, 'status'))
                              setCandidateSortField('status')
                            }}
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
                            onClick={() => {
                              setCandidateSortDirection((previous) => nextSortDirection(candidateSortField, previous, 'updated_at'))
                              setCandidateSortField('updated_at')
                            }}
                          >
                            {t('memory.updatedAt')} {sortIndicator(candidateSortField, candidateSortDirection, 'updated_at')}
                          </button>
                        </th>
                        <th>{t('memory.actions')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {pagedCandidateFacts.map((row) => {
                        const confirmKey = candidateActionKey(row, 'confirm')
                        const rejectKey = candidateActionKey(row, 'reject')
                        const promoteKey = candidateActionKey(row, 'promote')
                        const rowKey = candidateRowKey(row)

                        return (
                          <tr
                            key={`${row.fact_key}:${row.tenant_id}:${row.user_id}:${row.updated_at ?? row.id}`}
                            className={selectedFact?.kind === 'candidate' && selectedFact.fact.id === row.id ? 'memory-row memory-row--selected' : 'memory-row'}
                            onClick={() => setSelectedFact({ kind: 'candidate', fact: row })}
                          >
                            <td className="memory-governance__selection-cell">
                              <input
                                type="checkbox"
                                checked={selectedCandidateKeySet.has(rowKey)}
                                onChange={(event) => handleToggleCandidateSelection(row, event.target.checked)}
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
                                  onClick={(event) => {
                                    event.stopPropagation()
                                    void handleCandidateAction(row, 'confirm')
                                  }}
                                  disabled={!isActionAllowed(row.status, 'confirm') || Boolean(actionSubmitting[confirmKey]) || batchActionSubmitting !== null}
                                  title={isActionAllowed(row.status, 'confirm') ? '' : t('memory.onlyPendingConfirm')}
                                >
                                  {actionSubmitting[confirmKey] ? t('memory.confirming') : t('memory.confirm')}
                                </button>
                                <button
                                  type="button"
                                  className="rollouts-action"
                                  onClick={(event) => {
                                    event.stopPropagation()
                                    void handleCandidateAction(row, 'reject')
                                  }}
                                  disabled={!isActionAllowed(row.status, 'reject') || Boolean(actionSubmitting[rejectKey]) || batchActionSubmitting !== null}
                                  title={isActionAllowed(row.status, 'reject') ? '' : t('memory.onlyPendingConfirmedReject')}
                                >
                                  {actionSubmitting[rejectKey] ? t('memory.rejecting') : t('memory.reject')}
                                </button>
                                <button
                                  type="button"
                                  className="rollouts-action"
                                  onClick={(event) => {
                                    event.stopPropagation()
                                    void handleCandidateAction(row, 'promote')
                                  }}
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
                      {filteredCandidateFacts.length === 0 ? (
                        <tr>
                          <td colSpan={10}>
                            {candidateFacts.length === 0
                              ? t('memory.noCandidateFacts')
                              : t('memory.noLocalCandidateMatch')}
                          </td>
                        </tr>
                      ) : null}
                    </tbody>
                  </table>
                </section>

                <div className="memory-governance__pager" role="group" aria-label={t('memory.candidatePagination')}>
                  <span>
                    {t('memory.paginationSummary', {
                      page: candidatePage,
                      totalPages: candidatePageCount,
                      start: candidatePagination.start,
                      end: candidatePagination.end,
                      total: candidatePagination.total,
                    })}
                  </span>
                  <div className="policy-actions">
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setCandidatePage((value) => Math.max(1, value - 1))}
                      disabled={candidatePage === 1}
                    >
                      {t('memory.prevPage')}
                    </button>
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setCandidatePage((value) => Math.min(candidatePageCount, value + 1))}
                      disabled={candidatePage === candidatePageCount}
                    >
                      {t('memory.nextPage')}
                    </button>
                  </div>
                </div>
              </div>

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

              <div className="memory-governance__project-panel">
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
                          <option key={size} value={size}>
                            {size}
                          </option>
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
                        fetched: projectFacts.length,
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
                            onClick={() => {
                              setProjectSortDirection((previous) => nextSortDirection(projectSortField, previous, 'status'))
                              setProjectSortField('status')
                            }}
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
                            onClick={() => {
                              setProjectSortDirection((previous) => nextSortDirection(projectSortField, previous, 'updated_at'))
                              setProjectSortField('updated_at')
                            }}
                          >
                            {t('memory.updatedAt')} {sortIndicator(projectSortField, projectSortDirection, 'updated_at')}
                          </button>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {pagedProjectFacts.map((row) => (
                        <tr
                          key={`${row.fact_key}:${row.tenant_id}:${row.user_id}:${row.updated_at ?? row.id}`}
                          className={selectedFact?.kind === 'project' && selectedFact.fact.id === row.id ? 'memory-row memory-row--selected' : 'memory-row'}
                          onClick={() => setSelectedFact({ kind: 'project', fact: row })}
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
                      {filteredProjectFacts.length === 0 ? (
                        <tr>
                          <td colSpan={8}>
                            {projectFacts.length === 0
                              ? t('memory.noProjectFacts')
                              : t('memory.noLocalProjectMatch')}
                          </td>
                        </tr>
                      ) : null}
                    </tbody>
                  </table>
                </section>

                <div className="memory-governance__pager" role="group" aria-label={t('memory.projectPagination')}>
                  <span>
                    {t('memory.paginationSummary', {
                      page: projectPage,
                      totalPages: projectPageCount,
                      start: projectPagination.start,
                      end: projectPagination.end,
                      total: projectPagination.total,
                    })}
                  </span>
                  <div className="policy-actions">
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setProjectPage((value) => Math.max(1, value - 1))}
                      disabled={projectPage === 1}
                    >
                      {t('memory.prevPage')}
                    </button>
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setProjectPage((value) => Math.min(projectPageCount, value + 1))}
                      disabled={projectPage === projectPageCount}
                    >
                      {t('memory.nextPage')}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <div className="event-state memory-governance__hint">
              <strong>{t('memory.opsNote')}</strong>
              <div>{t('memory.candidateTableNote')}</div>
              <div>{t('memory.localSearchNote')}</div>
              <div>{t('memory.batchScopeNote')}</div>
            </div>

            {pendingBatchAction ? (
              <div className="dialog-backdrop" role="dialog" aria-modal="true" aria-label={t('memory.confirmBatchTitle', { action: pendingBatchLabel })}>
                <div className="dialog-card">
                  <div className="dialog-card__header">
                    <div>
                      <h2>{t('memory.confirmBatchTitle', { action: pendingBatchLabel })}</h2>
                      <p>{t('memory.confirmBatchDesc', { count: pendingBatchCount, action: pendingBatchLabel })}</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => {
                        setPendingBatchAction(null)
                        setPendingBatchFacts([])
                      }}
                    >
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
                    {pendingBatchFacts.length > 5 ? <div>{t('memory.remainingOmitted', { count: pendingBatchFacts.length - 5 })}</div> : null}
                  </div>
                  <div className="dialog-card__actions">
                    <button
                      type="button"
                      onClick={() => {
                        setPendingBatchAction(null)
                        setPendingBatchFacts([])
                      }}
                    >
                      {t('common.cancel')}
                    </button>
                    <button
                      type="button"
                      onClick={async () => {
                        const action = pendingBatchAction
                        const facts = pendingBatchFacts
                        setPendingBatchAction(null)
                        setPendingBatchFacts([])
                        if (action) {
                          await handleBatchCandidateAction(action, facts)
                        }
                      }}
                    >
                      {t('memory.confirmBatchAction', { action: pendingBatchLabel })}
                    </button>
                  </div>
                </div>
              </div>
            ) : null}
          </>
        ) : null}
      </div>
    </AppShell>
  )
}
