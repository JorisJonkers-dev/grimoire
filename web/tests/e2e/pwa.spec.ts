import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('installs as an app and reads the compendium offline', async ({ page, context }) => {
  await page.goto('/compendium/spells/fireball')
  await expect(page.getByRole('heading', { level: 1, name: 'Fireball' })).toBeVisible()
  const manifest = await page.locator('link[rel="manifest"]').getAttribute('href')
  const app = (await (await page.request.get(manifest ?? '')).json()) as { name: string; display: string; icons: unknown[] }
  expect(app).toMatchObject({ name: 'Grimoire', display: 'standalone' })
  expect(app.icons).toHaveLength(3)

  // Once the service worker controls the page, a reload caches the spell it reads.
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null)
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: 'Fireball' })).toBeVisible()

  await context.setOffline(true)
  await expect(page.getByTestId('offline')).toContainText('live play picks up again when the connection returns')
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: 'Fireball' })).toBeVisible()
  await expect(page.getByTestId('offline')).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await context.setOffline(false)
  await expect(page.getByTestId('offline')).toHaveCount(0)
})
