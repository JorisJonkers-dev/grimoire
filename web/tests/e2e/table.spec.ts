import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('the DM steers the table display: camera, ping, title card and blackout', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Table ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(page.url()).pathname

  const tableContext = await browser.newContext({ viewport: { width: 1920, height: 1080 } })
  const table = await tableContext.newPage()
  await table.goto(`${sessionUrl}/table`)
  await expect(table.getByTestId('camera')).toBeVisible()
  const before = await table.getByTestId('camera-world').getAttribute('style')

  await page.getByTestId('tool-camera').check()
  await page.locator('[data-hex="3,0"]').click()
  await expect(page.getByTestId('camera-free')).toBeChecked()
  await expect.poll(async () => table.getByTestId('camera-world').getAttribute('style')).not.toBe(before)

  await page.getByTestId('tool-ping').check()
  await page.locator('[data-hex="1,0"]').click()
  await expect(table.getByTestId('ping')).toBeVisible()

  await page.getByTestId('scene').selectOption('title')
  await page.getByTestId('scene-title').fill('Chapter One')
  await page.getByTestId('scene-body').fill('The road north')
  await page.getByTestId('show-scene').click()
  await expect(table.getByTestId('scene-title')).toContainText('Chapter One')
  await expect(table.getByTestId('scene-title')).toContainText('The road north')
  expect((await new AxeBuilder({ page: table }).analyze()).violations).toEqual([])

  await page.getByTestId('blackout-toggle').click()
  await expect(table.getByTestId('blackout')).toBeVisible()
  await expect(table.getByTestId('scene-title')).toHaveCount(0)
  await page.getByTestId('blackout-toggle').click()
  await expect(table.getByTestId('scene-title')).toBeVisible()

  await page.getByTestId('scene').selectOption('local')
  await page.getByTestId('show-scene').click()
  await expect(table.getByTestId('camera')).toBeVisible()
})
