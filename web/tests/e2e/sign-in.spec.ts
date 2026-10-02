import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('an invitee sets up an Account, signs out and signs back in with a password', async ({ browser, request }, info) => {
  test.skip(!['phone', 'desktop'].includes(info.project.name), 'the sign-in page is checked on phone and desktop')
  const created = await request.post('/api/v1/admin/account-invites', { headers: { 'X-User-Id': 'e2e-admin' }, data: { hours: 24 } })
  expect(created.status()).toBe(201)
  const { token } = (await created.json()) as { token: string }
  const username = `e2e-${info.project.name}-${String(Date.now())}`

  const context = await browser.newContext()
  const page = await context.newPage()
  await page.goto(`/account-invite#${token}`)
  await page.getByTestId('setup-username').fill(username)
  await page.getByTestId('setup-nickname').fill('Tamsin')
  await page.getByTestId('setup-email').fill(`${username}@example.com`)
  await page.getByTestId('setup-password').fill('correct horse battery')
  await page.getByRole('button', { name: 'Create my Account' }).click()
  await expect(page.getByTestId('account-link')).toHaveText('Tamsin')

  await page.getByTestId('sign-out').click()
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.getByTestId('sign-in-username').fill(username)
  await page.getByTestId('sign-in-password').fill('not the password')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByTestId('sign-in-failed')).toBeVisible()
  await page.getByTestId('sign-in-password').fill('correct horse battery')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByTestId('account-link')).toHaveText('Tamsin')
  await context.close()
})
