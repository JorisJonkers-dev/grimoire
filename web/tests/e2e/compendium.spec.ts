import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('find a spell, read it and open a condition it mentions', async ({ page }) => {
  await page.goto('/compendium/spells')
  await page.getByTestId('spell-search').fill('hold person')
  await page.getByRole('link', { name: /Hold Person/ }).first().click()
  await expect(page.getByRole('heading', { level: 1, name: 'Hold Person' })).toBeVisible()
  await page.getByRole('button', { name: /paralyzed/i }).first().click()
  await expect(page.getByTestId('condition-popover')).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('switch a spell between the 2024 and 2014 rules', async ({ page }) => {
  await page.goto('/compendium/spells/fireball')
  await expect(page.getByRole('heading', { level: 1, name: 'Fireball' })).toBeVisible()
  await page.getByRole('link', { name: '2014' }).click()
  await expect(page.getByRole('link', { name: '2014' })).toHaveAttribute('aria-current', 'page')
})

test('attribution names the author and the SRD', async ({ page }) => {
  await page.goto('/about/attribution')
  await expect(page.getByRole('heading', { level: 2, name: 'System Reference Document 5.2' })).toBeVisible()
  await expect(page.getByRole('contentinfo')).toContainText('Joris Jonkers')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('browse monsters, read a statblock and see the automation report', async ({ page }) => {
  await page.goto('/compendium/monster')
  await page.getByTestId('entry-search').fill('goblin')
  await page.getByRole('link', { name: /Goblin/ }).first().click()
  await expect(page.getByTestId('entry-facts')).toContainText('Armor Class')
  await expect(page.getByTestId('entry-facts')).toContainText('Challenge')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByRole('link', { name: 'Automation coverage' }).click()
  await expect(page.getByTestId('automation-table')).toContainText('Monsters')
})

test('every compendium tab lists entries', async ({ page }) => {
  await page.goto('/compendium/class')
  for (const tab of ['Classes', 'Species', 'Backgrounds', 'Feats', 'Weapons', 'Armor', 'Equipment', 'Magic items', 'Conditions']) {
    await page.getByRole('navigation', { name: 'Compendium' }).getByRole('link', { name: tab, exact: true }).click()
    await expect(page.getByRole('heading', { level: 1, name: tab })).toBeVisible()
    await expect(page.getByTestId('entry-list').getByRole('link').first()).toBeVisible()
  }
})
