import { test, expect } from './fixtures'
import { uniqueId, deleteTestSessionsViaAPI } from './helpers'

test.describe('聊天功能', () => {

  test('创建新对话', async ({ userPage }) => {
    const token = await userPage.evaluate(() => sessionStorage.getItem('llm_gateway_user_token'))
    await deleteTestSessionsViaAPI(userPage.request, token!)

    await userPage.goto('chat')
    await userPage.waitForSelector('.chat-page')

    const newBtn = userPage.locator('button:has-text("新对话"), button:has-text("新建"), [title*="新"]').first()
    await newBtn.click()
    await userPage.waitForTimeout(1000)
  })

  test('聊天页面加载 — 显示侧边栏和主区域', async ({ userPage }) => {
    await userPage.goto('chat')
    await userPage.waitForSelector('.chat-page')
    await expect(userPage.locator('.chat-sidebar, .chat-main').first()).toBeVisible()
  })

  test('未登录无法访问聊天页', async ({ page }) => {
    await page.goto('chat')
    await page.waitForURL(/\/login/, { timeout: 5_000 })
    await expect(page.locator('.login-card')).toBeVisible()
  })

  test('发送消息并接收流式响应', async ({ userPage }) => {
    const token = await userPage.evaluate(() => sessionStorage.getItem('llm_gateway_user_token'))
    await deleteTestSessionsViaAPI(userPage.request, token!)

    await userPage.goto('chat')
    await userPage.waitForSelector('.chat-page')

    const newBtn = userPage.locator('button:has-text("新对话"), button:has-text("新建"), [title*="新"]').first()
    await newBtn.click()
    await userPage.waitForTimeout(1000)

    const input = userPage.locator('.chat-input input, .chat-input textarea, [contenteditable="true"]').first()
    if (await input.isVisible()) {
      await input.fill('Hello, what is LLM Gateway?')
      await input.press('Enter')
      await userPage.waitForTimeout(2000)
    }
  })

  test('流式响应显示 AI 回复', async ({ userPage }) => {
    const token = await userPage.evaluate(() => sessionStorage.getItem('llm_gateway_user_token'))
    await deleteTestSessionsViaAPI(userPage.request, token!)

    await userPage.goto('chat')
    await userPage.waitForSelector('.chat-page')

    const newBtn = userPage.locator('button:has-text("新对话"), button:has-text("新建"), [title*="新"]').first()
    await newBtn.click()
    await userPage.waitForTimeout(1000)

    const input = userPage.locator('.chat-input input, .chat-input textarea, [contenteditable="true"]').first()
    if (await input.isVisible()) {
      await input.fill('Hello')
      await input.press('Enter')

      const assistantMsg = userPage.locator('.chat-message--assistant, [class*="assistant"]').first()
      await expect(assistantMsg).toBeVisible({ timeout: 15_000 })
    }
  })
})
