import { test, expect } from './fixtures'
import { uniqueId } from './helpers'

test.describe('Presets 页面', () => {

  test.beforeEach(async ({ userPage }) => {
    await userPage.goto('presets')
    await userPage.waitForSelector('.presets-section, .masks-section', { timeout: 10_000 })
  })

  // ── Prompt Presets ─────────────────────────────────────

  test('管理员登录 → 导航到 /presets 页面加载成功', async ({ userPage }) => {
    await expect(userPage.locator('h1')).toContainText('Prompt & Mask 管理')
    // 确认 Prompt Presets Tab 默认激活
    await expect(userPage.locator('.presets-section')).toBeVisible()
  })

  test('创建 Prompt Preset — 填写名称、模板、变量', async ({ userPage }) => {
    const name = uniqueId('e2e-preset')
    const systemPrompt = 'You are a helpful assistant for {{user_name}} working on {{task_desc}}.'

    // 点击"新建 Preset"按钮
    await userPage.click('button:has-text("新建 Preset")')
    await userPage.waitForSelector('form.page-surface')

    // 填写表单
    await userPage.fill('input[placeholder="My Preset"]', name)
    await userPage.fill('textarea[placeholder*="You are a helpful assistant"]', systemPrompt)
    await userPage.fill('input[placeholder="user_name, task_desc"]', 'user_name, task_desc')

    // 验证变量替换预览
    await expect(userPage.locator('text=替换预览')).toBeVisible()

    // 提交创建
    await userPage.click('button:has-text("创建")')

    // 等待列表刷新，验证新 preset 出现在表格中
    await expect(userPage.locator(`table tbody tr:has-text("${name}")`)).toBeVisible({ timeout: 10_000 })
  })

  test('验证列表显示新建的 Preset — 包含名称', async ({ userPage }) => {
    const name = uniqueId('e2e-preset-list')

    await userPage.click('button:has-text("新建 Preset")')
    await userPage.waitForSelector('form.page-surface')

    await userPage.fill('input[placeholder="My Preset"]', name)
    await userPage.fill('textarea[placeholder*="You are a helpful assistant"]', 'Test prompt')
    await userPage.click('button:has-text("创建")')

    // 验证表格行包含名称字段
    const row = userPage.locator(`table tbody tr:has-text("${name}")`)
    await expect(row).toBeVisible({ timeout: 10_000 })
    await expect(row.locator('td').nth(0)).toContainText(name)
  })

  test('编辑 Preset — 修改名称', async ({ userPage }) => {
    const originalName = uniqueId('e2e-preset-edit')
    const updatedName = 'updated-' + uniqueId('e2e-preset')

    // 先创建一个 preset
    await userPage.click('button:has-text("新建 Preset")')
    await userPage.waitForSelector('form.page-surface')
    await userPage.fill('input[placeholder="My Preset"]', originalName)
    await userPage.fill('textarea[placeholder*="You are a helpful assistant"]', 'Test prompt')
    await userPage.click('button:has-text("创建")')
    await expect(userPage.locator(`table tbody tr:has-text("${originalName}")`)).toBeVisible({ timeout: 10_000 })

    // 点击编辑按钮
    const row = userPage.locator(`table tbody tr:has-text("${originalName}")`)
    await row.locator('button:has-text("编辑")').click()

    // 表单应切换为编辑模式
    await userPage.waitForSelector('form.page-surface h3:has-text("编辑 Preset")')

    // 修改名称
    await userPage.fill('input[placeholder="My Preset"]', updatedName)
    await userPage.click('button:has-text("保存")')

    // 验证列表显示更新后的名称
    await expect(userPage.locator(`table tbody tr:has-text("${updatedName}")`)).toBeVisible({ timeout: 10_000 })
    // 旧名称不再出现（使用 10s timeout）
    await expect(userPage.locator(`table tbody tr:has-text("${originalName}")`)).toHaveCount(0, { timeout: 10_000 })
  })

  test('删除 Preset', async ({ userPage }) => {
    const name = uniqueId('e2e-preset-delete')

    // 先创建一个 preset
    await userPage.click('button:has-text("新建 Preset")')
    await userPage.waitForSelector('form.page-surface')
    await userPage.fill('input[placeholder="My Preset"]', name)
    await userPage.fill('textarea[placeholder*="You are a helpful assistant"]', 'Test prompt')
    await userPage.click('button:has-text("创建")')
    await expect(userPage.locator(`table tbody tr:has-text("${name}")`)).toBeVisible({ timeout: 10_000 })

    // 点击删除按钮，确认弹窗（需在 click 前注册 dialog handler）
    const row = userPage.locator(`table tbody tr:has-text("${name}")`)
    userPage.once('dialog', async (dialog) => {
      await dialog.accept()
    })
    await row.locator('button:has-text("删除")').click()

    // 等待删除完成，验证行消失
    await expect(userPage.locator(`table tbody tr:has-text("${name}")`)).toHaveCount(0, { timeout: 10_000 })
  })

  test('取消创建 Preset — 表单关闭，列表不变', async ({ userPage }) => {
    await userPage.click('button:has-text("新建 Preset")')
    await userPage.waitForSelector('form.page-surface')

    // 填写部分数据
    await userPage.fill('input[placeholder="My Preset"]', uniqueId('e2e-cancel'))
    await userPage.click('button:has-text("取消")')

    // 表单应消失
    await expect(userPage.locator('form.page-surface')).toHaveCount(0)
  })

  // ── Mask Rules ─────────────────────────────────────────

  test('切换到 Mask Rules Tab', async ({ userPage }) => {
    await userPage.click('button:has-text("Mask Rules")')
    await expect(userPage.locator('.masks-section')).toBeVisible()
    // 确认空状态或列表存在
    const emptyState = userPage.locator('text=暂无 Mask Rules')
    const table = userPage.locator('.masks-section table')
    const isVisible = await emptyState.isVisible().catch(() => false)
    const isTable = await table.isVisible().catch(() => false)
    expect(isVisible || isTable).toBeTruthy()
  })

  test('创建 Mask Rule', async ({ userPage }) => {
    const name = uniqueId('e2e-mask')
    const pattern = '1[3-9]\\d{9}'

    // 切换到 Mask Rules Tab
    await userPage.click('button:has-text("Mask Rules")')
    await userPage.waitForSelector('.masks-section')

    // 点击"新建 Mask Rule"
    await userPage.click('button:has-text("新建 Mask Rule")')
    await userPage.waitForSelector('form.page-surface')

    // 填写表单
    await userPage.fill('input[placeholder="手机号脱敏"]', name)
    await userPage.fill('input[placeholder="1[3-9]\\\\d{9}"]', pattern)
    await userPage.fill('input[placeholder="***"]', '***PHONE***')

    // 提交创建
    await userPage.click('button:has-text("创建")')

    // 验证列表显示新 mask rule
    await expect(userPage.locator(`table tbody tr:has-text("${name}")`)).toBeVisible({ timeout: 10_000 })
  })

  test('切换 Mask Rule 启用/停用', async ({ userPage }) => {
    const name = uniqueId('e2e-mask-toggle')

    // 切换到 Mask Rules Tab 并创建
    await userPage.click('button:has-text("Mask Rules")')
    await userPage.waitForSelector('.masks-section')

    await userPage.click('button:has-text("新建 Mask Rule")')
    await userPage.waitForSelector('form.page-surface')
    await userPage.fill('input[placeholder="手机号脱敏"]', name)
    await userPage.fill('input[placeholder="1[3-9]\\\\d{9}"]', 'test-pattern')
    await userPage.click('button:has-text("创建")')
    await expect(userPage.locator(`table tbody tr:has-text("${name}")`)).toBeVisible({ timeout: 10_000 })

    // 初始状态应为"启用"
    const row = userPage.locator(`table tbody tr:has-text("${name}")`)
    const statusBtn = row.locator('button.badge')
    await expect(statusBtn).toContainText('启用')

    // 点击切换为停用（需在 click 前注册 dialog handler）
    userPage.once('dialog', (d) => d.accept())
    await statusBtn.click()
    await userPage.waitForTimeout(2000)

    // 验证状态变为"停用"
    await expect(row.locator('button.badge')).toContainText('停用', { timeout: 15_000 })

    // 再次点击切换回启用
    userPage.once('dialog', (d) => d.accept())
    await row.locator('button.badge').click()
    await userPage.waitForTimeout(2000)

    // 验证状态变回"启用"
    await expect(row.locator('button.badge')).toContainText('启用', { timeout: 10_000 })
  })

  test('删除 Mask Rule', async ({ userPage }) => {
    const name = uniqueId('e2e-mask-delete')

    // 切换到 Mask Rules Tab 并创建
    await userPage.click('button:has-text("Mask Rules")')
    await userPage.waitForSelector('.masks-section')

    await userPage.click('button:has-text("新建 Mask Rule")')
    await userPage.waitForSelector('form.page-surface')
    await userPage.fill('input[placeholder="手机号脱敏"]', name)
    await userPage.fill('input[placeholder="1[3-9]\\\\d{9}"]', 'delete-pattern')
    await userPage.click('button:has-text("创建")')
    await expect(userPage.locator(`table tbody tr:has-text("${name}")`)).toBeVisible({ timeout: 10_000 })

    // 点击删除按钮（需在 click 前注册 dialog handler）
    const row = userPage.locator(`table tbody tr:has-text("${name}")`)
    userPage.once('dialog', async (dialog) => {
      await dialog.accept()
    })
    await row.locator('button:has-text("删除")').click()

    // 等待删除完成，验证行消失
    await expect(userPage.locator(`table tbody tr:has-text("${name}")`)).toHaveCount(0, { timeout: 10_000 })
  })
})
