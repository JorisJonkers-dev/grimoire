import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('a DM starts a campaign and a player joins through an invite link', async ({ page, browser }, info) => {
  const name = `Greyfen ${info.project.name} ${Date.now()}`
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(name)
  await page.getByTestId('campaign-display-name').fill('Morvain')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(page.getByTestId('member-list')).toContainText('Morvain (you)')

  await page.getByTestId('create-invite').click()
  const link = await page.getByTestId('invite-link').inputValue()
  expect(link).toMatch(/\/join#[A-Za-z0-9_-]{43}$/)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  const player = await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-invitee-${info.project.name}-${Date.now()}` } })
  const tab = await player.newPage()
  await tab.goto(new URL(link).pathname + new URL(link).hash)
  await expect(tab.getByTestId('join-form')).toContainText(`Morvain invites you to ${name}`)
  await tab.getByTestId('join-display-name').fill('Ireena')
  await tab.getByRole('button', { name: 'Join as Player' }).click()
  await expect(tab.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(tab.getByTestId('member-list')).toContainText('Ireena (you)')
  await expect(tab.getByTestId('invites')).toHaveCount(0)
  expect(await tab.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await player.close()

  await page.reload()
  await expect(page.getByTestId('member-list')).toContainText('Ireena')
})
