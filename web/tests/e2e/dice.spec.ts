import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('a roll card mixes a typed die with server rolls and lands on the total', async ({ page }, info) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Dice ${info.project.name} ${String(Date.now())}`)
  await page.getByTestId('campaign-display-name').fill('Roller')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('dice-link').click()

  await page.getByTestId('roll-purpose').fill('Stealth')
  await page.getByTestId('roll-edge').selectOption('advantage')
  await page.getByTestId('roll-bless').check()
  await page.getByTestId('mod-label').fill('Dexterity')
  await page.getByTestId('mod-value').fill('3')
  await page.getByTestId('mod-add').click()
  await expect(page.getByTestId('roll-notation')).toHaveText('2d20kh1+1d4')
  await page.getByRole('button', { name: 'Ask for the roll' }).click()

  const card = page.getByTestId('roll-card')
  await expect(card).toContainText('2 × d20, keep highest — Advantage')
  await card.getByTestId('face-0').fill('12')
  await card.getByTestId('face-0').press('Enter')
  await expect(card.getByTestId('die-0')).toContainText('your die')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await card.getByTestId('roll-rest').click()
  await expect(card.getByTestId('roll-total')).toContainText('Total')
  await expect(page.getByTestId('action-log')).toContainText('seed')
})
