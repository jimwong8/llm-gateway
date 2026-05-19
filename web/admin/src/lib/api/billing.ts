import { getUserToken } from './identity'
import { apiRequest, jsonRequest } from '../http'
import type { LedgerEntry, PricingEntry, BalanceResponse } from '../../types/billing'

function userAuthHeaders(): HeadersInit {
  const token = getUserToken()
  if (token) {
    return { Authorization: `Bearer ${token}` }
  }
  return {}
}

async function userFetch(input: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init)
  if (res.status === 401) {
    sessionStorage.removeItem('llm_gateway_user_token')
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }
  return res
}

export async function getBalance(): Promise<BalanceResponse> {
  const res = await userFetch('/api/billing/balance', { headers: { ...userAuthHeaders() } })
  if (!res.ok) throw new Error('Failed to fetch balance')
  return res.json()
}

export async function getLedger(params?: {
  type?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}): Promise<{ object: string; data: LedgerEntry[] }> {
  const query = new URLSearchParams()
  if (params?.type) query.set('type', params.type)
  if (params?.from) query.set('from', params.from)
  if (params?.to) query.set('to', params.to)
  if (params?.limit) query.set('limit', String(params.limit))
  if (params?.offset) query.set('offset', String(params.offset))
  const qs = query.toString()
  const res = await userFetch(`/api/billing/ledger${qs ? '?' + qs : ''}`, {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) throw new Error('Failed to fetch ledger')
  return res.json()
}

export async function listPricing(): Promise<{ object: string; data: PricingEntry[] }> {
  return apiRequest<{ object: string; data: PricingEntry[] }>('/api/admin/billing/pricing')
}

export async function upsertPricing(pricing: {
  provider: string
  model?: string
  input_price_per_1k: number
  output_price_per_1k: number
  is_default?: boolean
}): Promise<{ status: string }> {
  return jsonRequest<{ status: string }>('/api/admin/billing/pricing', {
    provider: pricing.provider,
    model: pricing.model || '',
    input_price_per_1k: pricing.input_price_per_1k,
    output_price_per_1k: pricing.output_price_per_1k,
    is_default: pricing.is_default ?? false,
  })
}

export async function creditWallet(params: {
  user_id: string
  amount: number
  reference_id: string
  description?: string
}): Promise<LedgerEntry> {
  return jsonRequest<LedgerEntry>('/api/admin/billing/credit', params)
}
