import { test, expect } from './fixtures'
import { uniqueId, addPricingViaAPI } from './helpers'

test.describe('计费功能', () => {

  test('账单页面加载', async ({ userPage }) => {
    await userPage.goto('/billing')
    await userPage.waitForSelector('h2')
    await expect(userPage.locator('h2')).toContainText('计费面板')
  })

  test('账单页面 — 钱包余额卡片存在', async ({ userPage }) => {
    await userPage.goto('/billing')
    await userPage.waitForSelector('.summary-card-grid')
    const cards = userPage.locator('.summary-card-grid .summary-metric-card, .summary-card-grid > div')
    const count = await cards.count()
    expect(count).toBeGreaterThanOrEqual(1)
  })

  test('账单页面 — 交易流水表格存在', async ({ userPage }) => {
    await userPage.goto('/billing')
    await userPage.waitForSelector('.page-surface')
    const table = userPage.locator('table.data-table').first()
    const loading = userPage.locator('.event-state:has-text("加载中")')
    const isTableVisible = await table.isVisible().catch(() => false)
    const isLoadingVisible = await loading.isVisible().catch(() => false)
    expect(isTableVisible || isLoadingVisible).toBeTruthy()
  })

  test('账单页面 — 未认证跳转', async ({ page }) => {
    await page.goto('/billing')
    await page.waitForURL(/\/login/, { timeout: 5_000 })
    await expect(page.locator('.login-card')).toBeVisible()
  })

  test('定价管理 — 管理员添加定价配置', async ({ adminPage, request }) => {
    const provider = uniqueId('e2e-test-provider')
    const model = uniqueId('e2e-test-model')
    await addPricingViaAPI(request, {
      provider,
      model,
      input_price_per_1k: 0.001,
      output_price_per_1k: 0.002,
    })

    await adminPage.goto('/billing-pricing')
    await adminPage.waitForSelector('h2')
    await expect(adminPage.locator(`text=${provider}`)).toBeVisible({ timeout: 10_000 })
  })

  test('定价管理 — 表单验证错误', async ({ adminPage }) => {
    await adminPage.goto('/billing-pricing')
    await adminPage.waitForSelector('h2')
    await adminPage.click('button:has-text("保存")')
    const alert = adminPage.locator('[role="alert"]')
    await expect(alert).toBeVisible()
  })
})
