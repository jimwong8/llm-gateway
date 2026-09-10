import type {
  MemoryCandidateFact,
  MemoryCandidateFactActionRequest,
  MemoryCandidateFactBatchActionResult,
  MemoryFactAction,
  MemoryFactFilters,
  MemoryProjectFact,
} from '../../types/memory'

export type CandidateSortField = 'status' | 'updated_at'
export type SortDirection = 'asc' | 'desc'
export type SelectedFact =
  | { kind: 'candidate'; fact: MemoryCandidateFact }
  | { kind: 'project'; fact: MemoryProjectFact }

export const pageSizeOptions = [10, 25, 50]

export type PaginationSummary = {
  start: number
  end: number
  total: number
}

export function buildActionPayload(filters: MemoryFactFilters, row: MemoryCandidateFact): MemoryCandidateFactActionRequest {
  return {
    tenant_id: filters.tenant_id.trim() || row.tenant_id,
    user_id: filters.user_id.trim() || row.user_id,
  }
}

export function actionLabel(action: MemoryFactAction, t?: (key: string) => string) {
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

export function isActionAllowed(status: string, action: MemoryFactAction) {
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
  if (!value) return 0
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 0 : date.getTime()
}

function candidateSortPriority(status: string) {
  switch (status.trim().toLowerCase()) {
    case 'pending': return 0
    case 'confirmed': return 1
    case 'promoted': return 2
    case 'rejected': return 3
    default: return 99
  }
}

function projectSortPriority(status: string) {
  switch (status.trim().toLowerCase()) {
    case 'active': return 0
    case 'superseded': return 1
    default: return 99
  }
}

export function compareCandidateSortField(
  a: MemoryCandidateFact,
  b: MemoryCandidateFact,
  field: CandidateSortField,
  direction: SortDirection,
) {
  let compareValue = 0
  if (field === 'status') {
    compareValue = candidateSortPriority(a.status) - candidateSortPriority(b.status)
    if (compareValue === 0) compareValue = a.status.localeCompare(b.status)
  } else {
    compareValue = parseSortTime(a.updated_at) - parseSortTime(b.updated_at)
  }
  if (compareValue === 0) compareValue = a.fact_key.localeCompare(b.fact_key)
  return direction === 'asc' ? compareValue : -compareValue
}

export function compareProjectSortField(
  a: MemoryProjectFact,
  b: MemoryProjectFact,
  field: CandidateSortField,
  direction: SortDirection,
) {
  let compareValue = 0
  if (field === 'status') {
    compareValue = projectSortPriority(a.status) - projectSortPriority(b.status)
    if (compareValue === 0) compareValue = a.status.localeCompare(b.status)
  } else {
    compareValue = parseSortTime(a.updated_at) - parseSortTime(b.updated_at)
  }
  if (compareValue === 0) compareValue = a.fact_key.localeCompare(b.fact_key)
  return direction === 'asc' ? compareValue : -compareValue
}

export function nextSortDirection(
  currentField: CandidateSortField,
  currentDirection: SortDirection,
  nextField: CandidateSortField,
): SortDirection {
  if (currentField !== nextField) return 'desc'
  return currentDirection === 'desc' ? 'asc' : 'desc'
}

export function sortIndicator(activeField: CandidateSortField, activeDirection: SortDirection, field: CandidateSortField) {
  if (activeField !== field) return '↕'
  return activeDirection === 'desc' ? '↓' : '↑'
}

export function matchesLocalFactQuery(fields: Array<string | number | undefined>, query: string) {
  const normalizedQuery = query.trim().toLowerCase()
  if (!normalizedQuery) return true
  return fields.some((value) => String(value ?? '').toLowerCase().includes(normalizedQuery))
}

export function candidateRowKey(row: MemoryCandidateFact) {
  return `${row.id}:${row.fact_key}:${row.tenant_id}:${row.user_id}`
}

export function candidateActionKey(row: MemoryCandidateFact, action: MemoryFactAction) {
  return `${action}:${row.fact_key}:${row.tenant_id}:${row.user_id}`
}

export function buildBatchActionSummary(result: MemoryCandidateFactBatchActionResult) {
  const status = result.status ?? result.fact?.status ?? 'unknown'
  return `${result.fact_key}→${status}`
}

export function buildBatchActionFailure(result: MemoryCandidateFactBatchActionResult, t: (key: string) => string) {
  const message = result.error?.message?.trim() || t('memory.actionFailed')
  return `${result.fact_key}：${message}`
}

export function buildPaginationSummary(total: number, page: number, pageSize: number): PaginationSummary {
  if (total === 0) return { start: 0, end: 0, total }
  const start = (page - 1) * pageSize + 1
  const end = Math.min(page * pageSize, total)
  return { start, end, total }
}
