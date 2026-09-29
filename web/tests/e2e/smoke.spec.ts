import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('home shows the live service status from the API and database', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Grimoire' })).toBeVisible()
  await expect(page.getByTestId('who-am-i')).toContainText('Signed in as e2e-player')
  const status = page.getByTestId('status-ok')
  await expect(status).toContainText('Database')
  await expect(status).toContainText('up')
  const results = await new AxeBuilder({ page }).analyze()
  expect(results.violations).toEqual([])
})

test('the component gallery is accessible on every device', async ({ page }) => {
  await page.goto('/gallery')
  await expect(page.getByRole('heading', { level: 1, name: 'Component gallery' })).toBeVisible()
  for (const button of await page.getByRole('button').all()) {
    const box = await button.boundingBox()
    expect(box?.height ?? 0).toBeGreaterThanOrEqual(44)
  }
  const results = await new AxeBuilder({ page }).analyze()
  expect(results.violations).toEqual([])
})
