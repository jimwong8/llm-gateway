/**
 * File Parser E2E 测试
 *
 * ⚠️ 当前项目中不存在 File Parser / 文件上传解析功能。
 * 该测试文件为占位实现，使用 test.skip 标记所有用例。
 *
 * 当以下功能实现后，取消 skip 并调整选择器：
 *   - 文件上传页面（路由待定，如 /admin/ui/file-parser 或 /playground/parser）
 *   - 支持 .md 等格式的文件上传和解析
 *   - 不支持格式的错误提示
 *
 * 参考现有风格：
 *   - 使用 adminPage fixture（管理员权限上传文件）
 *   - 使用 uniqueId 生成唯一文件名
 *   - 使用 page.setInputFiles() 进行文件上传
 */

import { test, expect } from './fixtures'
import * as fs from 'node:fs'
import * as path from 'node:path'
import * as os from 'node:os'

test.describe('File Parser — 文件上传解析', () => {

  // 创建临时测试文件
  let tempDir: string

  test.beforeAll(() => {
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'e2e-file-parser-'))
    // 创建有效的 .md 测试文件
    fs.writeFileSync(
      path.join(tempDir, 'test-valid.md'),
      '# Test Document\n\nThis is a **markdown** file for E2E testing.\n\n- Item 1\n- Item 2\n\n## Section\n\nHello world.',
    )
    // 创建不支持的格式测试文件
    fs.writeFileSync(
      path.join(tempDir, 'test-unsupported.xyz'),
      'This file format is not supported.',
    )
  })

  test.afterAll(() => {
    fs.rmSync(tempDir, { recursive: true, force: true })
  })

  test.skip('上传 .md 文件 → 验证解析结果', async ({ adminPage }) => {
    // TODO: 替换为实际的文件上传页面路由
    // await adminPage.goto('/file-parser')
    // await adminPage.waitForSelector('.file-parser-page')

    // 使用 setInputFiles 上传文件
    // const fileInput = adminPage.locator('input[type="file"]')
    // await fileInput.setInputFiles(path.join(tempDir, 'test-valid.md'))

    // 等待解析完成
    // await adminPage.waitForSelector('.parser-result', { timeout: 10_000 })

    // 验证解析结果显示
    // await expect(adminPage.locator('.parser-result')).toBeVisible()
    // const content = await adminPage.locator('.parser-result').textContent()
    // expect(content).toContain('Test Document')
    // expect(content).toContain('markdown')
  })

  test.skip('上传不支持的格式 → 验证错误提示', async ({ adminPage }) => {
    // TODO: 替换为实际的文件上传页面路由
    // await adminPage.goto('/file-parser')
    // await adminPage.waitForSelector('.file-parser-page')

    // 上传不支持的文件格式
    // const fileInput = adminPage.locator('input[type="file"]')
    // await fileInput.setInputFiles(path.join(tempDir, 'test-unsupported.xyz'))

    // 等待错误提示出现
    // await adminPage.waitForSelector('.config-error, [role="alert"]', { timeout: 10_000 })

    // 验证错误提示内容
    // const errorText = await adminPage.locator('.config-error, [role="alert"]').textContent()
    // expect(errorText).toMatch(/不支持|格式|无法解析|unsupported/i)
  })

  test.skip('拖拽上传 .md 文件 → 验证解析结果', async ({ adminPage }) => {
    // TODO: 如果页面支持拖拽上传，测试拖拽交互
    // await adminPage.goto('/file-parser')
    // const dropZone = adminPage.locator('.drop-zone, [class*="upload"]')
    // await dropZone.dispatchEvent('drop', {
    //   dataTransfer: { files: [path.join(tempDir, 'test-valid.md')] },
    // })
    // await expect(adminPage.locator('.parser-result')).toBeVisible({ timeout: 10_000 })
  })

  test.skip('空文件上传 → 提示选择文件', async ({ adminPage }) => {
    // TODO: 验证未选择文件时的状态
    // await adminPage.goto('/file-parser')
    // const uploadBtn = adminPage.locator('button:has-text("上传"), button:has-text("解析")')
    // const isDisabled = await uploadBtn.isDisabled().catch(() => false)
    // expect(isDisabled).toBeTruthy()
  })
})
