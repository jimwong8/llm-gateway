import { test, expect } from './fixtures'
import { uniqueId, cleanupAdminChannels } from './helpers'

test.describe('管理功能', () => {

  test.beforeEach(async ({ request }) => {
    await cleanupAdminChannels(request)
  })

  test('渠道管理 — 页面加载', async ({ adminPage }) => {
    await adminPage.goto('channels')
    await adminPage.waitForSelector('h1')
    await expect(adminPage.locator('h1')).toContainText('渠道管理')
  })

  test('渠道管理 — 搜索框存在', async ({ adminPage }) => {
    await adminPage.goto('channels')
    await adminPage.waitForSelector('.channels-page')
    const searchInput = adminPage.locator('input[placeholder*="搜索"]').first()
    await expect(searchInput).toBeVisible()
  })

  test('渠道管理 — 状态筛选下拉存在', async ({ adminPage }) => {
    await adminPage.goto('channels')
    await adminPage.waitForSelector('.channels-page')
    const filter = adminPage.locator('select.channels-filter').first()
    await expect(filter).toBeVisible()
  })

  test('渠道管理 — 添加渠道按钮存在', async ({ adminPage }) => {
    await adminPage.goto('channels')
    await adminPage.waitForSelector('.channels-page')
    const addBtn = adminPage.locator('button:has-text("添加渠道")').first()
    await expect(addBtn).toBeVisible()
  })

  test('管理员 — 健康检查端点可达', async ({ request }) => {
    const adminToken = process.env.E2E_ADMIN_KEY ?? 'sk-admin-e2e-test'
    const res = await request.get(`${process.env.E2E_API_BASE ?? 'http://localhost:8080'}/admin/health`, {
      headers: { 'Authorization': `Bearer ${adminToken}` },
    })
    expect(res.ok()).toBeTruthy()
    const body = await res.json()
    expect(body).toHaveProperty('service')
  })

  test('定价管理 — 页面加载', async ({ adminPage }) => {
    await adminPage.goto('billing-pricing')
    await adminPage.waitForSelector('h1')
    await expect(adminPage.locator('h1')).toContainText('定价管理')
  })

  test('定价管理 — 表单元素存在', async ({ adminPage }) => {
    await adminPage.goto('billing-pricing')
    await adminPage.waitForSelector('h1')
    const formInputs = adminPage.locator('input[placeholder="openai"], input[placeholder="gpt-4"]')
    const count = await formInputs.count()
    expect(count).toBeGreaterThanOrEqual(1)
  })
})
