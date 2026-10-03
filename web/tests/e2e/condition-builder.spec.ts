import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('an author builds Frostbite in the condition builder, stacking to three levels, and saves it', async ({ page }, info) => {
  const name = `Frostbite ${info.project.name} ${String(Date.now())}`
  await page.goto('/library')
  await page.getByTestId('library-kind').selectOption('condition')
  await page.getByTestId('library-name').fill(name)
  await page.getByTestId('library-add').click()
  await page.getByTestId('open-condition-builder').click()
  await expect(page.getByTestId('condition-form')).toBeVisible()
  await page.getByTestId('condition-icon').selectOption('snow')
  await page.getByTestId('condition-ends').selectOption('rest')
  await page.getByTestId('condition-stacks').check()
  await page.getByTestId('condition-max-level').fill('3')
  await page.getByTestId('condition-speed').fill('5')
  await page.getByTestId('condition-add-part').click()
  await page.getByTestId('condition-preview').click()
  const lines = page.getByTestId('condition-lines')
  await expect(lines).toContainText('Ends on a rest.')
  await expect(lines).toContainText('Each level reduces Speed by 5 feet; it rises to level 3 at most.')
  await expect(lines).toContainText('Attack rolls against the target have Advantage')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByTestId('condition-save').click()
  await expect(page.getByTestId('condition-status')).toHaveText('Saved as Revision 2.')
})
