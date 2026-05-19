import { getUserToken } from './identity'
import type { MaskRule, MaskRuleInput, PromptPreset, PromptPresetInput } from '../../types/preset'

function userAuthHeaders(): HeadersInit {
  const token = getUserToken()
  return token ? { 'Authorization': `Bearer ${token}` } : {}
}

async function userFetch(input: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init)
  if (res.status === 401 || res.status === 403) {
    window.location.href = '/admin/ui/login'
  }
  return res
}

// ── Prompt Presets ──────────────────────────────────────

export async function fetchPresets(): Promise<PromptPreset[]> {
  const res = await userFetch('/api/memory/presets', { headers: { ...userAuthHeaders() } })
  if (!res.ok) throw new Error(await res.text())
  const json = await res.json()
  // 后端返回 {"data": [...]}
  return Array.isArray(json) ? json : (json.data ?? [])
}

export async function createPreset(input: PromptPresetInput): Promise<PromptPreset> {
  const res = await userFetch('/api/memory/presets', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...userAuthHeaders() },
    body: JSON.stringify(input),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updatePreset(id: number, input: Partial<PromptPresetInput>): Promise<PromptPreset> {
  const res = await userFetch(`/api/memory/presets/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...userAuthHeaders() },
    body: JSON.stringify(input),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deletePreset(id: number): Promise<void> {
  const res = await userFetch(`/api/memory/presets/${id}`, {
    method: 'DELETE',
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) throw new Error(await res.text())
}

// ── Mask Rules ──────────────────────────────────────────

export async function fetchMaskRules(): Promise<MaskRule[]> {
  const res = await userFetch('/api/memory/masks', { headers: { ...userAuthHeaders() } })
  if (!res.ok) throw new Error(await res.text())
  const json = await res.json()
  // 后端返回 {"data": [...]}
  return Array.isArray(json) ? json : (json.data ?? [])
}

export async function createMaskRule(input: MaskRuleInput): Promise<MaskRule> {
  const res = await userFetch('/api/memory/masks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...userAuthHeaders() },
    body: JSON.stringify({
      name: input.name,
      pattern: input.pattern,
      replace: input.replacement ?? '***',
      tenant_id: input.tenant_id ?? 'default',
    }),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateMaskRule(id: number, input: { is_active?: boolean; name?: string; pattern?: string; replacement?: string }): Promise<MaskRule> {
  const res = await userFetch(`/api/memory/masks/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...userAuthHeaders() },
    body: JSON.stringify({
      ...(input.name !== undefined && { name: input.name }),
      ...(input.pattern !== undefined && { pattern: input.pattern }),
      ...(input.replacement !== undefined && { replace: input.replacement }),
      ...(input.is_active !== undefined && { enabled: input.is_active }),
    }),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deleteMaskRule(id: number): Promise<void> {
  const res = await userFetch(`/api/memory/masks/${id}`, {
    method: 'DELETE',
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) throw new Error(await res.text())
}
