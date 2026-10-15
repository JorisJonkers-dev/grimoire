import { expect, test } from '@playwright/test'
import { violations } from './axe'

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
  const add = async (list: string, kind: string) => {
    await page.getByTestId(`${list}-kind`).selectOption(kind)
    await page.getByTestId(`${list}-add`).click()
  }
  await add('parts', 'light')
  await page.getByTestId('parts-0-bright_ft').fill('20')
  await page.getByTestId('parts-0-dim_ft').fill('20')
  await add('parts', 'reveal')
  await page.getByTestId('parts-1-qualities-invisible').check()
  // A fey or undead creature that fails the save is charmed: a branch on its type, holding one on the save.
  for (const [i, creature] of ['undead', 'fey'].entries()) {
    const at = `parts-${String(i + 2)}`
    await add('parts', 'branch')
    await page.getByTestId(`${at}-when`).selectOption('creature_is')
    await page.getByTestId(`${at}-creature_type`).fill(creature)
    await add(`${at}-then`, 'branch')
    await page.getByTestId(`${at}-then-0-n`).fill('1')
    // The form works from the keyboard alone.
    await page.getByTestId(`${at}-then-0-then-kind`).selectOption('save_condition')
    await page.getByTestId(`${at}-then-0-then-add`).press('Enter')
    await page.getByTestId(`${at}-then-0-then-0-condition`).selectOption('charmed')
  }
  await page.getByTestId('turn-damage').check()
  await page.getByTestId('turn-dice').fill('1d6')
  await page.getByTestId('turn-type').selectOption('radiant')

  await page.getByTestId('preview').click()
  await expect(page.getByTestId('area-hex')).toHaveCount(18)
  const text = page.getByTestId('rules-text')
  await expect(text).toContainText('Casting Time: 1 action or Ritual.')
  await expect(text).toContainText('If a creature is a fey and fails the save, it has the Charmed condition.')
  await expect(text).toContainText('starts its turn in the area takes 1d6 radiant damage')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect(await violations(page)).toEqual([])

  await page.getByTestId('save-spell').click()
  await expect(page.getByTestId('builder-status')).toHaveText('Saved as Revision 2.')
  await page.reload()
  await expect(page.getByTestId('rules-text')).toContainText('Bright light fills 20 feet')
})
