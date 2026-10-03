import { expect, type APIRequestContext, type Browser, type BrowserContextOptions } from '@playwright/test'

/** Sets up an Account from an invite in its own browser and returns the signed-in page. */
export async function account(browser: Browser, request: APIRequestContext, username: string, nickname: string, options: BrowserContextOptions = {}) {
  const created = await request.post('/api/v1/admin/account-invites', { headers: { 'X-User-Id': 'e2e-admin' }, data: { hours: 24 } })
  const { token } = (await created.json()) as { token: string }
  const context = await browser.newContext(options)
  const page = await context.newPage()
  await page.goto(`/account-invite#${token}`)
  await page.getByTestId('setup-username').fill(username)
  await page.getByTestId('setup-nickname').fill(nickname)
  await page.getByTestId('setup-email').fill(`${username}@example.com`)
  await page.getByTestId('setup-password').fill('correct horse battery')
  await page.getByRole('button', { name: 'Create my Account' }).click()
  await expect(page.getByTestId('account-link')).toHaveText(nickname)
  return { context, page }
}
