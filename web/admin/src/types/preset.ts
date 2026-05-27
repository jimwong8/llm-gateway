/** Prompt Preset — 预定义的提示词模板 */
export type PromptPreset = {
  id: number
  user_id?: number
  tenant_id: string
  name: string
  description?: string
  template: string
  variables?: string
  tags?: string[]
  is_public?: boolean
  created_at?: string
  updated_at?: string
}

export type PromptPresetInput = {
  tenant_id?: string
  name: string
  description?: string
  template: string
  variables?: string[]
  tags?: string[]
  is_public?: boolean
}

/** Mask Rule — 敏感信息脱敏规则 */
export type MaskRule = {
  id: number
  user_id?: number
  tenant_id: string
  name: string
  pattern: string
  replace: string
  is_active: boolean
  created_at?: string
}

export type MaskRuleInput = {
  tenant_id?: string
  name: string
  pattern: string
  replacement?: string
  enabled?: boolean
}
