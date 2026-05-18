/** Prompt Preset — 预定义的提示词模板 */
export type PromptPreset = {
  id: number
  tenant_id: string
  name: string
  system_prompt: string
  model: string
  temperature: number
  max_tokens: number
  created_at?: string
  updated_at?: string
}

export type PromptPresetInput = {
  tenant_id: string
  name: string
  system_prompt: string
  model: string
  temperature?: number
  max_tokens?: number
}

/** Mask Rule — 敏感信息脱敏规则 */
export type MaskRule = {
  id: number
  tenant_id: string
  name: string
  pattern: string
  replacement: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export type MaskRuleInput = {
  tenant_id: string
  name: string
  pattern: string
  replacement?: string
  enabled?: boolean
}
