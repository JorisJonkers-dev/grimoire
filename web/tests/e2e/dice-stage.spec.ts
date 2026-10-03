import { devices, expect, type Browser, type Locator, type Page, test } from '@playwright/test'

async function enter(card: Locator, face: string) {
  await card.getByTestId('manual-0').click()
  await card.getByTestId('pad-0').getByRole('button', { name: face, exact: true }).click()
}

/** A session with Scout (the player's) and Grik (the DM's) waiting to roll initiative. */
async function fight(dm: Page, browser: Browser, stamp: string, playerDevice: object = {}) {
  await dm.goto('/campaigns')
  await dm.getByTestId('campaign-name').fill(`Dice ${stamp}`)
  await dm.getByTestId('campaign-display-name').fill('DM')
  await dm.getByRole('button', { name: 'Start as DM' }).click()
  await dm.getByTestId('create-invite').click()
  const link = new URL(await dm.getByTestId('invite-link').inputValue())
  const user = { 'X-User-Id': `e2e-dice-${stamp}` }
  const player = await (await browser.newContext({ ...playerDevice, extraHTTPHeaders: user })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')
  await dm.getByTestId('start-session').click()
  await expect(dm.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(dm.url()).pathname
  await player.goto(sessionUrl)
  for (const [label, kind, controller, hex] of [['Scout', 'party', 'Aria', '0,0'], ['Grik', 'enemy', '', '1,0']] as const) {
    await dm.getByTestId('token-monster').fill('goblin-warrior')
    await dm.getByTestId('token-kind').selectOption(kind)
    await dm.getByTestId('token-controller').selectOption(controller ? { label: controller } : '')
    await dm.getByTestId('token-label').fill(label)
    await dm.locator(`[data-hex="${hex}"]`).click()
    await expect(dm.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(label))
  }
  await dm.getByTestId('choose-combatants').click()
  await dm.getByTestId('begin-combat').click()
  await expect(player.getByTestId('roll-card')).toContainText('Initiative for Scout')
  return { player, sessionUrl, user }
}

test('dice are thrown on the GPU and always land on the server\'s result, flat where motion is reduced', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const { player, sessionUrl, user } = await fight(page, browser, String(Date.now()))
  const still = await (await browser.newContext({ extraHTTPHeaders: user, reducedMotion: 'reduce' })).newPage()
  await still.goto(sessionUrl)
  await expect(still.getByTestId('roll-card')).toBeVisible()
  await expect(player.getByTestId('dice-canvas')).toBeAttached()
  await expect(still.getByTestId('dice-canvas')).toHaveCount(0)

  // The DM's roll is thrown on the DM's own screen, a natural 1 marked; nobody else sees it.
  await enter(page.getByTestId('roll-card'), '1')
  await expect(page.getByTestId('dice-critical')).toHaveText('Critical miss')
  await expect(page.getByTestId('dice-result')).toHaveClass(/result--miss/)
  await expect(player.getByTestId('dice-result')).toHaveCount(0)

  // The player's natural 20 is thrown on every screen, and each shows the server's total.
  await enter(player.getByTestId('roll-card'), '20')
  // Each screen shows its result for a moment only, so they are all watched at once. The GPU drew the
  // dice where it could; with reduced motion they are flat icons.
  await Promise.all([
    ...[player, page, still].flatMap((p) => [
      expect(p.getByTestId('dice-total')).toHaveText('20'),
      expect(p.getByTestId('dice-critical')).toHaveText('Critical hit'),
      expect(p.getByTestId('dice-breakdown')).toHaveText('20'),
    ]),
    expect(still.getByTestId('dice-2d').getByRole('img')).toHaveAttribute('aria-label', 'd20 showing 20, kept'),
  ])
  await expect(player.getByTestId('dice-2d')).toHaveCount(0)
  await expect(player.getByTestId('dice-result')).toHaveCount(0, { timeout: 8000 })
})

test('on a slow phone the roll still shows its total in time', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'phone', 'the low-end check runs on the phone profile')
  const dm = await (await browser.newContext({ ...devices['Desktop Chrome'], extraHTTPHeaders: { 'X-User-Id': `e2e-dice-dm-${String(Date.now())}` } })).newPage()
  // The phone is the player's: this project's own page, slowed to a sixth of its speed.
  await dm.goto('/campaigns')
  await dm.getByTestId('campaign-name').fill(`Slow ${String(Date.now())}`)
  await dm.getByTestId('campaign-display-name').fill('DM')
  await dm.getByRole('button', { name: 'Start as DM' }).click()
  await dm.getByTestId('create-invite').click()
  const link = new URL(await dm.getByTestId('invite-link').inputValue())
  await page.goto(link.pathname + link.hash)
  await page.getByTestId('join-display-name').fill('Aria')
  await page.getByRole('button', { name: 'Join as Player' }).click()
  await expect(page.getByTestId('member-list')).toContainText('Aria (you)')
  await dm.getByTestId('start-session').click()
  await expect(dm.getByTestId('connection')).toHaveText('Live')
  await page.goto(new URL(dm.url()).pathname)
  await dm.getByTestId('token-monster').fill('goblin-warrior')
  await dm.getByTestId('token-kind').selectOption('party')
  await dm.getByTestId('token-controller').selectOption({ label: 'Aria' })
  await dm.getByTestId('token-label').fill('Scout')
  await dm.locator('[data-hex="0,0"]').click()
  await expect(dm.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /Scout/)
  await dm.getByTestId('choose-combatants').click()
  await dm.getByTestId('begin-combat').click()
  await expect(page.getByTestId('roll-card')).toContainText('Initiative for Scout')

  const cdp = await page.context().newCDPSession(page)
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 6 })
  const started = Date.now()
  await enter(page.getByTestId('roll-card'), '17')
  // Three seconds from the tap: the throw and the glide take about half of that on a fast device, and a
  // device that cannot keep up is moved to the flat dice, which show the total at once.
  await expect(page.getByTestId('dice-total')).toHaveText('17', { timeout: 3000 })
  expect(Date.now() - started).toBeLessThan(3500)
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 })
})
