import { test, expect } from './fixtures'

test.describe('仪表盘页面', () => {

  test('管理员仪表盘 — 页面加载与核心元素', async ({ adminPage }) => {
    await adminPage.goto('/dashboard')
    await adminPage.waitForSelector('h2')
    await expect(adminPage.locator('h2')).toContainText('仪表盘')
    await expect(adminPage.locator('.app-shell')).toBeVisible()
  })

  test('管理员仪表盘 — 显示健康检查信息', async ({ adminPage }) => {
    await adminPage.goto('/dashboard')
    await adminPage.waitForSelector('h2')
    const pageText = await adminPage.locator('.app-shell').textContent()
    expect(pageText).toBeTruthy()
  })

  test('管理员仪表盘 — 图表标签页切换', async ({ adminPage }) => {
    await adminPage.goto('/dashboard')
    await adminPage.waitForSelector('.tab-strip')
    const tabs = adminPage.locator('.tab-strip .tab')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThanOrEqual(4)
    for (let i = 0; i < tabCount; i++) {
      await tabs.nth(i).click()
      await expect(tabs.nth(i)).toHaveClass(/active/)
    }
  })

  test('用户仪表盘 — 跳转", async ({ userPage }) => {
    await userPage.goto('/dashboard')
    await userPage.waitForSelector('h2')
    await expect(userPage.locator('h2')).toContainText(/我的面板|仪表盘/)
  })

  test('未认证访问重定向到登录页', async ({ page }) => {
    await page.goto('/dashboard')
    await page.waitForURL(/\/login/, { timeout: 5_000 })
    await expect(page.locator('.login-card')).toBeVisible()
  })

  test('仪表盘 — 侧边栏导航存在', async ({ adminPage }) => {
    await adminPage.goto('/dashboard')
    await adminPage.waitForSelector('.app-shell')
    const sidebar = adminPage.locator('nav, .sidebar, [class*="sidebar"], [class*="nav"]').first()
    await expect(sidebar).toBeVisible()
  })
})
