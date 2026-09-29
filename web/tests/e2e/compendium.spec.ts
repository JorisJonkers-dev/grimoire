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
