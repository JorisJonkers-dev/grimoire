import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

async function enter(page: Page, face: string) {
  const card = page.getByTestId('roll-card')
  await card.getByTestId('manual-0').click()
  await card.getByTestId('pad-0').getByRole('button', { name: face, exact: true }).click()
}

test('initiative from Roll Cards, then turns with the action economy on every screen', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Combat ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())

  const context = await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-combat-${stamp}` } })
  const player = await context.newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')

  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(page.url()).pathname
  await player.goto(sessionUrl)
  const table = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-combat-${stamp}` } })).newPage()
  await table.goto(`${sessionUrl}/table`)

  await page.getByTestId('token-label').fill('Aria')
  await page.getByTestId('token-kind').selectOption('party')
  await page.getByTestId('token-controller').selectOption({ label: 'Aria' })
  await page.locator('[data-hex="0,0"]').click()
  await page.getByTestId('token-label').fill('Goblin')
  await page.getByTestId('token-kind').selectOption('enemy')
  await page.getByTestId('token-controller').selectOption('')
  await page.locator('[data-hex="3,0"]').click()
  await expect(page.locator('[data-hex="3,0"]')).toHaveAttribute('aria-label', /Goblin/)

  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('bonus-Aria').fill('2')
  await page.getByTestId('begin-combat').click()
  await expect(page.getByTestId('initiative-rail')).toContainText('Rolling initiative')
  await expect(player.getByTestId('roll-card')).toContainText('Initiative for Aria')
  await expect(page.getByTestId('roll-card')).toContainText('Initiative for Goblin')
  await enter(player, '15')
  await enter(page, '5')

  for (const p of [page, player, table]) {
    await expect(p.getByTestId('initiative-rail')).toContainText('Round 1')
    await expect(p.getByTestId('rail-Aria')).toContainText('17')
    await expect(p.getByTestId('rail-Aria')).toHaveAttribute('aria-current', 'step')
  }
  await expect(player.getByTestId('your-turn')).toHaveText('Your turn')
  const turn = player.getByTestId('turn-Aria')
  await turn.getByTestId('spend-action').click()
  await expect(turn.getByTestId('spend-action')).toBeDisabled()
  await player.locator('[data-hex="0,2"]').click()
  await expect(player.getByTestId('walk-preview')).toContainText('Walk 10 ft')
  await player.locator('[data-hex="0,2"]').click()
  await expect(turn.getByTestId('movement')).toHaveText('20 / 30 ft')
  await player.locator('[data-hex="-5,2"]').click()
  await expect(player.getByTestId('rejection')).toHaveText('Aria has 20 ft of movement left.')
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])
  await turn.getByTestId('end-turn').click()

  await expect(page.getByTestId('turn-Goblin')).toBeVisible()
  await expect(player.getByTestId('your-turn')).toHaveCount(0)
  await page.getByTestId('turn-Goblin').getByTestId('end-turn').click()
  for (const p of [page, player, table]) await expect(p.getByTestId('initiative-rail')).toContainText('Round 2')
  await expect(player.getByTestId('turn-Aria').getByTestId('movement')).toHaveText('30 / 30 ft')
  await page.getByTestId('end-combat').click()
  for (const p of [page, player, table]) await expect(p.getByTestId('initiative-rail')).toHaveCount(0)
})
