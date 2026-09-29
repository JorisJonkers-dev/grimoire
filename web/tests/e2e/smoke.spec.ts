import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('home shows the live service status from the API and database', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Grimoire' })).toBeVisible()
  const status = page.getByTestId('status-ok')
  await expect(status).toContainText('Database')
  await expect(status).toContainText('up')
  const results = await new AxeBuilder({ page }).analyze()
  expect(results.violations).toEqual([])
})
