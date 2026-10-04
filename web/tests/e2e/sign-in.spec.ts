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

test('the sign-in page centres its form beside the brand, with the external login first', async ({ page }) => {
  await page.route('**/api/v1/sign-in-methods', (route) => route.fulfill({ json: { oidc: 'jorisjonkers.dev' } }))
  await page.goto('/sign-in')
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible()
  await expect(page.getByTestId('auth-brand')).toContainText('The table is set. Take your seat.')
  await expect(page.getByRole('navigation', { name: 'Main' })).toHaveCount(0)

  // The column sits in the middle of its panel, and nothing runs off the side.
  const box = async (testId: string) => (await page.getByTestId(testId).boundingBox()) ?? { x: 0, y: 0, width: 0, height: 0 }
  const panel = (await page.getByRole('main').boundingBox()) ?? { x: 0, y: 0, width: 0, height: 0 }
  const column = await box('auth-column')
  expect(Math.abs(column.x + column.width / 2 - (panel.x + panel.width / 2))).toBeLessThanOrEqual(1)
  expect(column.width).toBeLessThanOrEqual(480)
  const wide = (page.viewportSize()?.width ?? 0) >= 900
  const brand = await box('auth-brand')
  if (wide) {
    // Beside the brand, and in the middle of the screen's height.
    expect(brand.x + brand.width).toBeLessThanOrEqual(panel.x + 1)
    expect(Math.abs(column.y + column.height / 2 - (page.viewportSize()?.height ?? 0) / 2)).toBeLessThanOrEqual(2)
  } else {
    expect(brand.y + brand.height).toBeLessThanOrEqual(panel.y + 1)
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)

  // Both ways in: the external login, then the Username.
  const external = await box('sign-in-oidc')
  const username = (await page.getByTestId('sign-in-username').boundingBox()) ?? external
  expect(external.y + external.height).toBeLessThan(username.y)
  await expect(page.getByTestId('sign-in-oidc')).toHaveText('Sign in with jorisjonkers.dev')
  await expect(page.getByTestId('sign-in-or')).toHaveText('or with your Username')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // The external login sends the browser to the login it names.
  await page.route('**/api/v1/oidc/sign-ins', (route) => route.fulfill({ status: 201, json: { url: `${new URL(page.url()).origin}/about/attribution?external=1` } }))
  await page.getByTestId('sign-in-oidc').click()
  await expect(page).toHaveURL(/\/about\/attribution\?external=1$/)
})
