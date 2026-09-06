export interface Webhook {
  id: string
  url: string
  events: string[]
  secret?: string
  enabled: boolean
  created_at: string
}

export const WEBHOOK_EVENTS = [
  'chat.completed',
  'chat.failed',
  'provider.error',
  'quota.exceeded',
  'billing.updated',
  'key.created',
  'key.revoked',
  'tenant.created',
  'tenant.updated',
]

export const WEBHOOK_EVENT_LABELS: Record<string, string> = {
  'chat.completed': '对话完成',
  'chat.failed': '对话失败',
  'provider.error': 'Provider 错误',
  'quota.exceeded': '配额超限',
  'billing.updated': '账单更新',
  'key.created': '密钥创建',
  'key.revoked': '密钥撤销',
  'tenant.created': '租户创建',
  'tenant.updated': '租户更新',
}
