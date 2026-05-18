import { test, expect } from './fixtures'
import { loginAsAdmin, loginAsUser, signupUser, uniqueId } from './helpers'

test.describe('认证流程', () => {

  test('管理员 Token 登录', async ({ page }) => {
    const token = process.env.E2E_ADMIN_KEY ?? 'sk-admin-e2e-test'
    await loginAsAdmin(page, token)
    await expect(page.locator('h2')).toContainText('仪表盘')
    const stored = await page.evaluate(() => sessionStorage.getItem('llm_gateway_admin_token'))
    expect(stored).toBe(token)
  })

  test('管理员登录 — 空 Token 提示错误', async ({ page }) => {
    await page.goto('/login')
    await page.waitForSelector('.login-card')
    await page.click('button:has-text("管理员")')
    await page.fill('input[name="admin-token"]', '')
    await page.click('button:has-text("进入控制台")')
    await expect(page.locator('[role="alert"]')).toContainText('请输入管理员 Token')
  })

  test('管理员登录 — 短 Token 提示无效', async ({ page }) => {
    await page.goto('/login')
    await page.waitForSelector('.login-card')
    await page.click('button:has-text("管理员")')
    await page.fill('input[name="admin-token"]', 'ab')
    await page.click('button:has-text("进入控制台")')
    await expect(page.locator('[role="alert"]')).toContainText('Token 格式无效')
  })

  test('用户注册', async ({ page }) => {
    const email = uniqueId('e2e-auth') + '@test.example'
    const username = uniqueId('e2e-auth')
    const password = 'TestPass123!'
    await signupUser(page, email, username, password)
    await expect(page).toHaveURL(/\/dashboard/)
    const token = await page.evaluate(() => sessionStorage.getItem('llm_gateway_user_token'))
    expect(token).toBeTruthy()
  })

  test('用户注册 — 空字段提示错误', async ({ page }) => {
    await page.goto('/signup')
    await page.waitForSelector('.login-card')
    await page.click('button:has-text("注册")')
    await expect(page.locator('[role="alert"]')).toContainText('请填写所有字段')
  })

  test('用户注册 — 短密码提示错误', async ({ page }) => {
    await page.goto('/signup')
    await page.waitForSelector('.login-card')
    await page.fill('#email', 'test@test.example')
    await page.fill('#username', 'testuser')
    await page.fill('#password', '1234567')
    await page.click('button:has-text("注册")')
    await expect(page.locator('[role="alert"]')).toContainText('密码至少 8 个字符')
  })

  test('用户登录 — 成功后跳转仪表盘', async ({ page }) => {
    const email = uniqueId('e2e-auth') + '@test.example'
    const username = uniqueId('e2e-auth')
    const password = 'TestPass123!'
    await signupUser(page, email, username, password)

    const token = await page.evaluate(() => sessionStorage.getItem('llm_gateway_user_token'))
    await page.evaluate(() => sessionStorage.clear())
    await page.goto('/login')

    await loginAsUser(page, email, password)
    await expect(page).toHaveURL(/\/dashboard/)
    const newToken = await page.evaluate(() => sessionStorage.getItem('llm_gateway_user_token'))
    expect(newToken).toBeTruthy()
    expect(newToken).not.toBe(token)
  })

  test('用户登录 — 错误密码提示失败', async ({ page }) => {
    const email = uniqueId('e2e-auth') + '@test.example'
    const username = uniqueId('e2e-auth')
    const password = 'TestPass123!'
    await signupUser(page, email, username, password)
    await page.evaluate(() => sessionStorage.clear())
    await page.goto('/login')

    await page.click('button:has-text("用户登录")')
    await page.fill('#email', email)
    await page.fill('#password', 'WrongPassword!')
    await page.click('button:has-text("登录")')
    await expect(page.locator('[role="alert"]')).toBeVisible()
  })

  test('API Key 创建流程', async ({ page }) => {
    const email = uniqueId('e2e-auth') + '@test.example'
    const username = uniqueId('e2e-auth')
    const password = 'TestPass123!'
    await signupUser(page, email, username, password)

    await page.goto('/api-keys')
    await page.waitForSelector('h2')
    await expect(page.locator('h2')).toContainText('API 密钥管理')

    await page.fill('input[placeholder="密钥名称"]', 'e2e-test-key')
    await page.click('button:has-text("创建新密钥")')

    await expect(page.locator('text=新密钥已创建')).toBeVisible({ timeout: 10_000 })
    const keyText = await page.locator('code').first().textContent()
    expect(keyText).toBeTruthy()
    expect(keyText!.length).toBeGreaterThan(10)
  })
})
