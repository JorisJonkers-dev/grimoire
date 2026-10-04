import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('anyone reads the guides, opens an entry from one and copies a link to it', async ({ page, context }, info) => {
  await page.goto('/compendium/spells')
  await page.getByRole('navigation', { name: 'Compendium' }).getByRole('link', { name: 'Guides' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Guides' })).toBeVisible()

  // Spells by level: the cantrips are there, Fire Bolt among them.
  const cantrips = page.getByTestId('spell-level').filter({ hasText: 'Cantrips' })
  await cantrips.locator('summary').click()
  await expect(cantrips.getByRole('link', { name: 'Fire Bolt', exact: true })).toBeVisible()
  // Attacks by Challenge Rating: a row for CR 1/4, with attacks to sum up.
  const quarter = page.getByTestId('challenge-row').filter({ has: page.getByRole('rowheader', { name: '1/4', exact: true }) })
  await expect(quarter.locator('td').nth(2)).toContainText('+')
  // Loot: four tiers, and rare items from level 5.
  await expect(page.getByTestId('loot-tier')).toHaveCount(4)
  const rare = page.getByTestId('loot-rarity').filter({ hasText: /^Rare magic items/ })
  await expect(rare.locator('summary')).toContainText('from level 5')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // From the guide to the entry, and a link to it to share.
  await rare.locator('summary').click()
  await rare.getByRole('link').first().click()
  await expect(page.getByTestId('entry-detail')).toBeVisible()
  if (info.project.name === 'desktop') await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await page.getByTestId('copy-link').click()
  // A browser that will not copy shows the address to copy by hand instead.
  await expect(page.getByTestId('link-copied').or(page.getByTestId('link-to-copy'))).toBeVisible()
  if (info.project.name === 'desktop') {
    await expect(page.getByTestId('link-copied')).toHaveText('Link copied.')
    const copied = await page.evaluate(() => navigator.clipboard.readText())
    expect(copied).toMatch(/\/compendium\/magic-item\/[a-z0-9-]+\?ruleset=srd-20(14|24)$/)
    // The link opens the same entry for whoever is given it.
    const title = await page.getByRole('heading', { level: 1 }).textContent()
    await page.goto(copied)
    await expect(page.getByRole('heading', { level: 1 })).toHaveText(title ?? '')
  }
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // The guides are there for a request with no session at all.
  const bare = await page.request.get('/api/v1/compendium/guides?ruleset=srd-2014')
  expect(bare.status()).toBe(200)
  expect(bare.headers()['ratelimit-limit']).toBeTruthy()
})
