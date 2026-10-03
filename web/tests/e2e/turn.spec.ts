import { expect, type Page, test } from '@playwright/test'

async function enter(page: Page, face: string) {
  const card = page.getByTestId('roll-card')
  await card.getByTestId('manual-0').click()
  await card.getByTestId('pad-0').getByRole('button', { name: face, exact: true }).click()
}

test('initiative is revealed on every screen and a player gets a banner as their turn starts', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Turns ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const user = { 'X-User-Id': `e2e-turn-${stamp}` }
  const player = await (await browser.newContext({ extraHTTPHeaders: user })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')

  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(page.url()).pathname
  await player.goto(sessionUrl)
  const table = await (await browser.newContext({ extraHTTPHeaders: user })).newPage()
  await table.goto(`${sessionUrl}/table`)
  const still = await (await browser.newContext({ extraHTTPHeaders: user, reducedMotion: 'reduce' })).newPage()
  await still.goto(sessionUrl)

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
  await page.getByTestId('begin-combat').click()
  await expect(page.getByTestId('initiative-rail')).toContainText('Rolling initiative')
  await enter(page, '5')
  await expect(player.getByTestId('roll-card')).toContainText('Initiative for Aria')
  await enter(player, '15')

  // The rolls show on every screen, and the player whose turn it is gets the banner.
  await Promise.all([
    ...[page, player, table].flatMap((p) => [expect(p.getByTestId('rolled-Aria')).toHaveText('15'), expect(p.getByTestId('rolled-Goblin')).toHaveText('5')]),
    expect(player.getByTestId('turn-banner')).toContainText("It's your turn"),
    expect(still.getByTestId('turn-banner')).toContainText('Aria'),
  ])
  await expect(still.getByTestId('initiative-rail')).toContainText('Round 1')
  await expect(still.getByTestId('rolled-Aria')).toHaveCount(0)
  await expect(page.getByTestId('turn-banner')).toHaveCount(0)
  for (const p of [page, player, table]) {
    await expect(p.getByTestId('rolled-Aria')).toHaveCount(0)
    await expect(p.getByTestId('roster-strip').getByRole('listitem').first()).toHaveAttribute('data-testid', 'rail-Aria')
  }
  await expect(player.getByTestId('turn-banner')).toHaveCount(0)

  // The banner never blocks the map, and comes back each time the turn does.
  await player.getByTestId('turn-Aria').getByTestId('end-turn').click()
  await expect(page.getByTestId('turn-Goblin')).toBeVisible()
  await expect(player.getByTestId('turn-banner')).toHaveCount(0)
  await page.getByTestId('turn-Goblin').getByTestId('end-turn').click()
  await expect(player.getByTestId('turn-banner')).toContainText("It's your turn")
  await player.getByTestId('turn-banner').click()
  await expect(player.getByTestId('turn-banner')).toHaveCount(0)
  await expect(player.getByTestId('initiative-rail')).toContainText('Round 2')
})
