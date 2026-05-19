import { test as base, expect } from '@playwright/test'
import { loginAsAdmin, signupUser, uniqueId, deleteTestSessionsViaAPI } from './helpers'
import type { Page } from '@playwright/test'

type E2EFixtures = {
  adminPage: Page
  userPage: Page
  testUser: { email: string; username: string; password: string; token?: string }
}

export const test = base.extend<E2EFixtures>({
  adminPage: async ({ browser }, use) => {
    const context = await browser.newContext({ storageState: undefined })
    const page = await context.newPage()
    const token = process.env.E2E_ADMIN_KEY ?? 'sk-admin-e2e-test'
    await loginAsAdmin(page, token)
    await use(page)
    await context.close()
  },

  userPage: async ({ browser }, use) => {
    const context = await browser.newContext({ storageState: undefined })
    const page = await context.newPage()
    const email = uniqueId('e2e-user') + '@test.example'
    const username = uniqueId('e2e-user')
    const password = 'TestPass123!'
    await signupUser(page, email, username, password)
    await use(page)
    const token = await page.evaluate(() => sessionStorage.getItem('llm_gateway_user_token'))
    if (token) {
      const request = page.request
      await deleteTestSessionsViaAPI(request, token)
    }
    await context.close()
  },

  testUser: async ({}, use) => {
    await use({
      email: uniqueId('e2e-user') + '@test.example',
      username: uniqueId('e2e-user'),
      password: 'TestPass123!',
    })
  },
})

export { expect }
