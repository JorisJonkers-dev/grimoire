import AxeBuilder from '@axe-core/playwright'
import { expect, type APIRequestContext, type Browser, test } from '@playwright/test'

// account sets up an Account from an invite in its own browser and returns the signed-in page.
async function account(browser: Browser, request: APIRequestContext, username: string, nickname: string) {
  const created = await request.post('/api/v1/admin/account-invites', { headers: { 'X-User-Id': 'e2e-admin' }, data: { hours: 24 } })
  const { token } = (await created.json()) as { token: string }
  const context = await browser.newContext()
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

test('two people become Friends from the Friends page', async ({ browser, request }, info) => {
  test.skip(!['phone', 'desktop'].includes(info.project.name), 'the Friends page is checked on phone and desktop')
  const stamp = `${info.project.name}-${String(Date.now())}`
  const asker = await account(browser, request, `ask-${stamp}`, 'Asker')
  const friend = await account(browser, request, `fri-${stamp}`, 'Friendly')

  await asker.page.getByTestId('friends-link').click()
  await asker.page.getByTestId('friend-username').fill(`fri-${stamp}`)
  await asker.page.getByRole('button', { name: 'Send a request' }).click()
  await expect(asker.page.getByTestId('friend-request-sent')).toBeVisible()
  await expect(asker.page.getByTestId('friend-outgoing')).toContainText('Friendly')
  expect((await new AxeBuilder({ page: asker.page }).analyze()).violations).toEqual([])
  expect(await asker.page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)

  await friend.page.goto('/friends')
  await friend.page.getByTestId(`accept-ask-${stamp}`).click()
  await expect(friend.page.getByTestId(`friend-ask-${stamp}`)).toContainText('Asker')
  await asker.page.reload()
  await expect(asker.page.getByTestId(`friend-fri-${stamp}`)).toContainText('Friendly')
  await asker.context.close()
  await friend.context.close()
})
