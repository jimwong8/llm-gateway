import { apiRequest, jsonRequest } from '../http'
import type { MaskRule, MaskRuleInput, PromptPreset, PromptPresetInput } from '../../types/preset'

// ── Prompt Presets ──────────────────────────────────────

export function fetchPresets() {
  return apiRequest<PromptPreset[]>('/api/memory/presets')
}

export function createPreset(input: PromptPresetInput) {
  return jsonRequest<PromptPreset>('/api/memory/presets', input)
}

export function updatePreset(id: number, input: Partial<PromptPresetInput>) {
  return jsonRequest<PromptPreset>(`/api/memory/presets/${id}`, input, { method: 'PUT' } as RequestInit)
}

export function deletePreset(id: number) {
  return apiRequest<void>(`/api/memory/presets/${id}`, { method: 'DELETE' } as RequestInit)
}

// ── Mask Rules ──────────────────────────────────────────

export function fetchMaskRules() {
  return apiRequest<MaskRule[]>('/api/memory/masks')
}

export function createMaskRule(input: MaskRuleInput) {
  return jsonRequest<MaskRule>('/api/memory/masks', input)
}

export function updateMaskRule(id: number, input: Partial<MaskRuleInput>) {
  return jsonRequest<MaskRule>(`/api/memory/masks/${id}`, input, { method: 'PUT' } as RequestInit)
}

export function deleteMaskRule(id: number) {
  return apiRequest<void>(`/api/memory/masks/${id}`, { method: 'DELETE' } as RequestInit)
}
