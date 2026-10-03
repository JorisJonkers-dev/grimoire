import AxeBuilder from '@axe-core/playwright'
import { expect, type Locator, type Page, test } from '@playwright/test'

async function enter(card: Locator, face: string) {
  await card.getByTestId('face-0').fill(face)
  await card.getByTestId('face-0').press('Enter')
}

async function place(page: Page, hex: string, label: string, kind: string, controller = '') {
  await page.getByTestId('token-monster').fill('goblin-warrior')
  await page.getByTestId('token-kind').selectOption(kind)
  await page.getByTestId('token-controller').selectOption(controller ? { label: controller } : '')
  await page.getByTestId('token-label').fill(label)
  await page.locator(`[data-hex="${hex}"]`).click()
  await expect(page.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(label))
}

test('the DM runs creatures from a console the players never see', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Console ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-console-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await player.goto(new URL(page.url()).pathname)
  await place(page, '0,0', 'Scout', 'party', 'Aria')
  await place(page, '1,0', 'Grik', 'enemy')
  await place(page, '2,-1', 'Snag', 'enemy')

  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('begin-combat').click()
  await enter(page.getByTestId('roll-card').filter({ hasText: 'Initiative for Grik' }), '20')
  await enter(page.getByTestId('roll-card').filter({ hasText: 'Initiative for Snag' }), '10')
  await enter(player.getByTestId('roll-card'), '1')

  // The console follows the turn: Grik is in hand, with its Suggested Action in the panel and under its token.
  const switcher = page.getByTestId('control-switcher')
  await expect(switcher.getByRole('button')).toHaveText([/Grik · acting/, 'Snag'])
  await expect(page.getByTestId('control-Grik')).toHaveAttribute('aria-pressed', 'true')
  const grik = page.getByTestId('creature-Grik')
  await expect(grik.getByTestId('creature-suggestion')).toContainText('against Scout')
  await expect(grik.getByTestId('agent-notes')).toContainText('No agent has acted on Grik.')
  await expect(page.locator('[data-testid^="caption-"]')).toHaveText(/→ Scout/)
  await expect(page.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /suggested: .* against Scout/)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // None of it reaches the player.
  await expect(player.getByTestId('initiative-rail')).toContainText('Round 1')
  await expect(player.getByTestId('control-switcher')).toHaveCount(0)
  await expect(player.locator('[data-testid^="creature-"]')).toHaveCount(0)
  await expect(player.locator('[data-testid^="caption-"]')).toHaveCount(0)
  await expect(player.locator('[data-hex="1,0"]')).not.toHaveAttribute('aria-label', /suggested/)

  // Switch to Snag: its panel replaces Grik's, and Grik's hotbar waits.
  await expect(page.getByTestId('hotbar-Grik')).toBeVisible()
  await page.getByTestId('control-Snag').click()
  await expect(page.getByTestId('creature-Snag')).toBeVisible()
  await expect(grik).toHaveCount(0)
  await expect(page.getByTestId('hotbar-Grik')).toHaveCount(0)

  // Take both in hand: one change of Tactics and one blow reach both.
  await page.getByTestId('control-several').check()
  await page.getByTestId('control-Grik').click()
  await expect(page.locator('[data-testid^="creature-"][data-testid$="-tactics"]')).toHaveCount(2)
  await page.getByTestId('bulk-tactics').selectOption('off')
  for (const label of ['Grik', 'Snag']) await expect(page.getByTestId(`creature-${label}`).getByTestId('creature-tactics')).toHaveValue('off')
  await expect(page.locator('[data-testid^="caption-"]')).toHaveCount(0)
  await page.getByTestId('bulk-hp').fill('2')
  await page.getByTestId('bulk-hurt').click()
  for (const label of ['Grik', 'Snag']) await expect(page.getByTestId(`creature-${label}`)).toContainText('8 / 10 hit points')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
