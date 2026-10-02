import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { buildFighter } from './wizard'

test('the DM grants a level and the player takes it through the level-up wizard', async ({ page, browser }, info) => {
  const stamp = `${info.project.name}-${String(Date.now())}`
  const dm = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-levelup-dm-${stamp}` } })).newPage()
  await dm.goto('/campaigns')
  await dm.getByTestId('campaign-name').fill(`Levels ${stamp}`)
  await dm.getByTestId('campaign-display-name').fill('DM')
  await dm.getByRole('button', { name: 'Start as DM' }).click()
  await dm.getByTestId('create-invite').click()
  const link = new URL(await dm.getByTestId('invite-link').inputValue())

  await page.goto(link.pathname + link.hash)
  await page.getByTestId('join-display-name').fill('Kara')
  await page.getByRole('button', { name: 'Join as Player' }).click()
  await page.getByTestId('build-character').click()
  await buildFighter(page, 'Kara', true)
  await page.getByTestId('create-character').click()
  const sheet = page.getByTestId('character-sheet')
  await expect(sheet.getByTestId('hp')).toHaveText('12 / 12')
  await expect(sheet.getByTestId('level-up')).toHaveCount(0)

  await dm.goto(new URL(page.url()).pathname)
  await dm.getByTestId('unlock-level').click()
  await expect(dm.getByTestId('unlock-level')).toHaveCount(0)

  await page.reload()
  await sheet.getByTestId('level-up').click()
  await expect(page.getByTestId('step-class')).toBeVisible()
  await expect(page.getByTestId('level-class-wizard')).toBeDisabled()
  await expect(page.getByTestId('step-class')).toContainText('Needs Intelligence 13+ (wizard)')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByTestId('next').click()
  await page.getByTestId('hp-average').check()
  await page.getByTestId('next').click()
  await expect(page.getByTestId('step-review')).toContainText('Fighter 2')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByTestId('take-level').click()

  await expect(sheet.getByTestId('sheet-classes')).toContainText('Level 2')
  await expect(sheet.getByTestId('hp')).toHaveText('20 / 20')
  await expect(sheet.getByTestId('level-up')).toHaveCount(0)
})
