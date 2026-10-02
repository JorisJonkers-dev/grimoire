import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { buildFighter } from './wizard'

test('the DM grants Heroic Inspiration and the player spends it on a reroll', async ({ page, browser }, info) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const stamp = `${info.project.name}-${String(Date.now())}`
  const dm = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-inspire-dm-${stamp}` } })).newPage()
  await dm.goto('/campaigns')
  await dm.getByTestId('campaign-name').fill(`Inspiration ${stamp}`)
  await dm.getByTestId('campaign-display-name').fill('DM')
  await dm.getByRole('button', { name: 'Start as DM' }).click()
  await dm.getByTestId('create-invite').click()
  const link = new URL(await dm.getByTestId('invite-link').inputValue())

  await page.goto(link.pathname + link.hash)
  await page.getByTestId('join-display-name').fill('Kara')
  await page.getByRole('button', { name: 'Join as Player' }).click()
  await page.getByTestId('build-character').click()
  await buildFighter(page, 'Kara')
  await page.getByTestId('create-character').click()
  await expect(page.getByTestId('inspiration')).toContainText('none')
  const sheet = new URL(page.url()).pathname

  await dm.goto(sheet)
  await dm.getByTestId('grant-inspiration').click()
  await expect(dm.getByTestId('inspiration')).toContainText('yours to spend')

  await page.goto(sheet.replace(/\/characters\/.*/, ''))
  await expect(page.getByTestId('inspired-Kara')).toBeVisible()
  await page.getByTestId('dice-link').click()
  await page.getByTestId('roll-purpose').fill('Athletics')
  await page.getByRole('button', { name: 'Ask for the roll' }).click()
  const card = page.getByTestId('roll-card')
  await card.getByTestId('roll-rest').click()
  await expect(card.getByTestId('inspiration-choice')).toContainText('You have Heroic Inspiration')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await card.getByTestId('reroll-0').click()
  await expect(card.getByTestId('rerolled')).toBeVisible()
  await expect(card.getByTestId('roll-total')).toContainText('Total')

  await page.goto(sheet)
  await expect(page.getByTestId('inspiration')).toContainText('none')
})
