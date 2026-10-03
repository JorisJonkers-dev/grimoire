import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('an author builds the Lamplighter in the class builder with its own slot table and saves it', async ({ page }, info) => {
  const name = `Lamplighter ${info.project.name} ${String(Date.now())}`
  await page.goto('/library')
  await page.getByTestId('library-kind').selectOption('class')
  await page.getByTestId('library-name').fill(name)
  await page.getByTestId('library-add').click()
  await page.getByTestId('open-class-builder').click()
  await expect(page.getByTestId('class-form')).toBeVisible()

  await page.getByTestId('class-save-1').selectOption('charisma')
  await page.getByTestId('class-feature-0-name').fill('Wickcraft')
  await page.getByTestId('class-casting-kind').selectOption('slots')
  await page.getByTestId('class-casting-ability').selectOption('charisma')
  await page.getByTestId('class-spell-list').selectOption('bard')
  await page.getByTestId('class-slots-1-1').fill('1')
  await page.getByTestId('class-add-column').click()
  await page.getByTestId('class-column-0-name').fill('Wick Marks')
  await page.getByTestId('class-column-0-1').fill('+1')

  await page.getByTestId('class-preview').click()
  const lines = page.getByTestId('class-lines')
  await expect(lines).toContainText('Spellcasting: Charisma, from the bard list, its own slot table')
  await expect(lines).toContainText('Level 1: Wickcraft · Cantrips 3 · Prepared 4 · Slots 1 · Wick Marks +1')
  await expect(lines).toContainText('Level 3: Subclass')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByTestId('class-save').click()
  await expect(page.getByTestId('class-status')).toHaveText('Saved as Revision 2.')
  await page.reload()
  await expect(page.getByTestId('class-lines')).toContainText('Wick Marks')
})
