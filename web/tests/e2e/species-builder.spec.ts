import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('an author builds the Marshkin in the species builder with a lineage and saves it', async ({ page }, info) => {
  const name = `Marshkin ${info.project.name} ${String(Date.now())}`
  await page.goto('/library')
  await page.getByTestId('library-kind').selectOption('species')
  await page.getByTestId('library-name').fill(name)
  await page.getByTestId('library-add').click()
  await page.getByTestId('open-species-builder').click()
  await expect(page.getByTestId('species-form')).toBeVisible()

  await page.getByTestId('species-size-small').check()
  await page.getByTestId('species-add-speed').click()
  await page.getByTestId('species-add-sense').click()
  await page.getByTestId('species-resist-poison').check()
  await page.getByTestId('species-add-lineage').click()
  await page.getByTestId('species-lineage-0-name').fill('Bog')
  await page.getByTestId('species-lineage-0-text').fill('You know the Druidcraft cantrip.')
  await page.getByTestId('species-lineage-0-add-spell').click()
  await page.getByTestId('species-lineage-0-spell-0-name').fill('Druidcraft')
  await page.getByTestId('species-lineage-0-spell-0-slug').fill('druidcraft')
  await page.getByTestId('species-lineage-0-spell-0-uses').selectOption('at_will')

  await page.getByTestId('species-preview').click()
  const lines = page.getByTestId('species-lines')
  await expect(lines).toContainText(`${name}, Bog lineage: 30 feet; Swim 30 feet.`)
  await expect(lines).toContainText('Size. Medium or Small, chosen when you select this species.')
  await expect(lines).toContainText('Level 1: Druidcraft. You can cast Druidcraft at will.')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByTestId('species-save').click()
  await expect(page.getByTestId('species-status')).toHaveText('Saved as Revision 2.')
  await page.reload()
  await expect(page.getByTestId('species-lines')).toContainText('Bog lineage')
})
