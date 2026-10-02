import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { buildFighter } from './wizard'

test('a player drags gear between slots, the bag and the Party Stash', async ({ page }, info) => {
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Packs ${info.project.name} ${String(Date.now())}`)
  await page.getByTestId('campaign-display-name').fill('Kara')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('build-character').click()
  await buildFighter(page, 'Kara', true)
  await page.getByTestId('create-character').click()
  await expect(page.getByTestId('ac')).toHaveText('18')
  await page.getByTestId('open-inventory').click()

  const equipment = page.getByTestId('equipment')
  await expect(equipment.getByTestId('slot-armor')).toContainText('Chain Mail')
  await expect(equipment.getByTestId('slot-off_hand')).toContainText('Shield')
  await expect(equipment.getByTestId('slot-main_hand')).toContainText('Longsword')
  await expect(page.getByTestId('carrying')).toContainText('64.0 / ')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // A phone has no native drag and drop; its player uses the buttons on each item.
  const phone = info.project.name === 'phone'
  if (phone) await equipment.getByTestId('unequip-off_hand').click()
  else await equipment.getByTestId('worn-off_hand').dragTo(page.getByTestId('bag'))
  await expect(page.getByTestId('item-shield')).toBeVisible()
  await expect(equipment.getByTestId('slot-off_hand')).not.toContainText('Shield')
  if (phone) await page.getByTestId('stash-shield').click()
  else await page.getByTestId('item-shield').dragTo(page.getByTestId('stash'))
  await expect(page.getByTestId('stash-item-shield')).toBeVisible()
  if (phone) await page.getByTestId('take-shield').click()
  else await page.getByTestId('stash-item-shield').dragTo(page.getByTestId('bag'))
  await expect(page.getByTestId('item-shield')).toBeVisible()
  if (phone) await page.getByTestId('equip-shield').selectOption('off_hand')
  else await page.getByTestId('item-shield').dragTo(equipment.getByTestId('slot-off_hand'))
  await expect(equipment.getByTestId('slot-off_hand')).toContainText('Shield')

  await page.getByTestId('unequip-armor').click()
  await expect(page.getByTestId('item-chain-mail')).toBeVisible()
  await page.getByRole('link', { name: '← Sheet' }).click()
  await expect(page.getByTestId('ac')).toHaveText('14')
})
