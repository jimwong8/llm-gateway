import type { Page, APIRequestContext } from '@playwright/test'
import { expect } from '@playwright/test'

const ADMIN_TOKEN_KEY = 'llm_gateway_admin_token'
const USER_TOKEN_KEY = 'llm_gateway_user_token'

export const API_BASE = process.env.E2E_API_BASE ?? 'http://localhost:8080'

let serialCounter = Date.now()

export function uniqueId(prefix: string): string {
  serialCounter += 1
  return `${prefix}-${serialCounter}`
}

export async function loginAsAdmin(page: Page, token?: string) {
  const adminToken = token ?? process.env.E2E_ADMIN_KEY ?? 'sk-admin-e2e-test'
  await page.goto('login')
  await page.waitForSelector('.login-card', { timeout: 10_000 })
  await page.click('button:has-text("管理员")')
  await page.fill('#admin-token', adminToken)
  await page.click('button:has-text("进入控制台")')
  await page.waitForURL(/\/dashboard/, { timeout: 10_000 })
  const stored = await page.evaluate((k: string) => sessionStorage.getItem(k), ADMIN_TOKEN_KEY)
  expect(stored).toBe(adminToken)
}

export async function signupUser(page: Page, email: string, username: string, password: string) {
  await page.goto('signup')
  await page.waitForSelector('.login-card', { timeout: 10_000 })
  await page.fill('#email', email)
  await page.fill('#username', username)
  await page.fill('#password', password)
  await page.click('button:has-text("注册")')
  await page.waitForURL(/\/dashboard/, { timeout: 15_000 })
  await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
}

export async function loginAsUser(page: Page, email: string, password: string) {
  await page.goto('login')
  await page.waitForSelector('.login-card', { timeout: 10_000 })
  await page.getByRole('tab', { name: '用户登录' }).click()
  await page.waitForSelector('#email', { timeout: 5_000 })
  await page.fill('#email', email)
  await page.fill('#password', password)
  await page.click('button[type="submit"]')
  await page.waitForURL(/\/dashboard/, { timeout: 15_000 })
  await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
}

export async function createApiKeyViaAPI(request: APIRequestContext, userToken: string, name = 'e2e-key') {
  const res = await request.post(`${API_BASE}/api/user/api-keys`, {
    headers: {
      'Authorization': `Bearer ${userToken}`,
      'Content-Type': 'application/json',
    },
    data: { name },
  })
  expect(res.ok()).toBeTruthy()
  return res.json() as Promise<{ key: string; api_key: { id: number; name: string } }>
}

export async function deleteTestSessionsViaAPI(request: APIRequestContext, userToken: string) {
  const listRes = await request.get(`${API_BASE}/api/chat/sessions?limit=100&offset=0`, {
    headers: { 'Authorization': `Bearer ${userToken}` },
  })
  if (!listRes.ok()) return
  const contentType = listRes.headers()['content-type'] ?? ''
  if (!contentType.includes('application/json')) return
  const body = await listRes.json() as { data?: { id: number }[] }
  if (!body.data) return
  for (const session of body.data) {
    await request.delete(`${API_BASE}/api/chat/sessions/${session.id}`, {
      headers: { 'Authorization': `Bearer ${userToken}` },
    }).catch(() => {})
  }
}

export async function cleanupAdminChannels(request: APIRequestContext) {
  const adminToken = process.env.E2E_ADMIN_KEY ?? 'sk-admin-e2e-test'
  const res = await request.get(`${API_BASE}/admin/channels`, {
    headers: { 'Authorization': `Bearer ${adminToken}` },
  })
  if (!res.ok()) return
  const channels = await res.json() as { object?: string; data?: { id: string; name: string }[] }
  const list = channels?.data ?? (Array.isArray(channels) ? channels : [])
  for (const ch of list) {
    if (ch.name?.startsWith('e2e-')) {
      await request.delete(`${API_BASE}/admin/channels/${ch.id}`, {
        headers: { 'Authorization': `Bearer ${adminToken}` },
      }).catch(() => {})
    }
  }
}

export async function cleanupAdminPricing(request: APIRequestContext) {
  const adminToken = process.env.E2E_ADMIN_KEY ?? 'sk-admin-e2e-test'
  const res = await request.get(`${API_BASE}/admin/billing/pricing`, {
    headers: { 'Authorization': `Bearer ${adminToken}` },
  })
  if (!res.ok()) return
  const body = await res.json() as { data?: { provider: string; model?: string; is_default?: boolean }[] }
  const entries = body?.data ?? []
  for (const entry of entries) {
    if (entry.provider === 'e2e-test-provider') {
      continue
    }
  }
}

export async function addPricingViaAPI(request: APIRequestContext, pricing: {
  provider: string
  model?: string
  input_price_per_1k: number
  output_price_per_1k: number
  is_default?: boolean
}) {
  const adminToken = process.env.E2E_ADMIN_KEY ?? 'sk-admin-e2e-test'
  const res = await request.post(`${API_BASE}/api/admin/billing/pricing`, {
    headers: {
      'Authorization': `Bearer ${adminToken}`,
      'Content-Type': 'application/json',
    },
    data: {
      provider: pricing.provider,
      model: pricing.model ?? '',
      input_price_per_1k: pricing.input_price_per_1k,
      output_price_per_1k: pricing.output_price_per_1k,
      is_default: pricing.is_default ?? false,
    },
  })
  expect(res.ok()).toBeTruthy()
}

export async function cleanupTestUsers(request: APIRequestContext, emails: string[]) {
  const adminToken = process.env.E2E_ADMIN_KEY ?? 'sk-admin-e2e-test'
  for (const email of emails) {
    await request.delete(`${API_BASE}/api/admin/users/${encodeURIComponent(email)}`, {
      headers: { 'Authorization': `Bearer ${adminToken}` },
    }).catch(() => {})
  }
}
