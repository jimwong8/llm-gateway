import type { FormEvent } from 'react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ApiError } from '../../lib/http'
import {
  listMemoryCandidateFacts,
  listMemoryProjectFacts,
  searchMemory,
  submitMemoryCandidateFactAction,
  submitMemoryCandidateFactBatchAction,
} from '../../lib/memory'
import type {
  MemoryCandidateFact,
  MemoryCandidateFactBatchActionResult,
  MemoryCandidateFactFilters,
  MemoryFactAction,
  MemoryFactFilters,
  MemoryProjectFact,
  MemoryProjectFactFilters,
  MemorySearchResult,
} from '../../types/memory'
import {
  actionLabel,
  buildActionPayload,
  buildBatchActionFailure,
  buildBatchActionSummary,
  buildPaginationSummary,
  candidateActionKey,
  candidateRowKey,
  compareCandidateSortField,
  compareProjectSortField,
  isActionAllowed,
  matchesLocalFactQuery,
  nextSortDirection,
  pageSizeOptions,
  sortIndicator,
} from './memoryUtils'
import type {
  CandidateSortField,
  PaginationSummary,
  SelectedFact,
  SortDirection,
} from './memoryUtils'

// ─── 常量 ────────────────────────────────────────────────────────────────────

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

// ─── Hook 返回类型 ───────────────────────────────────────────────────────────

export interface UseMemoryGovernanceReturn {
  activeTab: 'governance' | 'search'
  setActiveTab: (tab: 'governance' | 'search') => void
  draftFilters: MemoryFactFilters
  setDraftFilters: React.Dispatch<React.SetStateAction<MemoryFactFilters>>
  draftCandidateStatus: string
  setDraftCandidateStatus: (s: string) => void
  draftProjectStatus: string
  setDraftProjectStatus: (s: string) => void
  candidateFilters: MemoryCandidateFactFilters
  projectFilters: MemoryProjectFactFilters
  actionSubmitting: Record<string, boolean>
  actionError: string
  actionSuccess: string
  setActionError: (v: string) => void
  setActionSuccess: (v: string) => void
  selectedFact: SelectedFact | null
  setSelectedFact: (fact: SelectedFact | null) => void
  selectedCandidateKeys: string[]
  setSelectedCandidateKeys: React.Dispatch<React.SetStateAction<string[]>>
  batchActionSubmitting: MemoryFactAction | null
  pendingBatchAction: MemoryFactAction | null
  setPendingBatchAction: (a: MemoryFactAction | null) => void
  pendingBatchFacts: MemoryCandidateFact[]
  setPendingBatchFacts: (f: MemoryCandidateFact[]) => void
  candidateSortField: CandidateSortField
  setCandidateSortField: (f: CandidateSortField) => void
  candidateSortDirection: SortDirection
  setCandidateSortDirection: React.Dispatch<React.SetStateAction<SortDirection>>
  projectSortField: CandidateSortField
  setProjectSortField: (f: CandidateSortField) => void
  projectSortDirection: SortDirection
  setProjectSortDirection: React.Dispatch<React.SetStateAction<SortDirection>>
  candidateLocalQuery: string
  setCandidateLocalQuery: (q: string) => void
  projectLocalQuery: string
  setProjectLocalQuery: (q: string) => void
  candidatePageSize: number
  setCandidatePageSize: (s: number) => void
  projectPageSize: number
  setProjectPageSize: (s: number) => void
  candidatePage: number
  setCandidatePage: React.Dispatch<React.SetStateAction<number>>
  projectPage: number
  setProjectPage: React.Dispatch<React.SetStateAction<number>>
  selectAllVisibleRef: React.RefObject<HTMLInputElement>
  searchQuery: string
  setSearchQuery: (q: string) => void
  searchTenantId: string
  setSearchTenantId: (v: string) => void
  searchUserId: string
  setSearchUserId: (v: string) => void
  searchResults: MemorySearchResult[]
  setSearchResults: React.Dispatch<React.SetStateAction<MemorySearchResult[]>>
  searchLoading: boolean
  searchError: string
  setSearchError: (v: string) => void
  searchSubmitted: boolean
  setSearchSubmitted: (v: boolean) => void
  candidateFactsQuery: ReturnType<typeof useQuery>
  projectFactsQuery: ReturnType<typeof useQuery>
  candidateFacts: MemoryCandidateFact[]
  projectFacts: MemoryProjectFact[]
  sortedCandidateFacts: MemoryCandidateFact[]
  filteredCandidateFacts: MemoryCandidateFact[]
  pagedCandidateFacts: MemoryCandidateFact[]
  candidatePageCount: number
  sortedProjectFacts: MemoryProjectFact[]
  filteredProjectFacts: MemoryProjectFact[]
  pagedProjectFacts: MemoryProjectFact[]
  projectPageCount: number
  visibleCandidateKeys: string[]
  visibleCandidateKeySet: Set<string>
  selectedCandidateKeySet: Set<string>
  selectedVisibleCandidateFacts: MemoryCandidateFact[]
  selectedCandidateMetrics: { confirm: number; reject: number; promote: number }
  allVisibleSelected: boolean
  someVisibleSelected: boolean
  pendingBatchCount: number
  pendingBatchLabel: string
  metrics: {
    totalCandidates: number
    pendingCandidates: number
    confirmedCandidates: number
    promotedCandidates: number
    rejectedCandidates: number
    totalProjectFacts: number
    activeProjectFacts: number
    supersededProjectFacts: number
  }
  candidatePagination: PaginationSummary
  projectPagination: PaginationSummary
  handleSubmit: (event: FormEvent<HTMLFormElement>) => void
  handleResetFilters: () => void
  handleSearch: (event: FormEvent<HTMLFormElement>) => Promise<void>
  handleToggleCandidateSelection: (row: MemoryCandidateFact, checked: boolean) => void
  handleToggleSelectAllVisible: (checked: boolean) => void
  handleCandidateAction: (row: MemoryCandidateFact, action: MemoryFactAction) => Promise<void>
  handleBatchCandidateAction: (action: MemoryFactAction, actionableRows?: MemoryCandidateFact[]) => Promise<void>
  openBatchConfirmation: (action: MemoryFactAction, facts: MemoryCandidateFact[]) => void
}

// ─── Hook ────────────────────────────────────────────────────────────────────

export function useMemoryGovernance(
  t: (key: string, options?: Record<string, unknown>) => string,
): UseMemoryGovernanceReturn {
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
  const selectAllVisibleRef = useRef<HTMLInputElement>(null)

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
          [row.fact_key, row.fact_value, row.source_text, row.tenant_id, row.user_id, row.status, row.source_message_seq, row.superseded_by],
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
      totalCandidates, pendingCandidates, confirmedCandidates, promotedCandidates, rejectedCandidates,
      totalProjectFacts, activeProjectFacts, supersededProjectFacts,
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

  // Effects
  useEffect(() => {
    setSelectedCandidateKeys((previous) => previous.filter((key) => visibleCandidateKeySet.has(key)))
  }, [visibleCandidateKeySet])

  useEffect(() => { setCandidatePage(1) }, [candidateLocalQuery, candidatePageSize, candidateSortField, candidateSortDirection, candidateFilters])
  useEffect(() => { setProjectPage(1) }, [projectLocalQuery, projectPageSize, projectSortField, projectSortDirection, projectFilters])

  useEffect(() => {
    if (candidatePage > candidatePageCount) setCandidatePage(candidatePageCount)
  }, [candidatePage, candidatePageCount])

  useEffect(() => {
    if (projectPage > projectPageCount) setProjectPage(projectPageCount)
  }, [projectPage, projectPageCount])

  useEffect(() => {
    if (selectAllVisibleRef.current) {
      selectAllVisibleRef.current.indeterminate = !allVisibleSelected && someVisibleSelected
    }
  }, [allVisibleSelected, someVisibleSelected])

  // Handlers
  const handleSubmit = useCallback((event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const baseFilters = {
      tenant_id: draftFilters.tenant_id.trim(),
      user_id: draftFilters.user_id.trim(),
    }
    setCandidateFilters({ ...baseFilters, status: draftCandidateStatus.trim() })
    setProjectFilters({ ...baseFilters, status: draftProjectStatus.trim() })
    setActionError('')
    setActionSuccess('')
  }, [draftFilters, draftCandidateStatus, draftProjectStatus])

  const handleResetFilters = useCallback(() => {
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
  }, [])

  const handleSearch = useCallback(async (event: FormEvent<HTMLFormElement>) => {
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
  }, [searchQuery, searchTenantId, searchUserId, t])

  const handleToggleCandidateSelection = useCallback((row: MemoryCandidateFact, checked: boolean) => {
    const key = candidateRowKey(row)
    setSelectedCandidateKeys((previous) => {
      if (checked) return previous.includes(key) ? previous : [...previous, key]
      return previous.filter((value) => value !== key)
    })
  }, [])

  const handleToggleSelectAllVisible = useCallback((checked: boolean) => {
    setSelectedCandidateKeys((previous) => {
      if (checked) return Array.from(new Set([...previous, ...visibleCandidateKeys]))
      return previous.filter((key) => !visibleCandidateKeySet.has(key))
    })
  }, [visibleCandidateKeys, visibleCandidateKeySet])

  const handleCandidateAction = useCallback(async (row: MemoryCandidateFact, action: MemoryFactAction) => {
    const actKey = candidateActionKey(row, action)
    setActionSubmitting((previous) => ({ ...previous, [actKey]: true }))
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
      setActionSubmitting((previous) => ({ ...previous, [actKey]: false }))
    }
  }, [candidateFilters, candidateFactsQuery, projectFactsQuery, t])

  const handleBatchCandidateAction = useCallback(async (
    action: MemoryFactAction,
    actionableRows: MemoryCandidateFact[] = selectedVisibleCandidateFacts.filter((row) => isActionAllowed(row.status, action)),
  ) => {
    if (batchActionSubmitting) return
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
        if (matchedFailure) failedKeys.add(candidateRowKey(row))
      }
      if (successResults.length > 0) {
        const successPreview = successResults.slice(0, 3).map(buildBatchActionSummary).join('；')
        const hasMoreSuccess = successResults.length > 3 ? ' 等' : ''
        const failurePart = failureResults.length > 0 ? `，失败 ${failureResults.length} 条` : ''
        setActionSuccess(
          t('memory.batchSuccess', {
            action: actionLabel(action, t), successCount: successResults.length,
            failureCount: failureResults.length, preview: successPreview, failurePart, hasMore: hasMoreSuccess,
          }),
        )
        latestSelectedFact = successResults[successResults.length - 1]?.fact ?? null
      }
      if (failureResults.length > 0) {
        const failurePreview = failureResults.slice(0, 3).map((r) => buildBatchActionFailure(r, t)).join('；')
        const hasMoreFailure = failureResults.length > 3 ? ' 等' : ''
        setActionError(t('memory.batchPartialFailure', {
          action: actionLabel(action, t), preview: failurePreview, hasMore: hasMoreFailure,
        }))
      }
      if (latestSelectedFact) setSelectedFact({ kind: 'candidate', fact: latestSelectedFact })
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
  }, [batchActionSubmitting, selectedVisibleCandidateFacts, candidateFilters, candidateFactsQuery, projectFactsQuery, t])

  const openBatchConfirmation = useCallback((action: MemoryFactAction, facts: MemoryCandidateFact[]) => {
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
  }, [t])

  return {
    activeTab, setActiveTab,
    draftFilters, setDraftFilters,
    draftCandidateStatus, setDraftCandidateStatus,
    draftProjectStatus, setDraftProjectStatus,
    candidateFilters, projectFilters,
    actionSubmitting, actionError, actionSuccess, setActionError, setActionSuccess,
    selectedFact, setSelectedFact,
    selectedCandidateKeys, setSelectedCandidateKeys,
    batchActionSubmitting,
    pendingBatchAction, setPendingBatchAction,
    pendingBatchFacts, setPendingBatchFacts,
    candidateSortField, setCandidateSortField,
    candidateSortDirection, setCandidateSortDirection,
    projectSortField, setProjectSortField,
    projectSortDirection, setProjectSortDirection,
    candidateLocalQuery, setCandidateLocalQuery,
    projectLocalQuery, setProjectLocalQuery,
    candidatePageSize, setCandidatePageSize,
    projectPageSize, setProjectPageSize,
    candidatePage, setCandidatePage,
    projectPage, setProjectPage,
    selectAllVisibleRef,
    searchQuery, setSearchQuery,
    searchTenantId, setSearchTenantId,
    searchUserId, setSearchUserId,
    searchResults, setSearchResults, searchLoading, searchError, setSearchError, searchSubmitted, setSearchSubmitted,
    candidateFactsQuery, projectFactsQuery,
    candidateFacts, projectFacts,
    sortedCandidateFacts, filteredCandidateFacts, pagedCandidateFacts, candidatePageCount,
    sortedProjectFacts, filteredProjectFacts, pagedProjectFacts, projectPageCount,
    visibleCandidateKeys, visibleCandidateKeySet, selectedCandidateKeySet,
    selectedVisibleCandidateFacts, selectedCandidateMetrics,
    allVisibleSelected, someVisibleSelected,
    pendingBatchCount, pendingBatchLabel,
    metrics, candidatePagination, projectPagination,
    handleSubmit, handleResetFilters, handleSearch,
    handleToggleCandidateSelection, handleToggleSelectAllVisible,
    handleCandidateAction, handleBatchCandidateAction, openBatchConfirmation,
  }
}
