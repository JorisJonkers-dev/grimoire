import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { buildFighter } from './wizard'

test('a player builds a Character through the wizard and plays from the sheet', async ({ page }, info) => {
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Builder ${info.project.name} ${String(Date.now())}`)
  await page.getByTestId('campaign-display-name').fill('Tester')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('build-character').click()

  await expect(page.getByTestId('starting-level')).toContainText('level 1')
  await buildFighter(page, 'Kara', true)
  await expect(page.getByTestId('step-review')).toContainText('Kara')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByTestId('create-character').click()

  const sheet = page.getByTestId('character-sheet')
  await expect(sheet.getByRole('heading', { level: 1, name: 'Kara' })).toBeVisible()
  await expect(sheet.getByTestId('ac')).toHaveText('18')
  await expect(sheet.getByTestId('hp')).toHaveText('12 / 12')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await expect(sheet.getByTestId('attacks')).toContainText('mastery')
  await sheet.getByTestId('hp-amount').fill('5')
  await sheet.getByTestId('hp-temp').click()
  await expect(sheet.getByTestId('temp-hp')).toHaveText('+5 temporary')
  await sheet.getByTestId('hp-amount').fill('7')
  await sheet.getByTestId('hp-damage').click()
  await expect(sheet.getByTestId('hp')).toHaveText('10 / 12')
  await expect(sheet.getByTestId('temp-hp')).toHaveCount(0)
  await sheet.getByTestId('hp-heal').click()
  await expect(sheet.getByTestId('hp')).toHaveText('12 / 12')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  const phone = info.project.name === 'phone'
  const trait = sheet.getByTestId('trait').filter({ hasText: 'Second Wind' })
  if (phone) {
    await expect(trait).toBeHidden()
    await sheet.getByTestId('part-features').click()
  }
  await expect(trait).toBeVisible()
  await trait.locator('summary').click()
  await expect(trait).toContainText('Bonus Action')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
