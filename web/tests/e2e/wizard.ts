import type { Page } from '@playwright/test'

/** Walks the creation wizard for a human soldier fighter, with chain mail and a longsword when armed. */
export async function buildFighter(page: Page, name: string, armed = false) {
  await page.getByRole('radio', { name: /Human/ }).check()
  await page.getByTestId('next').click()
  await page.getByRole('radio', { name: /^Fighter/ }).check()
  await page.getByTestId('next').click()
  await page.getByRole('radio', { name: /Soldier/ }).check()
  await page.getByTestId('next').click()
  await page.getByTestId('bonus-0').selectOption('strength')
  await page.getByTestId('bonus-1').selectOption('constitution')
  await page.getByTestId('next').click()
  await page.getByRole('checkbox', { name: /Perception/ }).check()
  await page.getByRole('checkbox', { name: /Survival/ }).check()
  await page.getByTestId('next').click()
  if (armed) {
    await page.getByTestId('armor').selectOption('chain-mail')
    await page.getByTestId('shield').check()
    await page.getByRole('checkbox', { name: /^Longsword/ }).check()
  }
  await page.getByTestId('next').click()
  await page.getByTestId('appearance').fill('Tall, with a scar over one eye.')
  await page.getByTestId('next').click()
  await page.getByTestId('character-name').fill(name)
  await page.getByTestId('next').click()
}
