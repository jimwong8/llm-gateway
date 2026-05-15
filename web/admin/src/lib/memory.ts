import type {
  MemoryCandidateFact,
  MemoryCandidateFactActionRequest,
  MemoryCandidateFactBatchActionRequest,
  MemoryCandidateFactBatchActionResponse,
  MemoryCandidateFactBatchActionResponseWire,
  MemoryCandidateFactBatchActionResult,
  MemoryCandidateFactBatchActionResultWire,
  MemoryCandidateFactWire,
  MemoryCandidateFactFilters,
  MemoryFactAction,
  MemoryFactFilters,
  MemoryListResponse,
  MemoryProjectFact,
  MemoryProjectFactFilters,
  MemoryProjectFactWire,
} from '../types/memory'
import { apiRequest, jsonRequest } from './http'

function buildMemoryQuery(filters: MemoryFactFilters, status?: string) {
  const params = new URLSearchParams()

  if (filters.tenant_id.trim()) {
    params.set('tenant_id', filters.tenant_id.trim())
  }

  if (filters.user_id.trim()) {
    params.set('user_id', filters.user_id.trim())
  }

  if ((status ?? '').trim()) {
    params.set('status', (status ?? '').trim())
  }

  return params.toString()
}

function normalizeCandidateFact(input: MemoryCandidateFactWire): MemoryCandidateFact {
  return {
    id: input.id ?? input.ID ?? 0,
    tenant_id: input.tenant_id ?? input.TenantID ?? '',
    user_id: input.user_id ?? input.UserID ?? '',
    fact_key: input.fact_key ?? input.Key ?? '',
    fact_value: input.fact_value ?? input.Value ?? '',
    source_text: input.source_text ?? input.SourceText ?? '',
    status: input.status ?? input.Status ?? '',
    source_message_seq: input.source_message_seq ?? input.SourceMessageSeq ?? 0,
    confirmation_count: input.confirmation_count ?? input.ConfirmationCount ?? 0,
    created_at: input.created_at ?? input.CreatedAt,
    updated_at: input.updated_at ?? input.UpdatedAt,
  }
}

function normalizeProjectFact(input: MemoryProjectFactWire): MemoryProjectFact {
  return {
    id: input.id ?? input.ID ?? 0,
    tenant_id: input.tenant_id ?? input.TenantID ?? '',
    user_id: input.user_id ?? input.UserID ?? '',
    fact_key: input.fact_key ?? input.Key ?? '',
    fact_value: input.fact_value ?? input.Value ?? '',
    source_text: input.source_text ?? input.SourceText ?? '',
    status: input.status ?? input.Status ?? '',
    superseded_by: input.superseded_by ?? input.SupersededBy,
    source_message_seq: input.source_message_seq ?? input.SourceMessageSeq ?? 0,
    last_verified_at: input.last_verified_at ?? input.LastVerifiedAt,
    created_at: input.created_at ?? input.CreatedAt,
    updated_at: input.updated_at ?? input.UpdatedAt,
  }
}

function normalizeBatchActionResult(input: MemoryCandidateFactBatchActionResultWire): MemoryCandidateFactBatchActionResult {
  return {
    fact_key: input.fact_key ?? '',
    tenant_id: input.tenant_id ?? '',
    user_id: input.user_id ?? '',
    status: input.status,
    fact: input.fact ? normalizeCandidateFact(input.fact) : undefined,
    error: input.error
      ? {
          message: input.error.message ?? '',
          type: input.error.type ?? '',
        }
      : undefined,
  }
}

function normalizeBatchActionResponse(input: MemoryCandidateFactBatchActionResponseWire): MemoryCandidateFactBatchActionResponse {
  return {
    action: input.action ?? '',
    success_count: input.success_count ?? 0,
    failure_count: input.failure_count ?? 0,
    results: (input.results ?? []).map(normalizeBatchActionResult),
  }
}

export async function listMemoryCandidateFacts(filters: MemoryCandidateFactFilters) {
  const query = buildMemoryQuery(filters, filters.status)
  const path = query ? `/admin/memory/candidate-facts?${query}` : '/admin/memory/candidate-facts'
  const response = await apiRequest<MemoryListResponse<MemoryCandidateFactWire>>(path)
  return {
    ...response,
    data: response.data.map(normalizeCandidateFact),
  }
}

export async function listMemoryProjectFacts(filters: MemoryProjectFactFilters) {
  const query = buildMemoryQuery(filters, filters.status)
  const path = query ? `/admin/memory/project-facts?${query}` : '/admin/memory/project-facts'
  const response = await apiRequest<MemoryListResponse<MemoryProjectFactWire>>(path)
  return {
    ...response,
    data: response.data.map(normalizeProjectFact),
  }
}

export async function submitMemoryCandidateFactAction(
  factKey: string,
  action: MemoryFactAction,
  input: MemoryCandidateFactActionRequest,
) {
  const response = await jsonRequest<MemoryCandidateFactWire>(`/admin/memory/candidate-facts/${encodeURIComponent(factKey)}/${action}`, input)
  return normalizeCandidateFact(response)
}

export async function submitMemoryCandidateFactBatchAction(
  action: MemoryFactAction,
  input: MemoryCandidateFactBatchActionRequest,
) {
  const response = await jsonRequest<MemoryCandidateFactBatchActionResponseWire>(`/admin/memory/candidate-facts/actions/${action}`, input)
  return normalizeBatchActionResponse(response)
}
