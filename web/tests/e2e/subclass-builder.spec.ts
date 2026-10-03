import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('an author builds the Lantern Warden in the subclass builder, reads it back and saves it', async ({ page }, info) => {
  const name = `Lantern Warden ${info.project.name} ${String(Date.now())}`
  await page.goto('/library')
  await page.getByTestId('library-kind').selectOption('subclass')
  await page.getByTestId('library-name').fill(name)
  await page.getByTestId('library-add').click()
  await page.getByTestId('open-subclass-builder').click()
  await expect(page.getByTestId('subclass-form')).toBeVisible()

  await page.getByTestId('subclass-class').selectOption('fighter')
  await page.getByTestId('feature-0-name').fill('Lantern Oath')
  await page.getByTestId('feature-0-text').fill('You carry a lantern that never gutters.')
  await page.getByTestId('add-resource').click()
  await page.getByTestId('resource-0-name').fill('Lantern Light')
  await page.getByTestId('add-feature').click()
  await page.getByTestId('feature-1-name').fill('Kindle')
  await page.getByTestId('feature-1-uses').selectOption('lantern-light')
  await page.getByTestId('feature-1-spell').fill('light')
  await page.getByTestId('feature-1-spell-name').fill('Light')
  await page.getByTestId('add-choice').click()
  await page.getByTestId('choice-0-name').fill('Lantern Style')
  await page.getByTestId('choice-0-options').fill('Bog Glass, Ember Wick')
  await page.getByTestId('choice-0-options').blur()

  await page.getByTestId('subclass-preview').click()
  const lines = page.getByTestId('subclass-lines')
  await expect(lines).toContainText('Level 3: Kindle. You can cast Light with it, spending a use of Lantern Light.')
  await expect(lines).toContainText('Level 3 choice: Lantern Style, pick 1 of Bog Glass, Ember Wick.')
  await expect(lines).toContainText('Lantern Light: uses equal to your Proficiency Bonus from level 3, back on a Long Rest.')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByTestId('subclass-save').click()
  await expect(page.getByTestId('subclass-status')).toHaveText('Saved as Revision 2.')
  await page.reload()
  await expect(page.getByTestId('subclass-lines')).toContainText('Lantern Style')
})
