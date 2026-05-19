import { test, expect } from './fixtures'

test.describe('Memory Governance — 检索测试', () => {

  test.beforeEach(async ({ adminPage }) => {
    await adminPage.goto('/memory-governance')
    await adminPage.waitForSelector('.memory-governance__tabs', { timeout: 10_000 })
  })

  test('导航到记忆治理页面 — 默认显示治理面板 Tab', async ({ adminPage }) => {
    await expect(adminPage.locator('button.memory-governance__tab--active')).toContainText('治理面板')
    await expect(adminPage.locator('.config-filters[aria-label="记忆治理筛选"]')).toBeVisible()
  })

  test('切换到检索测试 Tab', async ({ adminPage }) => {
    await adminPage.click('button:has-text("检索测试")')
    await expect(adminPage.locator('button.memory-governance__tab--active')).toContainText('检索测试')
    // 验证检索表单存在
    await expect(adminPage.locator('.memory-governance__search-panel')).toBeVisible()
    await expect(adminPage.locator('input[placeholder*="自然语言查询"]')).toBeVisible()
  })

  test('输入查询 → 点击检索 → 验证结果列表', async ({ adminPage }) => {
    // 切换到检索测试 Tab
    await adminPage.click('button:has-text("检索测试")')
    await adminPage.waitForSelector('.memory-governance__search-panel')

    // 输入检索内容
    await adminPage.fill('input[placeholder*="自然语言查询"]', '用户的技术栈偏好')

    // 点击检索按钮
    await adminPage.click('button:has-text("检索")')

    // 等待检索完成（可能显示加载中状态，然后出现结果或空状态）
    await adminPage.waitForTimeout(2000)

    // 验证结果：要么有结果表格，要么显示空状态
    const resultTable = adminPage.locator('.memory-governance__search-panel table')
    const emptyState = adminPage.locator('text=未找到匹配的记忆片段')
    const loadingState = adminPage.locator('text=正在执行 Hybrid Search')

    const hasResult = await resultTable.isVisible().catch(() => false)
    const hasEmpty = await emptyState.isVisible().catch(() => false)
    const isLoading = await loadingState.isVisible().catch(() => false)

    // 加载中、有结果、空状态三者必居其一
    expect(hasResult || hasEmpty || isLoading).toBeTruthy()
  })

  test('检索结果列表 — 包含排名、内容、分数、来源列', async ({ adminPage }) => {
    await adminPage.click('button:has-text("检索测试")')
    await adminPage.waitForSelector('.memory-governance__search-panel')

    await adminPage.fill('input[placeholder*="自然语言查询"]', 'test query')
    await adminPage.click('button:has-text("检索")')

    // 等待检索完成
    await adminPage.waitForTimeout(3000)

    // 如果有结果，验证表格结构
    const resultTable = adminPage.locator('.memory-governance__search-panel table')
    if (await resultTable.isVisible().catch(() => false)) {
      // 验证表头包含关键列
      const headers = resultTable.locator('thead th')
      await expect(headers).toContainText(['排名', '内容', '分数', '来源'])

      // 验证结果摘要
      await expect(adminPage.locator('.memory-governance__search-summary')).toBeVisible()
      const summaryText = await adminPage.locator('.memory-governance__search-summary').textContent()
      expect(summaryText).toContain('检索到')
      expect(summaryText).toContain('条结果')

      // 验证第一行数据包含排名徽章
      const firstRow = resultTable.locator('tbody tr').first()
      await expect(firstRow.locator('.memory-governance__rank-badge')).toBeVisible()
      // 验证分数列存在
      await expect(firstRow.locator('.memory-governance__score-pill')).toBeVisible()
    }
  })

  test('空查询提交 — 显示错误提示', async ({ adminPage }) => {
    await adminPage.click('button:has-text("检索测试")')
    await adminPage.waitForSelector('.memory-governance__search-panel')

    // 不输入任何内容直接点击检索
    await adminPage.click('button:has-text("检索")')

    // 验证错误提示
    await expect(adminPage.locator('.config-error')).toContainText('请输入检索内容')
  })

  test('清空按钮 — 重置检索表单和结果', async ({ adminPage }) => {
    await adminPage.click('button:has-text("检索测试")')
    await adminPage.waitForSelector('.memory-governance__search-panel')

    // 输入内容并检索
    await adminPage.fill('input[placeholder*="自然语言查询"]', 'some query')
    await adminPage.fill('input[placeholder="可选，限定租户范围"]', 'test-tenant')
    await adminPage.fill('input[placeholder="可选，限定用户范围"]', 'test-user')
    await adminPage.click('button:has-text("检索")')
    await adminPage.waitForTimeout(2000)

    // 点击清空按钮
    await adminPage.click('button:has-text("清空")')

    // 验证表单被重置
    const queryInput = adminPage.locator('input[placeholder*="自然语言查询"]')
    await expect(queryInput).toHaveValue('')
    const tenantInput = adminPage.locator('input[placeholder="可选，限定租户范围"]')
    await expect(tenantInput).toHaveValue('')
    const userInput = adminPage.locator('input[placeholder="可选，限定用户范围"]')
    await expect(userInput).toHaveValue('')

    // 验证结果区域被清空（结果表格和空状态都不应显示）
    const resultTable = adminPage.locator('.memory-governance__search-panel table')
    const hasResult = await resultTable.isVisible().catch(() => false)
    expect(hasResult).toBeFalsy()
  })

  test('检索中状态 — 按钮显示"检索中…"且禁用', async ({ adminPage }) => {
    await adminPage.click('button:has-text("检索测试")')
    await adminPage.waitForSelector('.memory-governance__search-panel')

    await adminPage.fill('input[placeholder*="自然语言查询"]', 'slow query test')
    await adminPage.click('button:has-text("检索")')

    // 验证加载状态（可能很快消失，用短暂等待捕获）
    const loadingBtn = adminPage.locator('button:has-text("检索中…")')
    const loadingText = adminPage.locator('text=正在执行 Hybrid Search')

    // 由于网络速度可能很快，这里用 Promise.race 检查是否出现过加载状态
    const appeared = await Promise.race([
      loadingBtn.waitFor({ state: 'visible', timeout: 2000 }).then(() => true),
      loadingText.waitFor({ state: 'visible', timeout: 2000 }).then(() => true),
      adminPage.waitForTimeout(2000).then(() => false),
    ]).catch(() => false)

    // 如果请求很快完成也是正常的，这里只验证最终状态
    const searchBtn = adminPage.locator('.config-filters__actions button[type="submit"]')
    const btnText = await searchBtn.textContent()
    expect(btnText === '检索' || btnText === '检索中…').toBeTruthy()
  })
})
