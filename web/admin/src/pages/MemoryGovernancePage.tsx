import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { ApiError } from '../lib/http'
import {
  listMemoryCandidateFacts,
  listMemoryProjectFacts,
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

function actionLabel(action: MemoryFactAction) {
  switch (action) {
    case 'confirm':
      return '确认'
    case 'reject':
      return '拒绝'
    case 'promote':
      return '提升'
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

function buildBatchActionFailure(result: MemoryCandidateFactBatchActionResult) {
  const message = result.error?.message?.trim() || '操作失败'
  return `${result.fact_key}：${message}`
}

function openBatchConfirmation(
  action: MemoryFactAction,
  facts: MemoryCandidateFact[],
  setActionError: (value: string) => void,
  setActionSuccess: (value: string) => void,
  setPendingBatchAction: (value: MemoryFactAction | null) => void,
  setPendingBatchFacts: (value: MemoryCandidateFact[]) => void,
) {
  if (facts.length === 0) {
    setActionError('请先在当前可见候选事实中选择至少一条记录。')
    setActionSuccess('')
    return
  }

  const actionableRows = facts.filter((row) => isActionAllowed(row.status, action))
  if (actionableRows.length === 0) {
    setActionError(`当前可见选中条目中没有可${actionLabel(action)}的候选事实。`)
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
  const pendingBatchLabel = pendingBatchAction ? actionLabel(pendingBatchAction) : ''

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
      setActionSuccess(`已${actionLabel(action)}事实：${response.fact_key}（当前状态：${response.status}）`)
      setSelectedFact({ kind: 'candidate', fact: response })
      await Promise.all([candidateFactsQuery.refetch(), projectFactsQuery.refetch()])
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setActionError(`操作失败：${unknownError.message}`)
      } else {
        setActionError(unknownError instanceof Error ? unknownError.message : '操作失败')
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
      setActionError('请先在当前可见候选事实中选择至少一条记录。')
      setActionSuccess('')
      return
    }

    if (actionableRows.length === 0) {
      setActionError(`当前可见选中条目中没有可${actionLabel(action)}的候选事实。`)
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
        setActionSuccess(
          `批量${actionLabel(action)}完成：成功 ${successResults.length} 条${failureResults.length > 0 ? `，失败 ${failureResults.length} 条` : ''}。${successPreview}${successResults.length > 3 ? ' 等' : ''}`,
        )
        latestSelectedFact = successResults[successResults.length - 1]?.fact ?? null
      }

      if (failureResults.length > 0) {
        const failurePreview = failureResults.slice(0, 3).map(buildBatchActionFailure).join('；')
        setActionError(`批量${actionLabel(action)}存在失败：${failurePreview}${failureResults.length > 3 ? ' 等' : ''}`)
      }

      if (latestSelectedFact) {
        setSelectedFact({ kind: 'candidate', fact: latestSelectedFact })
      }

      setSelectedCandidateKeys(Array.from(failedKeys))
      await Promise.all([candidateFactsQuery.refetch(), projectFactsQuery.refetch()])
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setActionError(`批量操作失败：${unknownError.message}`)
      } else {
        setActionError(unknownError instanceof Error ? unknownError.message : '批量操作失败')
      }
    } finally {
      setBatchActionSubmitting(null)
    }
  }

  return (
    <AppShell
      title="记忆治理"
      description="查看候选事实与项目事实，按租户/用户/状态过滤，并直接确认、拒绝或提升候选事实。"
    >
      <div className="events-page">
        <form className="config-filters" aria-label="记忆治理筛选" onSubmit={handleSubmit}>
          <label>
            租户 ID
            <input
              value={draftFilters.tenant_id}
              onChange={(event) => setDraftFilters((previous) => ({ ...previous, tenant_id: event.target.value }))}
              placeholder="tenant-a"
            />
          </label>
          <label>
            用户 ID
            <input
              value={draftFilters.user_id}
              onChange={(event) => setDraftFilters((previous) => ({ ...previous, user_id: event.target.value }))}
              placeholder="user-1"
            />
          </label>
          <label>
            候选状态
            <select value={draftCandidateStatus} onChange={(event) => setDraftCandidateStatus(event.target.value)}>
              {candidateStatuses.map((status) => (
                <option key={status || 'all'} value={status}>
                  {status || 'all'}
                </option>
              ))}
            </select>
          </label>
          <label>
            项目状态
            <select value={draftProjectStatus} onChange={(event) => setDraftProjectStatus(event.target.value)}>
              {projectStatuses.map((status) => (
                <option key={status || 'all'} value={status}>
                  {status || 'all'}
                </option>
              ))}
            </select>
          </label>
          <div className="config-filters__actions">
            <button type="submit">刷新记忆事实</button>
            <button type="button" className="rollouts-action" onClick={handleResetFilters}>
              重置筛选
            </button>
          </div>
        </form>

        {candidateFactsQuery.isLoading || projectFactsQuery.isLoading ? <div className="event-state">正在加载记忆事实…</div> : null}
        {candidateFactsQuery.error ? <div className="config-error">候选事实加载失败，请检查记忆管理接口状态。</div> : null}
        {projectFactsQuery.error ? <div className="config-error">项目事实加载失败，请检查记忆管理接口状态。</div> : null}
        {actionError ? <div className="config-error">{actionError}</div> : null}
        {actionSuccess ? (
          <div className="event-state memory-governance__feedback" role="status" aria-live="polite">
            <strong>最近操作</strong>
            <div>{actionSuccess}</div>
            <div>候选事实与项目事实列表已自动刷新，可直接继续审阅下一条。</div>
          </div>
        ) : null}

        {!candidateFactsQuery.isLoading && !projectFactsQuery.isLoading && !candidateFactsQuery.error && !projectFactsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>候选事实</span>
                <strong>{metrics.totalCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>待处理</span>
                <strong>{metrics.pendingCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>已确认</span>
                <strong>{metrics.confirmedCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>已提升</span>
                <strong>{metrics.promotedCandidates}</strong>
                <small>已拒绝 {metrics.rejectedCandidates}</small>
              </section>
            </div>

            <div className="summary-card-grid">
              <section className="summary-card">
                <span>项目事实</span>
                <strong>{metrics.totalProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>活跃项目事实</span>
                <strong>{metrics.activeProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>已取代的事实</span>
                <strong>{metrics.supersededProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>当前筛选</span>
                <strong>{candidateFilters.tenant_id || '全部租户'}</strong>
                <small>
                  用户 {candidateFilters.user_id || '全部用户'} · 候选 {candidateFilters.status || '全部'} · 项目 {projectFilters.status || '全部'}
                </small>
              </section>
            </div>

            <div className="memory-governance__content">
              <div className="memory-governance__candidate-panel">
                <section className="event-state memory-governance__batch-toolbar" aria-label="候选事实批量操作">
                      <div>
                        <strong>批量操作</strong>
                        <div>
                          已选当前可见 {selectedVisibleCandidateFacts.length} / {pagedCandidateFacts.length} · 本地筛选 {filteredCandidateFacts.length} · 已拉取 {candidateFacts.length} · 可确认 {selectedCandidateMetrics.confirm} · 可拒绝 {selectedCandidateMetrics.reject} · 可提升 {selectedCandidateMetrics.promote}
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
                        )
                      }
                      disabled={batchActionSubmitting !== null || selectedCandidateMetrics.confirm === 0}
                    >
                      {batchActionSubmitting === 'confirm' ? '批量确认中…' : '批量确认'}
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
                        )
                      }
                      disabled={batchActionSubmitting !== null || selectedCandidateMetrics.reject === 0}
                    >
                      {batchActionSubmitting === 'reject' ? '批量拒绝中…' : '批量拒绝'}
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
                        )
                      }
                      disabled={batchActionSubmitting !== null || selectedCandidateMetrics.promote === 0}
                    >
                      {batchActionSubmitting === 'promote' ? '批量提升中…' : '批量提升'}
                    </button>
                  </div>
                </section>

                <section className="event-state memory-governance__table-toolbar" aria-label="候选事实本地控制">
                  <div className="memory-governance__table-toolbar-fields">
                    <label>
                      本地搜索
                      <input
                        value={candidateLocalQuery}
                        onChange={(event) => setCandidateLocalQuery(event.target.value)}
                        placeholder="搜索 key / value / source / tenant / user / status"
                      />
                    </label>
                    <label>
                       每页条数
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
                    <strong>当前可见范围</strong>
                    <span>
                      显示第 {candidatePagination.start}-{candidatePagination.end} 条，共 {candidatePagination.total} 条本地筛选结果（后端已拉取 {candidateFacts.length} 条）。
                    </span>
                  </div>
                </section>

                <section className="event-table" aria-label="候选事实表">
                  <table>
                    <thead>
                      <tr>
                        <th className="memory-governance__selection-cell">
                          <input
                            ref={selectAllVisibleRef}
                            type="checkbox"
                            checked={allVisibleSelected}
                            onChange={(event) => handleToggleSelectAllVisible(event.target.checked)}
                            aria-label="选择当前可见候选事实"
                          />
                        </th>
                        <th>事实键</th>
                        <th>值</th>
                        <th>租户</th>
                        <th>用户</th>
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
                            状态 {sortIndicator(candidateSortField, candidateSortDirection, 'status')}
                          </button>
                        </th>
                        <th>确认次数</th>
                        <th>来源序号</th>
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
                            更新时间 {sortIndicator(candidateSortField, candidateSortDirection, 'updated_at')}
                          </button>
                        </th>
                        <th>操作</th>
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
                                aria-label={`选择候选事实 ${row.fact_key}`}
                              />
                            </td>
                            <td>{row.fact_key}</td>
                            <td>
                              <div className="memory-fact-cell">
                                <strong>{row.fact_value}</strong>
                                <small>{row.source_text || '无来源摘录'}</small>
                              </div>
                            </td>
                            <td>{row.tenant_id || '—'}</td>
                            <td>{row.user_id}</td>
                            <td>
                              <div className="memory-fact-cell">
                                <span className={`status-pill ${row.status}`}>{row.status}</span>
                                <small>最近更新：{formatDate(row.updated_at)}</small>
                              </div>
                            </td>
                            <td>
                              <div className="memory-fact-cell memory-fact-cell--compact">
                                <strong>{row.confirmation_count}</strong>
                                <small>来源序号：{row.source_message_seq}</small>
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
                                  title={isActionAllowed(row.status, 'confirm') ? '' : '仅 pending 状态支持确认'}
                                >
                                  {actionSubmitting[confirmKey] ? '确认中…' : '确认'}
                                </button>
                                <button
                                  type="button"
                                  className="rollouts-action"
                                  onClick={(event) => {
                                    event.stopPropagation()
                                    void handleCandidateAction(row, 'reject')
                                  }}
                                  disabled={!isActionAllowed(row.status, 'reject') || Boolean(actionSubmitting[rejectKey]) || batchActionSubmitting !== null}
                                  title={isActionAllowed(row.status, 'reject') ? '' : '仅 pending / confirmed 状态支持拒绝'}
                                >
                                  {actionSubmitting[rejectKey] ? '拒绝中…' : '拒绝'}
                                </button>
                                <button
                                  type="button"
                                  className="rollouts-action"
                                  onClick={(event) => {
                                    event.stopPropagation()
                                    void handleCandidateAction(row, 'promote')
                                  }}
                                  disabled={!isActionAllowed(row.status, 'promote') || Boolean(actionSubmitting[promoteKey]) || batchActionSubmitting !== null}
                                  title={isActionAllowed(row.status, 'promote') ? '' : '仅 confirmed 状态支持提升'}
                                >
                                  {actionSubmitting[promoteKey] ? '提升中…' : '提升'}
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
                              ? '当前后端筛选结果下暂无 candidate facts。'
                              : '没有匹配当前本地搜索的 candidate facts，请调整关键字或重置本地搜索。'}
                          </td>
                        </tr>
                      ) : null}
                    </tbody>
                  </table>
                </section>

                <div className="memory-governance__pager" role="group" aria-label="候选事实分页">
                  <span>
                    第 {candidatePage} / {candidatePageCount} 页 · 显示第 {candidatePagination.start}-{candidatePagination.end} 条，共 {candidatePagination.total} 条
                  </span>
                  <div className="policy-actions">
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setCandidatePage((value) => Math.max(1, value - 1))}
                      disabled={candidatePage === 1}
                    >
                      上一页
                    </button>
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setCandidatePage((value) => Math.min(candidatePageCount, value + 1))}
                      disabled={candidatePage === candidatePageCount}
                    >
                      下一页
                    </button>
                  </div>
                </div>
              </div>

              <section className="memory-governance__detail event-state" aria-label="Selected Memory Fact Details">
                <strong>{selectedFact ? '事实详情' : '选择一条事实查看详情'}</strong>
                {selectedFact ? (
                  <div className="memory-governance__detail-grid">
                    <div>
                      <span>类型</span>
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
                      <pre>{selectedFact.fact.source_text || '无来源摘录'}</pre>
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
                              .map((action) => actionLabel(action))
                              .join(' / ') || '只读'}
                          </strong>
                        </div>
                      </>
                    ) : (
                      <>
                        <div>
                          <span>Superseded By</span>
                          <strong>{selectedFact.fact.superseded_by ? `#${selectedFact.fact.superseded_by}` : '当前生效'}</strong>
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
                  <div>点击候选事实或项目事实表中的任意一行，可集中查看完整来源摘录、状态和可执行动作说明。</div>
                )}
              </section>

              <div className="memory-governance__project-panel">
                <section className="event-state memory-governance__table-toolbar" aria-label="项目事实本地控制">
                  <div className="memory-governance__table-toolbar-fields">
                    <label>
                      本地搜索
                      <input
                        value={projectLocalQuery}
                        onChange={(event) => setProjectLocalQuery(event.target.value)}
                        placeholder="搜索 key / value / source / tenant / user / status"
                      />
                    </label>
                    <label>
                       每页条数
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
                    <strong>当前可见范围</strong>
                    <span>
                      显示第 {projectPagination.start}-{projectPagination.end} 条，共 {projectPagination.total} 条本地筛选结果（后端已拉取 {projectFacts.length} 条）。
                    </span>
                  </div>
                </section>

                <section className="event-table" aria-label="项目事实表">
                  <table>
                    <thead>
                      <tr>
                        <th>事实键</th>
                        <th>值</th>
                        <th>租户</th>
                        <th>用户</th>
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
                            状态 {sortIndicator(projectSortField, projectSortDirection, 'status')}
                          </button>
                        </th>
                        <th>来源序号</th>
                        <th>最后验证</th>
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
                            更新时间 {sortIndicator(projectSortField, projectSortDirection, 'updated_at')}
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
                              <small>{row.source_text || '无来源摘录'}</small>
                            </div>
                          </td>
                          <td>{row.tenant_id || '—'}</td>
                          <td>{row.user_id}</td>
                          <td>
                            <div className="memory-fact-cell">
                              <span className={`status-pill ${row.status}`}>{row.status}</span>
                              <small>{row.superseded_by ? `由事实 #${row.superseded_by} 取代` : '当前生效'}</small>
                            </div>
                          </td>
                          <td>
                            <div className="memory-fact-cell memory-fact-cell--compact">
                              <strong>{row.source_message_seq}</strong>
                              <small>验证：{formatDate(row.last_verified_at)}</small>
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
                              ? '当前后端筛选结果下暂无 project facts。'
                              : '没有匹配当前本地搜索的 project facts，请调整关键字或重置本地搜索。'}
                          </td>
                        </tr>
                      ) : null}
                    </tbody>
                  </table>
                </section>

                <div className="memory-governance__pager" role="group" aria-label="项目事实分页">
                  <span>
                    第 {projectPage} / {projectPageCount} 页 · 显示第 {projectPagination.start}-{projectPagination.end} 条，共 {projectPagination.total} 条
                  </span>
                  <div className="policy-actions">
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setProjectPage((value) => Math.max(1, value - 1))}
                      disabled={projectPage === 1}
                    >
                      上一页
                    </button>
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setProjectPage((value) => Math.min(projectPageCount, value + 1))}
                      disabled={projectPage === projectPageCount}
                    >
                      下一页
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <div className="event-state memory-governance__hint">
              <strong>运维备注</strong>
              <div>候选事实表强调“值 + 来源摘录 + 状态/确认数”，便于快速决定确认、拒绝或提升。</div>
              <div>本地搜索与分页只作用于前端已拉取的数据，不会改变后端筛选条件，也不会触发额外接口变更。</div>
              <div>批量选择与批量操作严格限定在当前可见候选事实页，翻页或调整本地搜索后会自动收敛到新的可见范围。</div>
            </div>

            {pendingBatchAction ? (
              <div className="dialog-backdrop" role="dialog" aria-modal="true" aria-label="Batch Action Confirmation">
                <div className="dialog-card">
                  <div className="dialog-card__header">
                    <div>
                      <h2>确认批量{pendingBatchLabel}</h2>
                      <p>即将对 {pendingBatchCount} 条当前可见候选事实执行“{pendingBatchLabel}”。操作会通过批量接口一次性提交，请确认后继续。</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => {
                        setPendingBatchAction(null)
                        setPendingBatchFacts([])
                      }}
                    >
                      关闭
                    </button>
                  </div>
                  <div className="memory-governance__confirm-list">
                    {pendingBatchFacts.slice(0, 5).map((row) => (
                      <div key={candidateRowKey(row)}>
                        <strong>{row.fact_key}</strong>
                        <small>{row.fact_value} · {row.status}</small>
                      </div>
                    ))}
                    {pendingBatchFacts.length > 5 ? <div>…其余 {pendingBatchFacts.length - 5} 条已省略</div> : null}
                  </div>
                  <div className="dialog-card__actions">
                    <button
                      type="button"
                      onClick={() => {
                        setPendingBatchAction(null)
                        setPendingBatchFacts([])
                      }}
                    >
                      取消
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
                      确认批量{pendingBatchLabel}
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
