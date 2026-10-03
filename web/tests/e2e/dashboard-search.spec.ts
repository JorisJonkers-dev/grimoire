import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('the Dashboard shows a Session under way one click away, and the search finds only what is mine', async ({ page, browser }, info) => {
  // Every project runs this at the same moment as the same user: each needs a name of its own.
  const stamp = `${String(Date.now())}${info.project.name}`
  const name = `Emberfall ${stamp}`
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(name)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')

  // Home: the Session is there to join, in one click.
  await page.goto('/')
  const live = page.getByTestId('live-session').filter({ hasText: name })
  await expect(live).toContainText('you are the DM')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await live.getByRole('link', { name: `Join Session 1 of ${name}` }).click()
  await expect(page.getByTestId('connection')).toHaveText('Live')

  // The header's search box finds the Campaign by its name, with a line that says what it is.
  await page.goto('/')
  await page.getByTestId('header-search-input').fill(stamp)
  await page.getByTestId('header-search-input').press('Enter')
  await expect(page.getByTestId('search-count')).toHaveText(`1 result for “${stamp}”.`)
  const hit = page.getByTestId('search-hit')
  await expect(hit).toContainText(name)
  await expect(hit).toContainText('You are the DM of this Campaign')
  // The compendium is there for everyone, with a preview of each result.
  await page.getByTestId('search-input').fill('fireball')
  const spell = page.getByTestId('search-group').filter({ hasText: 'Compendium' }).getByTestId('search-hit').filter({ hasText: 'Fireball' }).first()
  await expect(spell).toContainText('Level 3 evocation spell')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await spell.getByRole('link').click()
  await expect(page.getByRole('heading', { level: 1, name: 'Fireball' })).toBeVisible()

  // Somebody else finds nothing of the Campaign, and has no Session of it on their Dashboard.
  const other = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-search-${stamp}` } })).newPage()
  await other.goto(`/search?q=${stamp}`)
  await expect(other.getByTestId('search-count')).toHaveText(`Nothing found for “${stamp}”.`)
  await other.goto('/')
  await expect(other.getByTestId('nothing-needed')).toBeVisible()
  await expect(other.getByTestId('live-session').filter({ hasText: name })).toHaveCount(0)
})
