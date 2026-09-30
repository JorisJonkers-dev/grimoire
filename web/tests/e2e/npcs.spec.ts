import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('a DM edits an NPC, compares two revisions and restores the first', async ({ page }, info) => {
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Prep ${info.project.name} ${String(Date.now())}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('npcs-link').click()
  await page.getByTestId('npc-name').fill('Strahd')
  await page.getByRole('button', { name: 'Add NPC' }).click()

  await page.getByTestId('npc-notes').fill('Wants Ireena.')
  await page.getByRole('button', { name: 'Save' }).click()
  const history = page.getByTestId('npc-history')
  await expect(history).toContainText('#2 update')
  await history.getByRole('checkbox', { name: 'Compare revision 1' }).check()
  await history.getByRole('checkbox', { name: 'Compare revision 2' }).check()
  await expect(page.getByTestId('npc-diff')).toContainText('Wants Ireena.')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByTestId('restore-1').click()
  await expect(history).toContainText('#3 restore from #1')
  await expect(page.getByTestId('npc-notes')).toHaveValue('')
})
