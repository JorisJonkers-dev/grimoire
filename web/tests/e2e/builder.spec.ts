import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('an author builds the Marsh Lantern in the Effect builder, previews its area and saves it', async ({ page }, info) => {
  const name = `Marsh Lantern ${info.project.name} ${String(Date.now())}`
  await page.goto('/library')
  await page.getByTestId('library-kind').selectOption('spell')
  await page.getByTestId('library-name').fill(name)
  await page.getByTestId('library-add').click()
  await page.getByTestId('open-builder').click()
  await expect(page.getByTestId('builder-form')).toBeVisible()

  await page.getByTestId('shape').selectOption('emanation')
  await page.getByTestId('size').fill('10')
  await page.getByTestId('range').fill('0')
  await page.getByTestId('save').selectOption('wisdom')
  await page.getByTestId('duration').selectOption('minutes')
  await page.getByTestId('duration-amount').fill('1')
  await page.getByTestId('concentration').check()
  await page.getByTestId('ritual').check()
  await page.getByTestId('material').check()
  await page.getByTestId('material-text').fill('a lantern of bog glass')
  for (const type of ['light', 'reveal', 'condition', 'damage']) {
    await page.getByTestId('add-type').selectOption(type)
    await page.getByTestId('add-part').click()
  }
  await page.getByTestId('condition-2').selectOption('charmed')
  await page.getByTestId('only-2-undead').check()
  await page.getByTestId('only-2-fey').check()
  await page.getByTestId('when-3').selectOption('start_of_turn')
  await page.getByTestId('dice-3').fill('1d6')
  await page.getByTestId('damage-type-3').selectOption('radiant')

  await page.getByTestId('preview').click()
  await expect(page.getByTestId('area-hex')).toHaveCount(18)
  const text = page.getByTestId('rules-text')
  await expect(text).toContainText('Casting Time: 1 action or Ritual.')
  await expect(text).toContainText('has the Charmed condition, if it is Undead or Fey.')
  await expect(text).toContainText('starts its turn in the area takes 1d6 radiant damage')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByTestId('save-spell').click()
  await expect(page.getByTestId('builder-status')).toHaveText('Saved as Revision 2.')
  await page.reload()
  await expect(page.getByTestId('rules-text')).toContainText('Bright light fills 20 feet')
})
